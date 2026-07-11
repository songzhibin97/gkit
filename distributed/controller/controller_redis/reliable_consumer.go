package controller_redis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	json "github.com/json-iterator/go"

	"github.com/songzhibin97/gkit/distributed/task"
)

func (c *ControllerRedis) consumeReliableDelivery(
	attemptCtx context.Context,
	queue *reliableQueue,
	delivery *reliableDelivery,
	handler task.Processor,
) error {
	var signature task.Signature
	decoder := json.NewDecoder(bytes.NewReader(delivery.payload))
	decoder.UseNumber()
	if err := decoder.Decode(&signature); err != nil {
		decodeErr := fmt.Errorf("decode queued task: %w", err)
		if c.deliveryMustRelease(attemptCtx) {
			return errors.Join(decodeErr, c.releaseReliableDelivery(queue, delivery))
		}
		return errors.Join(decodeErr, c.deferReliableDelivery(queue, delivery))
	}

	if !c.IsRegisterTask(signature.Name) {
		if c.deliveryMustRelease(attemptCtx) {
			return errors.Join(attemptCtx.Err(), c.releaseReliableDelivery(queue, delivery))
		}
		if signature.IgnoreNotRegisteredTask {
			return c.acknowledgeReliableDelivery(queue, delivery)
		}
		notRegisteredErr := fmt.Errorf("task %q is not registered", signature.Name)
		return errors.Join(notRegisteredErr, c.deferReliableDelivery(queue, delivery))
	}

	if !c.tryStartReliableProcessing(attemptCtx) {
		return errors.Join(attemptCtx.Err(), c.releaseReliableDelivery(queue, delivery))
	}

	renewCtx, cancelRenew := context.WithCancel(context.Background())
	renewDone := make(chan error, 1)
	go func() {
		renewDone <- c.maintainReliableDelivery(renewCtx, queue, delivery)
	}()
	processErr := handler.Process(&signature)
	cancelRenew()
	renewErr := <-renewDone

	if processErr != nil {
		finalizeErr := c.deferReliableDelivery(queue, delivery)
		return errors.Join(fmt.Errorf("process task %q: %w", signature.ID, processErr), renewErr, finalizeErr)
	}
	ackErr := c.acknowledgeReliableDelivery(queue, delivery)
	return errors.Join(renewErr, ackErr)
}

func (c *ControllerRedis) deliveryMustRelease(ctx context.Context) bool {
	c.deliveryMu.Lock()
	defer c.deliveryMu.Unlock()
	return c.stopping || ctx.Err() != nil
}

func (c *ControllerRedis) tryStartReliableProcessing(ctx context.Context) bool {
	c.deliveryMu.Lock()
	defer c.deliveryMu.Unlock()
	return !c.stopping && ctx.Err() == nil
}

func (c *ControllerRedis) releaseReliableDelivery(queue *reliableQueue, delivery *reliableDelivery) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.reliableFinalizationTimeout())
	defer cancel()
	return queue.release(ctx, delivery)
}

func (c *ControllerRedis) deferReliableDelivery(queue *reliableQueue, delivery *reliableDelivery) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.reliableFinalizationTimeout())
	defer cancel()
	_, err := queue.deferRetry(ctx, delivery)
	return err
}

func (c *ControllerRedis) acknowledgeReliableDelivery(queue *reliableQueue, delivery *reliableDelivery) error {
	window := c.ackConfirmationWindow
	if window <= 0 {
		window = consumerRestoreTimeout
	}
	confirmationDeadline := time.Now().Add(window)
	var lastErr error
	for {
		remaining := time.Until(confirmationDeadline)
		if remaining <= 0 {
			if lastErr != nil {
				return lastErr
			}
			return context.DeadlineExceeded
		}
		operationTimeout := c.reliableFinalizationTimeout()
		if remaining < operationTimeout {
			operationTimeout = remaining
		}
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		err := queue.acknowledge(ctx, delivery)
		cancel()
		if err == nil || errors.Is(err, ErrDeliveryLeaseLost) {
			return err
		}
		lastErr = err
		remaining = time.Until(confirmationDeadline)
		if remaining <= 0 {
			return lastErr
		}
		wait := 25 * time.Millisecond
		if remaining < wait {
			wait = remaining
		}
		time.Sleep(wait)
	}
}

func (c *ControllerRedis) maintainReliableDelivery(ctx context.Context, queue *reliableQueue, delivery *reliableDelivery) error {
	var lastRenewErr error
	var renewFailures uint64
	for {
		remaining := time.Until(delivery.confirmedUntil)
		if remaining <= 0 {
			if lastRenewErr != nil {
				return errors.Join(ErrDeliveryLeaseLost, lastRenewErr)
			}
			return ErrDeliveryLeaseLost
		}

		wait := remaining / 2
		if lastRenewErr != nil {
			wait = reliableIdleDelay(25*time.Millisecond, delivery.token, renewFailures)
			if wait > remaining {
				wait = remaining
			}
		}
		if !waitReliablePoll(ctx, wait) {
			return nil
		}

		remaining = time.Until(delivery.confirmedUntil)
		if remaining <= 0 {
			continue
		}
		operationTimeout := remaining / 2
		if operationTimeout > 5*time.Second {
			operationTimeout = 5 * time.Second
		}
		if operationTimeout <= 0 {
			continue
		}
		renewCtx, cancel := context.WithTimeout(ctx, operationTimeout)
		err := queue.renew(renewCtx, delivery)
		cancel()
		if err == nil {
			lastRenewErr = nil
			renewFailures = 0
			continue
		}
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, ErrDeliveryLeaseLost) {
			return err
		}
		lastRenewErr = err
		renewFailures++
	}
}

func (c *ControllerRedis) reliableFinalizationTimeout() time.Duration {
	if c.finalizationTimeout <= 0 {
		return consumerRestoreTimeout
	}
	return c.finalizationTimeout
}

func waitReliablePoll(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func reliableIdleDelay(base time.Duration, queue string, emptyCount uint64) time.Duration {
	seed := make([]byte, len(queue)+8)
	copy(seed, queue)
	binary.BigEndian.PutUint64(seed[len(queue):], emptyCount)
	digest := sha256.Sum256(seed)
	basisPoints := 8000 + int(binary.BigEndian.Uint16(digest[:2]))%4001
	return time.Duration(int64(base) * int64(basisPoints) / 10000)
}

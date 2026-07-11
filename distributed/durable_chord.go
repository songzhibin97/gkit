package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/backend/result"
	"github.com/songzhibin97/gkit/distributed/locker"
	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/tools/rand_string"
)

var ErrDurableTimedChordLockerUnsupported = errors.New("durable timed chord requires context-aware locker")

type DefinitePublishRejection interface {
	error
	RejectedWithoutBrokerSideEffect() bool
}

const (
	chordDispatchInterval    = 250 * time.Millisecond
	chordOperationTimeout    = 5 * time.Second
	chordPublicationLease    = 30 * time.Second
	chordConfirmationTimeout = 30 * time.Second
	chordScanLimit           = 100
)

func (s *Server) sendDurableGroupCallback(ctx context.Context, groupCallback *task.GroupCallback, concurrency int) (*result.GroupCallbackAsyncResult, error) {
	s.lifecycleMu.Lock()
	if s.closing {
		s.lifecycleMu.Unlock()
		return nil, context.Canceled
	}
	startupErr := s.startupErr
	s.lifecycleMu.Unlock()
	if startupErr != nil {
		return nil, startupErr
	}

	group := &task.Group{GroupID: groupCallback.Group.GroupID, Name: groupCallback.Group.Name, Tasks: task.CopySignatures(groupCallback.Group.Tasks...)}
	callback := task.CopySignature(groupCallback.Callback)
	if s.prePublishHandler != nil {
		s.prePublishHandler(callback)
	}
	if callback.ID == "" {
		return nil, fmt.Errorf("%w: pre-publish hook cleared callback id", backend.ErrChordInvalidInput)
	}
	s.bindDefaultRouter(callback)
	deliveryKey := backend.ChordDeliveryKey(group.GroupID, callback.ID)
	if callback.Meta == nil {
		callback.Meta = task.NewMeta(callback.MetaSafe)
	}
	callback.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	callbackPayload, err := json.Marshal(callback)
	if err != nil {
		return nil, fmt.Errorf("serialize durable chord callback: %w", err)
	}
	registration := backend.ChordRegistration{
		DeliveryKey: deliveryKey,
		GroupID:     group.GroupID,
		GroupName:   group.Name,
		Retention:   s.config.ResultExpire,
		Callback:    callbackPayload,
	}
	memberIDs := make(map[string]struct{}, len(group.Tasks))
	for ordinal, member := range group.Tasks {
		if s.prePublishHandler != nil {
			s.prePublishHandler(member)
		}
		member.GroupID = group.GroupID
		member.GroupTaskCount = len(group.Tasks)
		member.CallbackChord = nil
		if member.Meta == nil {
			member.Meta = task.NewMeta(member.MetaSafe)
		}
		member.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
		member.Meta.Set(backend.DurableChordMemberMeta, true)
		member.Meta.Set(backend.DurableChordMemberOrdinal, ordinal)
		if member.ID == "" {
			return nil, fmt.Errorf("%w: pre-publish hook cleared member id at ordinal %d", backend.ErrChordInvalidInput, ordinal)
		}
		if _, exists := memberIDs[member.ID]; exists {
			return nil, fmt.Errorf("%w: pre-publish hook created duplicate member id %q", backend.ErrChordInvalidInput, member.ID)
		}
		memberIDs[member.ID] = struct{}{}
		s.bindDefaultRouter(member)
		payload, marshalErr := json.Marshal(member)
		if marshalErr != nil {
			return nil, fmt.Errorf("serialize durable chord member %d: %w", ordinal, marshalErr)
		}
		registration.Members = append(registration.Members, backend.ChordMemberRegistration{Ordinal: ordinal, TaskID: member.ID, Payload: payload})
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		return nil, err
	}
	ref, err := s.durableBackend.RegisterChord(ctx, registration)
	if err != nil {
		return nil, err
	}
	if ref.Created {
		if err := s.durableBackend.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
			abortErr := s.durableBackend.AbortRegistration(ctx, ref)
			// Reconcile uses create-if-absent writes. Without ownership markers on
			// legacy group/task rows, deleting them here could erase state owned by
			// another caller. A later identical registration safely reuses them.
			return nil, errors.Join(err, abortErr)
		}
	} else {
		if err := s.waitForChordRegistration(ctx, ref); err != nil {
			return nil, err
		}
		return result.NewGroupCallbackAsyncResult(group.Tasks, callback, s.backend), nil
	}

	if concurrency < 1 {
		concurrency = 1
	}
	var publishErrs []error
	var publishErrsMu sync.Mutex
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for ordinal := range group.Tasks {
		if err := ctx.Err(); err != nil {
			publishErrsMu.Lock()
			publishErrs = append(publishErrs, err)
			publishErrsMu.Unlock()
			break
		}
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
			publishErrsMu.Lock()
			publishErrs = append(publishErrs, ctx.Err())
			publishErrsMu.Unlock()
			break
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(ordinal int) {
			defer wg.Done()
			defer func() { <-semaphore }()
			if err := s.publishChordMember(ctx, ref.DeliveryKey, ordinal); err != nil {
				publishErrsMu.Lock()
				publishErrs = append(publishErrs, err)
				publishErrsMu.Unlock()
			}
		}(ordinal)
	}
	wg.Wait()
	async := result.NewGroupCallbackAsyncResult(group.Tasks, callback, s.backend)
	return async, errors.Join(publishErrs...)
}

func (s *Server) waitForChordRegistration(ctx context.Context, ref backend.ChordRegistrationRef) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		cursor := ""
		found := false
		for {
			page, err := s.durableBackend.ScanChordDeliveries(ctx, backend.ChordScan{Cursor: cursor, Limit: chordScanLimit, Now: time.Now()})
			if err != nil {
				return err
			}
			for _, delivery := range page.Deliveries {
				if delivery.DeliveryKey != ref.DeliveryKey {
					continue
				}
				found = true
				if delivery.RegistrationOwner != ref.Owner || delivery.RegistrationVersion != ref.Version {
					return backend.ErrChordRegistrationAborted
				}
				ready := true
				for _, member := range delivery.Members {
					if member.State == backend.ChordMemberSetup {
						ready = false
						break
					}
				}
				if ready {
					return nil
				}
				break
			}
			if found || page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		if !found {
			return backend.ErrChordRegistrationAborted
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Server) startChordDispatcher() error {
	if err := s.runChordPass(s.chordCtx); err != nil {
		return fmt.Errorf("durable chord startup scan: %w", err)
	}
	s.chordWG.Add(1)
	go func() {
		defer s.chordWG.Done()
		ticker := time.NewTicker(chordDispatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.chordCtx.Done():
				return
			case <-ticker.C:
				if err := s.runChordPass(s.chordCtx); err != nil {
					s.recordLifecycleError(fmt.Errorf("durable chord dispatcher: %w", err))
				}
			}
		}
	}()
	return nil
}

func (s *Server) runChordPass(ctx context.Context) error {
	owner, err := backend.NewChordOwner()
	if err != nil {
		return err
	}
	cursor := ""
	for {
		operationCtx, cancel := context.WithTimeout(ctx, chordOperationTimeout)
		page, scanErr := s.durableBackend.ScanChordDeliveries(operationCtx, backend.ChordScan{Cursor: cursor, Limit: chordScanLimit, Now: time.Now()})
		cancel()
		if scanErr != nil {
			return scanErr
		}
		for index := range page.Deliveries {
			delivery := &page.Deliveries[index]
			operationCtx, cancel = context.WithTimeout(ctx, chordOperationTimeout)
			reconcileErr := s.durableBackend.ReconcileChord(operationCtx, delivery.DeliveryKey)
			cancel()
			if reconcileErr != nil {
				return reconcileErr
			}
			for ordinal := range delivery.Members {
				operationCtx, cancel = context.WithTimeout(ctx, chordOperationTimeout)
				publishErr := s.publishChordMemberWithOwner(operationCtx, delivery.DeliveryKey, ordinal, owner)
				cancel()
				if publishErr != nil && !errors.Is(publishErr, context.Canceled) {
					s.recordLifecycleError(publishErr)
				}
			}
			operationCtx, cancel = context.WithTimeout(ctx, chordOperationTimeout)
			publishErr := s.publishChordCallbackWithOwner(operationCtx, delivery.DeliveryKey, owner)
			cancel()
			if publishErr != nil && !errors.Is(publishErr, context.Canceled) {
				s.recordLifecycleError(publishErr)
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	operationCtx, cancel := context.WithTimeout(ctx, chordOperationTimeout)
	_, cleanupErr := s.durableBackend.CleanupTerminalChordDeliveries(operationCtx, time.Now(), chordScanLimit)
	cancel()
	return cleanupErr
}

func (s *Server) publishChordMember(ctx context.Context, deliveryKey string, ordinal int) error {
	owner, err := backend.NewChordOwner()
	if err != nil {
		return err
	}
	return s.publishChordMemberWithOwner(ctx, deliveryKey, ordinal, owner)
}

func (s *Server) publishChordMemberWithOwner(ctx context.Context, deliveryKey string, ordinal int, owner string) error {
	now := time.Now()
	lease, claimed, err := s.durableBackend.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: deliveryKey, Ordinal: ordinal, Owner: owner, Now: now, LeaseDuration: chordPublicationLease})
	if err != nil || !claimed {
		return err
	}
	var signature task.Signature
	if err := json.Unmarshal(lease.Payload, &signature); err != nil {
		recordCtx, cancel := s.newChordOutcomeContext()
		defer cancel()
		return s.durableBackend.RecordMemberPublishOutcome(recordCtx, lease, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeRejected, Now: time.Now(), Error: err.Error()})
	}
	publishErr := s.controller.Publish(ctx, &signature)
	outcome := classifyChordPublishOutcome(publishErr, time.Now())
	recordCtx, cancel := s.newChordOutcomeContext()
	recordErr := s.durableBackend.RecordMemberPublishOutcome(recordCtx, lease, outcome)
	cancel()
	return errors.Join(publishErr, recordErr)
}

func (s *Server) publishChordCallbackWithOwner(ctx context.Context, deliveryKey, owner string) error {
	now := time.Now()
	lease, claimed, err := s.durableBackend.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: deliveryKey, Owner: owner, Now: now, LeaseDuration: chordPublicationLease})
	if err != nil || !claimed {
		return err
	}
	var signature task.Signature
	if err := json.Unmarshal(lease.Payload, &signature); err != nil {
		recordCtx, cancel := s.newChordOutcomeContext()
		defer cancel()
		return s.durableBackend.RecordCallbackPublishOutcome(recordCtx, lease, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeRejected, Now: time.Now(), Error: err.Error(), NextAttemptAt: time.Now().Add(time.Second)})
	}
	publishErr := s.controller.Publish(ctx, &signature)
	outcome := classifyChordPublishOutcome(publishErr, time.Now())
	recordCtx, cancel := s.newChordOutcomeContext()
	recordErr := s.durableBackend.RecordCallbackPublishOutcome(recordCtx, lease, outcome)
	cancel()
	return errors.Join(publishErr, recordErr)
}

func (s *Server) newChordOutcomeContext() (context.Context, context.CancelFunc) {
	root := s.chordOutcomeCtx
	if root == nil {
		root = context.Background()
	}
	return context.WithTimeout(root, chordOperationTimeout)
}

func classifyChordPublishOutcome(err error, now time.Time) backend.ChordPublishOutcome {
	if err == nil {
		return backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now, ConfirmationDeadline: now.Add(chordConfirmationTimeout)}
	}
	var rejection DefinitePublishRejection
	if errors.As(err, &rejection) && rejection.RejectedWithoutBrokerSideEffect() {
		return backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeRejected, Now: now, NextAttemptAt: now.Add(time.Second), Error: err.Error()}
	}
	return backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeUnknown, Now: now, ConfirmationDeadline: now.Add(chordConfirmationTimeout), Error: err.Error()}
}

func (s *Server) recordLifecycleError(err error) {
	if err == nil {
		return
	}
	s.lifecycleMu.Lock()
	s.lifecycleErrs = append(s.lifecycleErrs, err)
	s.lifecycleMu.Unlock()
}

func (s *Server) shutdown(ctx context.Context) error {
	s.lifecycleMu.Lock()
	s.closing = true
	s.lifecycleMu.Unlock()
	if s.registrationCancel != nil {
		s.registrationCancel()
	}
	if s.cleanupRootCancel != nil {
		s.cleanupRootCancel()
	}
	stopCtx := s.scheduler.Stop()
	if err := waitForContext(ctx, stopCtx); err != nil {
		s.recordLifecycleError(err)
	}
	if err := waitForGroup(ctx, &s.registrationWG); err != nil {
		s.recordLifecycleError(err)
	}
	if s.chordCancel != nil {
		s.chordCancel()
	}
	if err := waitForGroup(ctx, &s.chordWG); err != nil {
		s.recordLifecycleError(err)
	}
	if s.chordOutcomeCancel != nil {
		s.chordOutcomeCancel()
	}
	s.lifecycleMu.Lock()
	errs := append([]error(nil), s.lifecycleErrs...)
	s.lifecycleMu.Unlock()
	if ctx.Err() != nil {
		errs = append(errs, ctx.Err())
	}
	return errors.Join(errs...)
}

func waitForContext(ctx context.Context, target context.Context) error {
	select {
	case <-target.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func waitForGroup(ctx context.Context, group *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) contextLocker() (locker.ContextLocker, bool) {
	value, ok := s.lock.(locker.ContextLocker)
	return value, ok
}

func (s *Server) runDurableTimedGroupCallback(contextLock locker.ContextLocker, schedule cron.Schedule, spec, name, groupID string, concurrency int, callback *task.Signature, signatures ...*task.Signature) (returnErr error) {
	s.lifecycleMu.Lock()
	if s.closing {
		s.lifecycleMu.Unlock()
		return context.Canceled
	}
	s.registrationWG.Add(1)
	s.lifecycleMu.Unlock()
	defer s.registrationWG.Done()

	ctx, cancel := context.WithTimeout(s.registrationCtx, s.config.DurableChordRegistrationTimeout)
	defer cancel()
	runSuffix := rand_string.RandomLetter(timedRunSuffixLength)
	groupCallback := newTimedGroupCallbackRun(groupID, name, runSuffix, callback, signatures...)
	key := getLockName(name, spec)
	mark := rand_string.RandomLetter(16)
	ttl := timedTaskLockTTL(time.Until(schedule.Next(time.Now())))
	if err := contextLock.LockContext(ctx, key, ttl, mark); err != nil {
		return fmt.Errorf("lock durable timed chord %q: %w", name, err)
	}
	lockExpiresAt := time.Now().Add(time.Duration(ttl) * time.Millisecond)
	defer func() {
		remaining := time.Until(lockExpiresAt)
		if remaining <= 0 {
			unlockErr := fmt.Errorf("unlock durable timed chord %q: lock expired", name)
			s.recordLifecycleError(unlockErr)
			returnErr = errors.Join(returnErr, unlockErr)
			return
		}
		cleanupTimeout := 5 * time.Second
		if remaining < cleanupTimeout {
			cleanupTimeout = remaining
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(s.cleanupRootCtx, cleanupTimeout)
		defer cleanupCancel()
		if err := contextLock.UnlockContext(cleanupCtx, key, mark); err != nil {
			unlockErr := fmt.Errorf("unlock durable timed chord %q: %w", name, err)
			s.recordLifecycleError(unlockErr)
			returnErr = errors.Join(returnErr, unlockErr)
		}
	}()
	if _, err := s.SendGroupCallbackWithContext(ctx, groupCallback, concurrency); err != nil {
		return fmt.Errorf("send durable timed chord %q: %w", name, err)
	}
	return nil
}

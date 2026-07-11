package tcp

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type sendRecvResult struct {
	data []byte
	err  error
}

func TestSendRecvWithTimeoutBoundsSendAndClearsDeadline(t *testing.T) {
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()
	conn := NewConnByNetConn(client)

	const timeout = 10 * time.Millisecond
	result := make(chan sendRecvResult, 1)
	started := time.Now()
	go func() {
		data, err := conn.SendRecvWithTimeout([]byte("blocked"), timeout, 1, nil)
		result <- sendRecvResult{data: data, err: err}
	}()

	var outcome sendRecvResult
	select {
	case outcome = <-result:
	case <-time.After(200 * time.Millisecond):
		// The old implementation has no write deadline. Close the peer before
		// failing so the blocked write exits and the test leaves no goroutine.
		_ = peer.Close()
		outcome = <-result
		t.Fatalf("SendRecvWithTimeout remained blocked in Send for %v: %v", time.Since(started), outcome.err)
	}
	if outcome.err == nil {
		t.Fatal("SendRecvWithTimeout returned nil error while the peer read nothing")
	}
	var netErr net.Error
	if !errors.As(outcome.err, &netErr) || !netErr.Timeout() {
		t.Fatalf("SendRecvWithTimeout error = %v, want timeout error", outcome.err)
	}
	if !strings.Contains(outcome.err.Error(), "tcp send") {
		t.Fatalf("SendRecvWithTimeout error = %q, want send context", outcome.err)
	}
	if !conn.sendTimeout.IsZero() || !conn.recvTimeout.IsZero() {
		t.Fatalf("deadlines after SendRecvWithTimeout = (send %v, recv %v), want cleared", conn.sendTimeout, conn.recvTimeout)
	}

	peerDone := make(chan error, 1)
	go func() {
		request := make([]byte, 2)
		if _, err := peer.Read(request); err != nil {
			peerDone <- err
			return
		}
		_, err := peer.Write([]byte("r"))
		peerDone <- err
	}()
	response, err := conn.SendRecv([]byte("ok"), 1, nil)
	if err != nil {
		t.Fatalf("connection remained constrained after timeout: %v", err)
	}
	if string(response) != "r" {
		t.Fatalf("response after cleared deadline = %q, want %q", response, "r")
	}
	if err := <-peerDone; err != nil {
		t.Fatalf("peer exchange after cleared deadline: %v", err)
	}
}

func TestSendRecvWithTimeoutUsesSingleBudget(t *testing.T) {
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()
	conn := NewConnByNetConn(client)

	const (
		timeout   = 200 * time.Millisecond
		sendDelay = 75 * time.Millisecond
		maxTotal  = 250 * time.Millisecond
	)
	peerReady := make(chan struct{})
	requestRead := make(chan struct{})
	releasePeer := make(chan struct{})
	peerDone := make(chan error, 1)
	go func() {
		close(peerReady)
		time.Sleep(sendDelay)
		request := make([]byte, len("request"))
		if _, err := peer.Read(request); err != nil {
			peerDone <- err
			return
		}
		close(requestRead)
		<-releasePeer
		peerDone <- nil
	}()
	<-peerReady

	started := time.Now()
	_, err := conn.SendRecvWithTimeout([]byte("request"), timeout, 1, nil)
	elapsed := time.Since(started)
	close(releasePeer)
	if peerErr := <-peerDone; peerErr != nil {
		t.Fatalf("peer read: %v", peerErr)
	}
	select {
	case <-requestRead:
	default:
		t.Fatal("peer did not read request; receive-phase budget was not exercised")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("SendRecvWithTimeout error = %v, want receive timeout", err)
	}
	if elapsed > maxTotal {
		t.Fatalf("SendRecvWithTimeout took %v, want one %v budget (max %v)", elapsed, timeout, maxTotal)
	}
}

package distributed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/log"
)

var _ backend.Backend = (*groupTestBackend)(nil)

type issue104DefiniteRejection struct{ definite bool }

func (e issue104DefiniteRejection) Error() string { return "publish rejected" }
func (e issue104DefiniteRejection) RejectedWithoutBrokerSideEffect() bool {
	return e.definite
}

type issue104FailingScanBackend struct {
	backend.Backend
	backend.DurableChordBackend
	err error
}

type issue104FailOnceBackend struct {
	backend.Backend
	backend.DurableChordBackend
	mu                    sync.Mutex
	memberOutcomeFails    int
	memberTerminalFails   int
	callbackTerminalFails int
	forceLease            time.Duration
	forceConfirmation     time.Duration
}

func (b *issue104FailOnceBackend) consume(counter *int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if *counter <= 0 {
		return false
	}
	*counter--
	return true
}

func (b *issue104FailOnceBackend) ClaimMemberPublication(ctx context.Context, claim backend.ChordMemberClaim) (backend.ChordMemberLease, bool, error) {
	if b.forceLease > 0 {
		claim.LeaseDuration = b.forceLease
	}
	return b.DurableChordBackend.ClaimMemberPublication(ctx, claim)
}

func (b *issue104FailOnceBackend) RecordMemberPublishOutcome(ctx context.Context, lease backend.ChordMemberLease, outcome backend.ChordPublishOutcome) error {
	if b.consume(&b.memberOutcomeFails) {
		return errors.New("injected member outcome write failure")
	}
	if b.forceConfirmation > 0 && (outcome.Kind == backend.ChordPublishOutcomeSucceeded || outcome.Kind == backend.ChordPublishOutcomeUnknown) {
		outcome.ConfirmationDeadline = outcome.Now.Add(b.forceConfirmation)
	}
	return b.DurableChordBackend.RecordMemberPublishOutcome(ctx, lease, outcome)
}

func (b *issue104FailOnceBackend) RecordCallbackPublishOutcome(ctx context.Context, lease backend.ChordCallbackLease, outcome backend.ChordPublishOutcome) error {
	if b.forceConfirmation > 0 && (outcome.Kind == backend.ChordPublishOutcomeSucceeded || outcome.Kind == backend.ChordPublishOutcomeUnknown) {
		outcome.ConfirmationDeadline = outcome.Now.Add(b.forceConfirmation)
	}
	return b.DurableChordBackend.RecordCallbackPublishOutcome(ctx, lease, outcome)
}

func (b *issue104FailOnceBackend) RecordMemberTerminal(ctx context.Context, key string, ordinal int, taskID string, outcome backend.MemberTerminalOutcome, results []*task.Result) error {
	if b.consume(&b.memberTerminalFails) {
		return errors.New("injected member receipt write failure")
	}
	return b.DurableChordBackend.RecordMemberTerminal(ctx, key, ordinal, taskID, outcome, results)
}

func (b *issue104FailOnceBackend) RecordCallbackTerminal(ctx context.Context, key string, outcome backend.CallbackTerminalOutcome) error {
	if b.consume(&b.callbackTerminalFails) {
		return errors.New("injected callback terminal write failure")
	}
	return b.DurableChordBackend.RecordCallbackTerminal(ctx, key, outcome)
}

func (b *issue104FailingScanBackend) ScanChordDeliveries(context.Context, backend.ChordScan) (backend.ChordDeliveryPage, error) {
	return backend.ChordDeliveryPage{}, b.err
}

type issue104BlockingContextLocker struct {
	unlockEntered chan struct{}
	ttl           int
}

func (l *issue104BlockingContextLocker) Lock(string, int, string) error  { return nil }
func (l *issue104BlockingContextLocker) UnLock(string, string) error     { return nil }
func (l *issue104BlockingContextLocker) Renew(string, int, string) error { return nil }
func (l *issue104BlockingContextLocker) LockContext(_ context.Context, _ string, ttl int, _ string) error {
	l.ttl = ttl
	return nil
}
func (l *issue104BlockingContextLocker) UnlockContext(ctx context.Context, _, _ string) error {
	select {
	case l.unlockEntered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestDurableChordEndToEndUsesReceiptsAndDeliveryKey(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	durable := backendValue.(backend.DurableChordBackend)
	published := make(chan *task.Signature, 16)
	prePublishCalls := make(map[string]int)
	var prePublishMu sync.Mutex
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		published <- task.CopySignature(signature)
		return nil
	}}
	server, err := NewServerE(
		controller,
		backendValue,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		func(signature *task.Signature) {
			prePublishMu.Lock()
			prePublishCalls[signature.ID]++
			prePublishMu.Unlock()
			signature.Meta = nil
			if signature.Name == "member" {
				signature.CallbackChord = task.NewSignature("legacy-callback", "callback")
				signature.GroupID = "hook-overwrite"
			}
			signature.Router = "durable-route"
		},
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	if err := server.RegisteredTask("member", func() (string, error) { return "member-result", nil }); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisteredTask("callback", func(string, string) (string, error) { return "callback-result", nil }); err != nil {
		t.Fatal(err)
	}
	memberA := task.NewSignature("member-a", "member")
	memberB := task.NewSignature("member-b", "member")
	group, _ := task.NewGroup("durable-group", "durable-group", memberA, memberB)
	callback := task.NewSignature("shared-callback", "callback")
	groupCallback, _ := task.NewGroupCallback(group, "durable-group", callback)

	if _, err := server.SendGroupCallbackWithContext(context.Background(), groupCallback, 2); err != nil {
		t.Fatalf("SendGroupCallbackWithContext: %v", err)
	}
	if memberA.CallbackChord != callback || memberB.CallbackChord != callback {
		t.Fatal("durable send mutated caller member templates")
	}
	if memberA.Router != "" || memberB.Router != "" || callback.Router != "" {
		t.Fatal("durable send mutated caller routing templates")
	}

	worker := server.NewWorker("issue104", 1, "queue")
	seenMembers := map[string]bool{}
	for len(seenMembers) < 2 {
		select {
		case signature := <-published:
			if signature.Name != "member" {
				continue
			}
			if signature.CallbackChord != nil {
				t.Fatal("durable member retained legacy CallbackChord")
			}
			if signature.GroupID != group.GroupID {
				t.Fatalf("durable member group = %q, want %q", signature.GroupID, group.GroupID)
			}
			if marker, ok := signature.Meta.Get(backend.DurableChordMemberMeta); !ok || !metadataBool(marker) {
				t.Fatal("durable member hook overwrote reserved marker")
			}
			if key, ok := signature.Meta.Get(backend.DurableChordDeliveryKeyMeta); !ok || key == "" {
				t.Fatal("durable member hook overwrote delivery key")
			}
			if signature.Router != "durable-route" {
				t.Fatalf("durable member route = %q", signature.Router)
			}
			if err := worker.Process(signature); err != nil {
				t.Fatalf("process durable member %s: %v", signature.ID, err)
			}
			seenMembers[signature.ID] = true
		case <-time.After(2 * time.Second):
			t.Fatal("durable members were not published")
		}
	}

	var callbackSignature *task.Signature
	select {
	case callbackSignature = <-published:
		for callbackSignature.Name != "callback" {
			select {
			case callbackSignature = <-published:
			case <-time.After(2 * time.Second):
				t.Fatal("durable callback was not published")
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("durable callback was not published")
	}
	value, ok := callbackSignature.Meta.Get(backend.DurableChordDeliveryKeyMeta)
	if !ok || value == "" {
		t.Fatal("durable callback lacks delivery key metadata")
	}
	if callbackSignature.Router != "durable-route" {
		t.Fatalf("durable callback route = %q", callbackSignature.Router)
	}
	if err := worker.Process(callbackSignature); err != nil {
		t.Fatalf("process durable callback: %v", err)
	}

	deliveryKey := value.(string)
	deadline := time.Now().Add(2 * time.Second)
	for {
		page, err := durable.ScanChordDeliveries(context.Background(), backend.ChordScan{Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, delivery := range page.Deliveries {
			if delivery.DeliveryKey == deliveryKey {
				found = true
				if delivery.CallbackState == backend.ChordDelivered {
					prePublishMu.Lock()
					defer prePublishMu.Unlock()
					for _, id := range []string{memberA.ID, memberB.ID, callback.ID} {
						if prePublishCalls[id] != 1 {
							t.Fatalf("pre-publish calls for %s = %d, want exactly once", id, prePublishCalls[id])
						}
					}
					return
				}
			}
		}
		if !found {
			t.Fatal("durable delivery disappeared")
		}
		if time.Now().After(deadline) {
			t.Fatal("callback terminal outcome was not recorded by delivery key")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDurableTimedChordRejectsLegacyLocker(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	server, err := NewServerE(
		&groupTestController{},
		backendValue,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Cleanup(func() { _ = server.Shutdown(ctx) })
	err = server.RegisteredTimedGroupCallback("* * * * *", "durable", "group", 1, task.NewSignature("callback", "callback"), task.NewSignature("member", "member"))
	if !errors.Is(err, ErrDurableTimedChordLockerUnsupported) {
		t.Fatalf("RegisteredTimedGroupCallback error = %v", err)
	}
}

func TestShutdownCancelsUnlockAlreadyInProgress(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	contextLock := &issue104BlockingContextLocker{unlockEntered: make(chan struct{}, 1)}
	server, err := NewServerE(
		&groupTestController{},
		backendValue,
		contextLock,
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := cron.ParseStandard("* * * * *")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- server.runDurableTimedGroupCallback(
			contextLock,
			schedule,
			"* * * * *",
			"durable",
			"group",
			1,
			task.NewSignature("callback", "callback"),
			task.NewSignature("member", "member"),
		)
	}()
	select {
	case <-contextLock.unlockEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("UnlockContext did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	shutdownErr := server.Shutdown(ctx)
	if !errors.Is(shutdownErr, context.Canceled) {
		t.Fatalf("Shutdown error = %v, want retained unlock cancellation", shutdownErr)
	}
	select {
	case runErr := <-done:
		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("scheduled run error = %v, want context cancellation", runErr)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled run survived Shutdown")
	}
	if contextLock.ttl <= 0 {
		t.Fatalf("lock TTL = %d, want finite positive fallback", contextLock.ttl)
	}
}

func TestDurableChordPublishOutcomeClassification(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name string
		err  error
		want backend.ChordPublishOutcomeKind
	}{
		{name: "nil", want: backend.ChordPublishOutcomeSucceeded},
		{name: "ordinary", err: errors.New("enqueue then error"), want: backend.ChordPublishOutcomeUnknown},
		{name: "canceled", err: context.Canceled, want: backend.ChordPublishOutcomeUnknown},
		{name: "typed false", err: issue104DefiniteRejection{}, want: backend.ChordPublishOutcomeUnknown},
		{name: "typed true", err: issue104DefiniteRejection{definite: true}, want: backend.ChordPublishOutcomeRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome := classifyChordPublishOutcome(tc.err, now)
			if outcome.Kind != tc.want {
				t.Fatalf("outcome = %s, want %s", outcome.Kind, tc.want)
			}
			if tc.err != nil && tc.want != backend.ChordPublishOutcomeRejected && outcome.ConfirmationDeadline.IsZero() {
				t.Fatal("unknown outcome has no confirmation deadline")
			}
		})
	}
}

func TestDurableChordCompatibilityMatrix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enable  bool
		strict  bool
		wantErr error
	}{
		{name: "disabled compatible", enable: false, strict: false},
		{name: "disabled strict", enable: false, strict: true},
		{name: "enabled compatible", enable: true, strict: false},
		{name: "enabled strict", enable: true, strict: true, wantErr: backend.ErrDurableChordUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backendValue := &groupTestBackend{}
			controller := &groupTestController{}
			server, err := NewServerE(
				controller,
				backendValue,
				timedGroupTestLocker{},
				log.NewHelper(log.DefaultLogger),
				nil,
				SetEnableDurableChordRegistration(tc.enable),
				SetRequireDurableChordBackend(tc.strict),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = server.Shutdown(ctx)
			})
			group, _ := task.NewGroup("compatibility-"+tc.name, "group", task.NewSignature("member-"+tc.name, "member"))
			groupCallback, _ := task.NewGroupCallback(group, "group", task.NewSignature("callback-"+tc.name, "callback"))
			_, sendErr := server.SendGroupCallbackWithContext(context.Background(), groupCallback, 1)
			if !errors.Is(sendErr, tc.wantErr) {
				t.Fatalf("send error = %v, want %v", sendErr, tc.wantErr)
			}
			wantPublishes := int64(1)
			if tc.wantErr != nil {
				wantPublishes = 0
			}
			if got := controller.publishCount.Load(); got != wantPublishes {
				t.Fatalf("publish count = %d, want %d", got, wantPublishes)
			}
			timedErr := server.RegisteredTimedGroupCallback("* * * * *", "compatibility-"+tc.name, "timed-group", 1, task.NewSignature("timed-callback", "callback"), task.NewSignature("timed-member", "member"))
			if !errors.Is(timedErr, tc.wantErr) {
				t.Fatalf("timed registration error = %v, want %v", timedErr, tc.wantErr)
			}
		})
	}
}

func TestNonDurableTimedChordAcceptsLegacyLocker(t *testing.T) {
	server, err := NewServerE(
		&groupTestController{},
		&groupTestBackend{},
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Cleanup(func() { _ = server.Shutdown(ctx) })
	if err := server.RegisteredTimedGroupCallback("* * * * *", "legacy", "group", 1, task.NewSignature("callback", "callback"), task.NewSignature("member", "member")); err != nil {
		t.Fatalf("legacy timed chord registration: %v", err)
	}
}

func TestDurableChordStartupRecoversEveryScanPageWhenRegistrationDisabled(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	durable := backendValue.(backend.DurableChordBackend)
	const registrations = chordScanLimit + 1
	stale := time.Now().Add(-time.Minute)
	for index := 0; index < registrations; index++ {
		registration, _ := issue104Registration(t, fmt.Sprintf("startup-group-%03d", index), "shared-startup-callback", "member")
		ref, err := durable.RegisterChord(context.Background(), registration)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 || index > 10 {
			continue // MEMBER_SETUP / WAITING fixtures span both scan pages.
		}
		if err := durable.ReconcileChord(context.Background(), ref.DeliveryKey); err != nil {
			t.Fatal(err)
		}
		switch index {
		case 1: // MEMBER_READY
		case 2: // expired MEMBER_LEASED
			if _, claimed, err := durable.ClaimMemberPublication(context.Background(), backend.ChordMemberClaim{DeliveryKey: ref.DeliveryKey, Ordinal: 0, Owner: "stale-member", Now: stale, LeaseDuration: time.Second}); err != nil || !claimed {
				t.Fatalf("member leased fixture = %t, %v", claimed, err)
			}
		case 3, 4: // overdue MEMBER_PUBLISHED / MEMBER_PUBLISH_UNKNOWN
			lease, claimed, err := durable.ClaimMemberPublication(context.Background(), backend.ChordMemberClaim{DeliveryKey: ref.DeliveryKey, Ordinal: 0, Owner: "member-outcome", Now: stale, LeaseDuration: time.Second})
			if err != nil || !claimed {
				t.Fatalf("member outcome fixture = %t, %v", claimed, err)
			}
			kind := backend.ChordPublishOutcomeSucceeded
			if index == 4 {
				kind = backend.ChordPublishOutcomeUnknown
			}
			if err := durable.RecordMemberPublishOutcome(context.Background(), lease, backend.ChordPublishOutcome{Kind: kind, Now: stale, ConfirmationDeadline: stale.Add(time.Second)}); err != nil {
				t.Fatal(err)
			}
		case 5, 6, 7, 8, 9: // callback READY/LEASED/PUBLISHED/UNKNOWN/DELIVERED
			if err := durable.RecordMemberTerminal(context.Background(), ref.DeliveryKey, 0, registration.Members[0].TaskID, backend.MemberTerminalSuccess, nil); err != nil {
				t.Fatal(err)
			}
			if index == 5 {
				break
			}
			if index == 9 {
				if err := durable.RecordCallbackTerminal(context.Background(), ref.DeliveryKey, backend.CallbackTerminalSuccess); err != nil {
					t.Fatal(err)
				}
				break
			}
			lease, claimed, err := durable.ClaimCallbackPublication(context.Background(), backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: "stale-callback", Now: stale, LeaseDuration: time.Second})
			if err != nil || !claimed {
				t.Fatalf("callback fixture = %t, %v", claimed, err)
			}
			if index == 6 {
				break
			}
			kind := backend.ChordPublishOutcomeSucceeded
			if index == 8 {
				kind = backend.ChordPublishOutcomeUnknown
			}
			if err := durable.RecordCallbackPublishOutcome(context.Background(), lease, backend.ChordPublishOutcome{Kind: kind, Now: stale, ConfirmationDeadline: stale.Add(time.Second)}); err != nil {
				t.Fatal(err)
			}
		case 10: // SUPPRESSED
			if err := durable.RecordMemberTerminal(context.Background(), ref.DeliveryKey, 0, registration.Members[0].TaskID, backend.MemberTerminalFailure, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	controller := &groupTestController{}
	server, err := NewServerE(
		controller,
		backendValue,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Cleanup(func() { _ = server.Shutdown(ctx) })
	const expectedPublications = 99 // 95 recoverable members + 4 recoverable callbacks.
	if got := controller.publishCount.Load(); got != expectedPublications {
		t.Fatalf("startup published %d items, want %d across every state and scan page", got, expectedPublications)
	}
}

func TestDurableChordStartupFailureIsReturnedAndStored(t *testing.T) {
	startupErr := errors.New("scan unavailable")
	newFailing := func(t *testing.T) *issue104FailingScanBackend {
		t.Helper()
		base := newGroupConvergenceRedisBackend(t)
		return &issue104FailingScanBackend{Backend: base, DurableChordBackend: base.(backend.DurableChordBackend), err: startupErr}
	}
	server, err := NewServerE(
		&groupTestController{},
		newFailing(t),
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if !errors.Is(err, startupErr) {
		t.Fatalf("NewServerE error = %v, want startup scan failure", err)
	}
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = server.Shutdown(ctx)
		cancel()
	}

	compatibilityServer := NewServer(
		&groupTestController{},
		newFailing(t),
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Cleanup(func() { _ = compatibilityServer.Shutdown(ctx) })
	registration, callback := issue104Registration(t, "stored-error-group", "callback", "member")
	var member task.Signature
	if err := json.Unmarshal(registration.Members[0].Payload, &member); err != nil {
		t.Fatal(err)
	}
	group, _ := task.NewGroup(registration.GroupID, registration.GroupName, &member)
	groupCallback, _ := task.NewGroupCallback(group, registration.GroupName, callback)
	if _, err := compatibilityServer.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); !errors.Is(err, startupErr) {
		t.Fatalf("compatibility NewServer durable send error = %v, want stored startup error", err)
	}
	if err := compatibilityServer.RegisteredTimedGroupCallback("* * * * *", "stored-error", "group", 1, callback, &member); !errors.Is(err, startupErr) {
		t.Fatalf("compatibility NewServer timed registration error = %v, want stored startup error", err)
	}
}

func TestDurableChordAttachedRegistrationObservesCreatorAbort(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	durable := backendValue.(backend.DurableChordBackend)
	registration, _ := issue104Registration(t, "attached-abort-group", "callback", "member")
	ref, err := durable.RegisterChord(context.Background(), registration)
	if err != nil {
		t.Fatal(err)
	}
	attached, err := durable.RegisterChord(context.Background(), registration)
	if err != nil || attached.Created {
		t.Fatalf("attached registration = %#v, %v", attached, err)
	}
	server := &Server{durableBackend: durable}
	if err := durable.AbortRegistration(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	recreated, err := durable.RegisterChord(context.Background(), registration)
	if err != nil || !recreated.Created {
		t.Fatalf("recreated registration = %#v, %v", recreated, err)
	}
	if err := durable.ReconcileChord(context.Background(), recreated.DeliveryKey); err != nil {
		t.Fatal(err)
	}
	if waitErr := server.waitForChordRegistration(context.Background(), attached); !errors.Is(waitErr, backend.ErrChordRegistrationAborted) {
		t.Fatalf("attached registration error = %v, want creator generation abort", waitErr)
	}
}

func TestShutdownCancelsAndJoinsDirectDurablePublication(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	publishEntered := make(chan struct{}, 1)
	controller := &groupTestController{publishFn: func(ctx context.Context, _ *task.Signature) error {
		publishEntered <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}}
	server, err := NewServerE(
		controller,
		backendValue,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	group, _ := task.NewGroup("shutdown-publish-group", "group", task.NewSignature("shutdown-member", "member"))
	groupCallback, _ := task.NewGroupCallback(group, "group", task.NewSignature("shutdown-callback", "callback"))
	sendDone := make(chan error, 1)
	go func() {
		_, sendErr := server.SendGroupCallbackWithContext(context.Background(), groupCallback, 1)
		sendDone <- sendErr
	}()
	select {
	case <-publishEntered:
	case <-time.After(time.Second):
		t.Fatal("durable member Publish did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	select {
	case sendErr := <-sendDone:
		if !errors.Is(sendErr, context.Canceled) {
			t.Fatalf("durable send error = %v, want shutdown cancellation", sendErr)
		}
	case <-time.After(time.Second):
		t.Fatal("durable publication goroutine survived Shutdown")
	}
	if active := controller.active.Load(); active != 0 {
		t.Fatalf("active publishers after Shutdown = %d", active)
	}
	deliveryKey := backend.ChordDeliveryKey(group.GroupID, groupCallback.Callback.ID)
	delivery := issue104FindDelivery(t, backendValue.(backend.DurableChordBackend), deliveryKey)
	if delivery.Members[0].State != backend.ChordMemberPublishUnknown {
		t.Fatalf("shutdown publication outcome = %s, want MEMBER_PUBLISH_UNKNOWN", delivery.Members[0].State)
	}
}

func TestDurableChordRestartRecoversOutcomeWriteFailureWithStablePayload(t *testing.T) {
	base := newGroupConvergenceRedisBackend(t)
	wrapped := &issue104FailOnceBackend{
		Backend:             base,
		DurableChordBackend: base.(backend.DurableChordBackend),
		memberOutcomeFails:  1,
		forceLease:          10 * time.Millisecond,
	}
	firstPublished := make(chan *task.Signature, 2)
	firstServer, err := NewServerE(
		&groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
			firstPublished <- task.CopySignature(signature)
			return nil
		}},
		wrapped,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	member := task.NewSignature("outcome-failure-member", "member")
	group, _ := task.NewGroup("outcome-failure-group", "group", member)
	groupCallback, _ := task.NewGroupCallback(group, "group", task.NewSignature("outcome-failure-callback", "callback"))
	if _, err := firstServer.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); err == nil || !strings.Contains(err.Error(), "injected member outcome") {
		t.Fatalf("first durable send error = %v, want outcome persistence failure", err)
	}
	first := <-firstPublished
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	if err := firstServer.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	shutdownCancel()
	time.Sleep(20 * time.Millisecond)

	secondPublished := make(chan *task.Signature, 2)
	secondServer, err := NewServerE(
		&groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
			secondPublished <- task.CopySignature(signature)
			return nil
		}},
		wrapped,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = secondServer.Shutdown(ctx)
	})
	second := <-secondPublished
	firstBody, _ := json.Marshal(first)
	secondBody, _ := json.Marshal(second)
	if first.ID != second.ID || string(firstBody) != string(secondBody) {
		t.Fatalf("recovered publication changed identity/payload: first=%s second=%s", first.ID, second.ID)
	}
}

func TestDurableChordRestartRecoversEnqueueThenErrorWithStablePayload(t *testing.T) {
	base := newGroupConvergenceRedisBackend(t)
	wrapped := &issue104FailOnceBackend{
		Backend:             base,
		DurableChordBackend: base.(backend.DurableChordBackend),
		forceConfirmation:   10 * time.Millisecond,
	}
	firstPublished := make(chan *task.Signature, 2)
	firstServer, err := NewServerE(
		&groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
			firstPublished <- task.CopySignature(signature)
			return errors.New("broker accepted but response was lost")
		}},
		wrapped,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	group, _ := task.NewGroup("enqueue-error-group", "group", task.NewSignature("enqueue-error-member", "member"))
	groupCallback, _ := task.NewGroupCallback(group, "group", task.NewSignature("enqueue-error-callback", "callback"))
	if _, err := firstServer.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); err == nil || !strings.Contains(err.Error(), "broker accepted") {
		t.Fatalf("first durable send error = %v, want enqueue-then-error", err)
	}
	first := <-firstPublished
	deliveryKey := backend.ChordDeliveryKey(group.GroupID, groupCallback.Callback.ID)
	if delivery := issue104FindDelivery(t, wrapped, deliveryKey); delivery.Members[0].State != backend.ChordMemberPublishUnknown {
		t.Fatalf("enqueue-then-error state = %s, want MEMBER_PUBLISH_UNKNOWN", delivery.Members[0].State)
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	if err := firstServer.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	shutdownCancel()
	time.Sleep(20 * time.Millisecond)

	secondPublished := make(chan *task.Signature, 2)
	secondServer, err := NewServerE(
		&groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
			secondPublished <- task.CopySignature(signature)
			return nil
		}},
		wrapped,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = secondServer.Shutdown(ctx)
	})
	second := <-secondPublished
	firstBody, _ := json.Marshal(first)
	secondBody, _ := json.Marshal(second)
	if first.ID != second.ID || string(firstBody) != string(secondBody) {
		t.Fatalf("enqueue-then-error recovery changed identity/payload: first=%s second=%s", first.ID, second.ID)
	}
}

func TestDurableChordRetriesReceiptAndCallbackTerminalWriteFailures(t *testing.T) {
	base := newGroupConvergenceRedisBackend(t)
	wrapped := &issue104FailOnceBackend{
		Backend:               base,
		DurableChordBackend:   base.(backend.DurableChordBackend),
		memberTerminalFails:   1,
		callbackTerminalFails: 1,
		forceConfirmation:     10 * time.Millisecond,
	}
	published := make(chan *task.Signature, 8)
	server, err := NewServerE(
		&groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
			published <- task.CopySignature(signature)
			return nil
		}},
		wrapped,
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	memberExecutions := 0
	callbackExecutions := 0
	if err := server.RegisteredTask("member-fail-once", func() (string, error) {
		memberExecutions++
		return "member-result", nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisteredTask("callback-fail-once", func(string) (string, error) {
		callbackExecutions++
		return "callback-result", nil
	}); err != nil {
		t.Fatal(err)
	}
	group, _ := task.NewGroup("terminal-write-group", "group", task.NewSignature("terminal-write-member", "member-fail-once"))
	groupCallback, _ := task.NewGroupCallback(group, "group", task.NewSignature("terminal-write-callback", "callback-fail-once"))
	if _, err := server.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); err != nil {
		t.Fatal(err)
	}
	memberSignature := <-published
	worker := server.NewWorker("fail-once", 1, "queue")
	if err := worker.Process(memberSignature); err == nil || !strings.Contains(err.Error(), "injected member receipt") {
		t.Fatalf("first member execution error = %v, want receipt persistence failure", err)
	}
	deliveryKey := backend.ChordDeliveryKey(group.GroupID, groupCallback.Callback.ID)
	if delivery := issue104FindDelivery(t, wrapped, deliveryKey); delivery.CallbackState != backend.ChordWaiting || delivery.Members[0].Receipt != nil {
		t.Fatalf("failed receipt changed readiness = %#v", delivery)
	}
	var recoveredMember *task.Signature
	select {
	case recoveredMember = <-published:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not republish member after receipt write failure")
	}
	if recoveredMember.ID != memberSignature.ID {
		t.Fatalf("recovered member id = %s, want %s", recoveredMember.ID, memberSignature.ID)
	}
	if err := worker.Process(recoveredMember); err != nil {
		t.Fatal(err)
	}
	var callbackSignature *task.Signature
	select {
	case callbackSignature = <-published:
	case <-time.After(time.Second):
		t.Fatal("callback was not recovered after receipt retry")
	}
	if err := worker.Process(callbackSignature); err == nil || !strings.Contains(err.Error(), "injected callback terminal") {
		t.Fatalf("first callback execution error = %v, want terminal persistence failure", err)
	}
	if delivery := issue104FindDelivery(t, wrapped, deliveryKey); delivery.CallbackState == backend.ChordDelivered {
		t.Fatal("failed callback terminal write finalized delivery")
	}
	var recoveredCallback *task.Signature
	select {
	case recoveredCallback = <-published:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not republish callback after terminal write failure")
	}
	if recoveredCallback.ID != callbackSignature.ID {
		t.Fatalf("recovered callback id = %s, want %s", recoveredCallback.ID, callbackSignature.ID)
	}
	if err := worker.Process(recoveredCallback); err != nil {
		t.Fatal(err)
	}
	if delivery := issue104FindDelivery(t, wrapped, deliveryKey); delivery.CallbackState != backend.ChordDelivered {
		t.Fatalf("callback terminal retry state = %s", delivery.CallbackState)
	}
	if memberExecutions != 2 || callbackExecutions != 2 {
		t.Fatalf("duplicate execution evidence member=%d callback=%d, want 2/2", memberExecutions, callbackExecutions)
	}
}

func TestDurableChordOldWorkerStatusDoesNotSatisfyReceipt(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	durable := backendValue.(backend.DurableChordBackend)
	registration, _ := issue104Registration(t, "rollout-group", "rollout-callback", "member")
	ref, err := durable.RegisterChord(context.Background(), registration)
	if err != nil {
		t.Fatal(err)
	}
	published := make(chan *task.Signature, 8)
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		published <- task.CopySignature(signature)
		return nil
	}}
	server, err := NewServerE(controller, backendValue, timedGroupTestLocker{}, log.NewHelper(log.DefaultLogger), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Cleanup(func() { _ = server.Shutdown(ctx) })
	if err := server.RegisteredTask("member", func() (string, error) { return "upgraded", nil }); err != nil {
		t.Fatal(err)
	}
	member := <-published
	if member.CallbackChord != nil {
		t.Fatal("durable member can enter legacy TriggerCompleted path")
	}
	if err := backendValue.SetStateSuccess(member, []*task.Result{{Type: "string", Value: "old-worker"}}); err != nil {
		t.Fatal(err)
	}
	delivery := issue104FindDelivery(t, durable, ref.DeliveryKey)
	if delivery.CallbackState != backend.ChordWaiting || delivery.Members[0].Receipt != nil {
		t.Fatalf("generic old-worker status changed durable readiness = %#v", delivery)
	}
	worker := server.NewWorker("upgraded", 1, "queue")
	if err := worker.Process(member); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		delivery = issue104FindDelivery(t, durable, ref.DeliveryKey)
		if delivery.CallbackState != backend.ChordWaiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("upgraded worker did not record durable receipt")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDurableCallbackWorkerUsesDeliveryKeyAndRetriesRemainNonterminal(t *testing.T) {
	backendValue := newGroupConvergenceRedisBackend(t)
	durable := backendValue.(backend.DurableChordBackend)
	registrationA, callbackA := issue104Registration(t, "callback-worker-a", "shared-callback-worker", "member")
	registrationB, callbackB := issue104Registration(t, "callback-worker-b", "shared-callback-worker", "member")
	refA := issue104ReadyRegistration(t, durable, registrationA)
	refB := issue104ReadyRegistration(t, durable, registrationB)
	server := &Server{
		config:          &Config{ConsumeQueue: "queue"},
		backend:         backendValue,
		durableBackend:  durable,
		controller:      &groupTestController{},
		registeredTasks: &sync.Map{},
		helper:          log.NewHelper(log.DefaultLogger),
	}
	if err := server.RegisteredTask("success-callback", func() (string, error) { return "ok", nil }); err != nil {
		t.Fatal(err)
	}
	if err := server.RegisteredTask("retry-callback", func() (string, error) { return "", errors.New("retry") }); err != nil {
		t.Fatal(err)
	}
	callbackA.Name = "success-callback"
	callbackB.Name = "retry-callback"
	worker := server.NewWorker("callback-worker", 1, "queue")
	if err := worker.Process(callbackA); err != nil {
		t.Fatal(err)
	}
	callbackB.RetryCount = 1
	if err := worker.Process(callbackB); err != nil {
		t.Fatal(err)
	}
	if got := issue104FindDelivery(t, durable, refA.DeliveryKey); got.CallbackState != backend.ChordDelivered || got.TerminalOutcome != backend.CallbackTerminalSuccess {
		t.Fatalf("success callback terminal = %#v", got)
	}
	if got := issue104FindDelivery(t, durable, refB.DeliveryKey); got.CallbackState != backend.ChordReady {
		t.Fatalf("retryable callback finalized delivery = %#v", got)
	}
	callbackB.RetryCount = 0
	if err := worker.Process(callbackB); err != nil {
		t.Fatal(err)
	}
	if got := issue104FindDelivery(t, durable, refB.DeliveryKey); got.CallbackState != backend.ChordDelivered || got.TerminalOutcome != backend.CallbackTerminalFailure {
		t.Fatalf("final callback failure terminal = %#v", got)
	}
	if got := issue104FindDelivery(t, durable, refA.DeliveryKey); got.TerminalOutcome != backend.CallbackTerminalSuccess {
		t.Fatalf("colliding callback ID changed other delivery = %#v", got)
	}
}

func issue104Registration(t *testing.T, groupID, callbackID, memberName string) (backend.ChordRegistration, *task.Signature) {
	t.Helper()
	deliveryKey := backend.ChordDeliveryKey(groupID, callbackID)
	callback := task.NewSignature(callbackID, "callback")
	callback.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	callbackPayload, err := json.Marshal(callback)
	if err != nil {
		t.Fatal(err)
	}
	member := task.NewSignature(groupID+"-member", memberName)
	member.GroupID = groupID
	member.CallbackChord = nil
	member.Meta.Set(backend.DurableChordDeliveryKeyMeta, deliveryKey)
	member.Meta.Set(backend.DurableChordMemberMeta, true)
	member.Meta.Set(backend.DurableChordMemberOrdinal, 0)
	memberPayload, err := json.Marshal(member)
	if err != nil {
		t.Fatal(err)
	}
	registration := backend.ChordRegistration{
		GroupID:   groupID,
		GroupName: groupID,
		Retention: -1,
		Callback:  callbackPayload,
		Members:   []backend.ChordMemberRegistration{{Ordinal: 0, TaskID: member.ID, Payload: memberPayload}},
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		t.Fatal(err)
	}
	return registration, callback
}

func issue104ReadyRegistration(t *testing.T, durable backend.DurableChordBackend, registration backend.ChordRegistration) backend.ChordRegistrationRef {
	t.Helper()
	ref, err := durable.RegisterChord(context.Background(), registration)
	if err != nil {
		t.Fatal(err)
	}
	if err := durable.ReconcileChord(context.Background(), ref.DeliveryKey); err != nil {
		t.Fatal(err)
	}
	if err := durable.RecordMemberTerminal(context.Background(), ref.DeliveryKey, 0, registration.Members[0].TaskID, backend.MemberTerminalSuccess, nil); err != nil {
		t.Fatal(err)
	}
	return ref
}

func issue104FindDelivery(t *testing.T, durable backend.DurableChordBackend, key string) backend.ChordDelivery {
	t.Helper()
	cursor := ""
	for {
		page, err := durable.ScanChordDeliveries(context.Background(), backend.ChordScan{Cursor: cursor, Limit: 10, Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		for _, delivery := range page.Deliveries {
			if delivery.DeliveryKey == key {
				return delivery
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	t.Fatalf("delivery %s not found", key)
	return backend.ChordDelivery{}
}

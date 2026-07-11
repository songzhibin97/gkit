package chordtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

func Run(t *testing.T, durable backend.DurableChordBackend) {
	t.Helper()
	ctx := context.Background()
	prefix := fmt.Sprintf("issue104-%d", time.Now().UnixNano())
	runConcurrentRegistrationContract(t, durable, prefix)
	registration := newRegistration(t, prefix+"-group", "shared-callback", 2)

	created, err := durable.RegisterChord(ctx, registration)
	if err != nil || !created.Created {
		t.Fatalf("RegisterChord created = %#v, %v", created, err)
	}
	attached, err := durable.RegisterChord(ctx, registration)
	if err != nil || attached.Created || attached.DeliveryKey != created.DeliveryKey {
		t.Fatalf("RegisterChord attached = %#v, %v", attached, err)
	}
	if err := durable.AbortRegistration(ctx, attached); !errors.Is(err, backend.ErrChordRegistrationOwnershipLost) {
		t.Fatalf("noncreator AbortRegistration error = %v", err)
	}
	stale := created
	stale.Owner = "not-owner"
	if err := durable.AbortRegistration(ctx, stale); !errors.Is(err, backend.ErrChordRegistrationOwnershipLost) {
		t.Fatalf("stale AbortRegistration error = %v", err)
	}
	conflict := registration
	conflict.Members = append([]backend.ChordMemberRegistration(nil), registration.Members...)
	conflict.Members[0].Payload = append([]byte(nil), conflict.Members[0].Payload...)
	conflict.Members[0].Payload = append(conflict.Members[0].Payload, ' ')
	conflict.DefinitionHash = ""
	if err := backend.FinalizeChordRegistration(&conflict); err != nil {
		t.Fatal(err)
	}
	if _, err := durable.RegisterChord(ctx, conflict); !errors.Is(err, backend.ErrChordRegistrationConflict) {
		t.Fatalf("conflicting RegisterChord error = %v", err)
	}
	if err := durable.AbortRegistration(ctx, created); err != nil {
		t.Fatalf("creator AbortRegistration: %v", err)
	}
	if err := durable.AbortRegistration(ctx, created); err != nil {
		t.Fatalf("repeat AbortRegistration: %v", err)
	}

	created, err = durable.RegisterChord(ctx, registration)
	if err != nil || !created.Created {
		t.Fatalf("RegisterChord after abort = %#v, %v", created, err)
	}
	if err := durable.ReconcileChord(ctx, created.DeliveryKey); err != nil {
		t.Fatalf("ReconcileChord: %v", err)
	}

	now := time.Now()
	leaseA, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: created.DeliveryKey, Ordinal: 0, Owner: "member-a", Now: now, LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("first member claim = %#v, %t, %v", leaseA, claimed, err)
	}
	if _, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: created.DeliveryKey, Ordinal: 0, Owner: "member-b", Now: now, LeaseDuration: time.Second}); err != nil || claimed {
		t.Fatalf("concurrent member claim = %t, %v", claimed, err)
	}
	leaseB, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: created.DeliveryKey, Ordinal: 0, Owner: "member-b", Now: now.Add(2 * time.Second), LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("reclaimed member = %#v, %t, %v", leaseB, claimed, err)
	}
	if err := durable.RecordMemberPublishOutcome(ctx, leaseA, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now}); !errors.Is(err, backend.ErrChordLeaseLost) {
		t.Fatalf("stale member outcome error = %v", err)
	}
	unknownDeadline := now.Add(4 * time.Second)
	if err := durable.RecordMemberPublishOutcome(ctx, leaseB, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeUnknown, Now: now.Add(2 * time.Second), ConfirmationDeadline: unknownDeadline}); err != nil {
		t.Fatalf("record unknown member outcome: %v", err)
	}
	if _, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: created.DeliveryKey, Ordinal: 0, Owner: "member-c", Now: unknownDeadline.Add(-time.Millisecond)}); err != nil || claimed {
		t.Fatalf("early unknown reclaim = %t, %v", claimed, err)
	}
	leaseC, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: created.DeliveryKey, Ordinal: 0, Owner: "member-c", Now: unknownDeadline, LeaseDuration: time.Second})
	if err != nil || !claimed || string(leaseC.Payload) != string(leaseA.Payload) || leaseC.TaskID != leaseA.TaskID {
		t.Fatalf("unknown reclaim = %#v, %t, %v", leaseC, claimed, err)
	}
	if err := durable.RecordMemberPublishOutcome(ctx, leaseC, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: unknownDeadline, ConfirmationDeadline: unknownDeadline.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := durable.RecordMemberTerminal(ctx, created.DeliveryKey, 0, leaseC.TaskID, backend.MemberTerminalSuccess, []*task.Result{{Type: "string", Value: "first"}}); err != nil {
		t.Fatalf("record member 0 receipt: %v", err)
	}

	lease1, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: created.DeliveryKey, Ordinal: 1, Owner: "member-d", Now: now, LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("member 1 claim = %#v, %t, %v", lease1, claimed, err)
	}
	if err := durable.RecordMemberPublishOutcome(ctx, lease1, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now, ConfirmationDeadline: now.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := durable.RecordMemberTerminal(ctx, created.DeliveryKey, 1, lease1.TaskID, backend.MemberTerminalSuccess, []*task.Result{{Type: "string", Value: "second"}}); err != nil {
		t.Fatalf("record member 1 receipt: %v", err)
	}

	delivery := findDelivery(t, durable, created.DeliveryKey)
	if delivery.CallbackState != backend.ChordReady {
		t.Fatalf("callback state = %s, want READY", delivery.CallbackState)
	}
	var callback task.Signature
	if err := json.Unmarshal(delivery.CallbackPayload, &callback); err != nil {
		t.Fatal(err)
	}
	if len(callback.Args) != 2 || callback.Args[0].Value != "first" || callback.Args[1].Value != "second" {
		t.Fatalf("callback args = %#v", callback.Args)
	}

	callbackLeaseA, claimed, err := durable.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: created.DeliveryKey, Owner: "callback-a", Now: now, LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("callback claim = %#v, %t, %v", callbackLeaseA, claimed, err)
	}
	callbackLeaseB, claimed, err := durable.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: created.DeliveryKey, Owner: "callback-b", Now: now.Add(2 * time.Second), LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("callback reclaim = %#v, %t, %v", callbackLeaseB, claimed, err)
	}
	if err := durable.RecordCallbackPublishOutcome(ctx, callbackLeaseA, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now}); !errors.Is(err, backend.ErrChordLeaseLost) {
		t.Fatalf("stale callback outcome error = %v", err)
	}
	if err := durable.RecordCallbackPublishOutcome(ctx, callbackLeaseB, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now.Add(2 * time.Second), ConfirmationDeadline: now.Add(3 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := durable.RecordCallbackTerminal(ctx, created.DeliveryKey, backend.CallbackTerminalSuccess); err != nil {
		t.Fatal(err)
	}
	delivery = findDelivery(t, durable, created.DeliveryKey)
	if delivery.CallbackState != backend.ChordDelivered || delivery.TerminalExpireAt == nil || !delivery.TerminalExpireAt.After(delivery.TerminalAt) {
		t.Fatalf("terminal delivery = %#v", delivery)
	}

	other := newRegistration(t, prefix+"-other-group", "shared-callback", 1)
	otherRef, err := durable.RegisterChord(ctx, other)
	if err != nil || otherRef.DeliveryKey == created.DeliveryKey {
		t.Fatalf("shared callback registration = %#v, %v", otherRef, err)
	}
	if err := durable.ReconcileChord(ctx, otherRef.DeliveryKey); err != nil {
		t.Fatal(err)
	}
	if err := durable.RecordMemberTerminal(ctx, otherRef.DeliveryKey, 0, other.Members[0].TaskID, backend.MemberTerminalSuccess, []*task.Result{{Type: "string", Value: "other"}}); err != nil {
		t.Fatal(err)
	}
	if err := durable.RecordCallbackTerminal(ctx, otherRef.DeliveryKey, backend.CallbackTerminalFailure); err != nil {
		t.Fatal(err)
	}
	if got := findDelivery(t, durable, created.DeliveryKey); got.CallbackState != backend.ChordDelivered || got.TerminalOutcome != backend.CallbackTerminalSuccess {
		t.Fatalf("shared callback ID changed first delivery = %#v", got)
	}
	if got := findDelivery(t, durable, otherRef.DeliveryKey); got.CallbackState != backend.ChordDelivered || got.TerminalOutcome != backend.CallbackTerminalFailure {
		t.Fatalf("shared callback ID second delivery = %#v", got)
	}

	runSuppressionAndRetentionContract(t, durable, prefix)
	runConcurrentClaimContract(t, durable, prefix)
}

func runConcurrentRegistrationContract(t *testing.T, durable backend.DurableChordBackend, prefix string) {
	t.Helper()
	registration := newRegistration(t, prefix+"-registration-race", "registration-callback", 2)
	start := make(chan struct{})
	refs := make(chan backend.ChordRegistrationRef, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for attempt := 0; attempt < 10; attempt++ {
				ref, err := durable.RegisterChord(context.Background(), registration)
				if err == nil {
					refs <- ref
					return
				}
				if attempt == 9 {
					errs <- err
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(refs)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent identical registration: %v", err)
	}
	created := 0
	attached := 0
	deliveryKey := ""
	for ref := range refs {
		if deliveryKey == "" {
			deliveryKey = ref.DeliveryKey
		} else if ref.DeliveryKey != deliveryKey {
			t.Fatalf("concurrent registrations produced keys %q and %q", deliveryKey, ref.DeliveryKey)
		}
		if ref.Created {
			created++
		} else {
			attached++
		}
	}
	if created != 1 || attached != 1 {
		t.Fatalf("concurrent identical registration created=%d attached=%d, want 1/1", created, attached)
	}
	delivery := findDelivery(t, durable, deliveryKey)
	for ordinal, member := range delivery.Members {
		if member.TaskID != registration.Members[ordinal].TaskID || string(member.Payload) != string(registration.Members[ordinal].Payload) {
			t.Fatalf("stored member %d identity/payload changed", ordinal)
		}
	}
}

func runSuppressionAndRetentionContract(t *testing.T, durable backend.DurableChordBackend, prefix string) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name          string
		retention     int64
		failureFirst  bool
		wantRetention time.Duration
		wantNoExpiry  bool
	}{
		{name: "zero", retention: 0, wantRetention: time.Hour},
		{name: "negative", retention: -1, wantNoExpiry: true, failureFirst: true},
		{name: "positive", retention: 3, wantRetention: 3 * time.Second},
	} {
		t.Run("retention_"+tc.name, func(t *testing.T) {
			registration := newRegistration(t, prefix+"-retention-"+tc.name, "retention-callback", 2)
			registration.Retention = tc.retention
			registration.DefinitionHash = ""
			if err := backend.FinalizeChordRegistration(&registration); err != nil {
				t.Fatal(err)
			}
			ref, err := durable.RegisterChord(ctx, registration)
			if err != nil {
				t.Fatal(err)
			}
			if err := durable.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
				t.Fatal(err)
			}
			if _, err := durable.CleanupTerminalChordDeliveries(ctx, time.Now().Add(24*time.Hour), 10000); err != nil {
				t.Fatal(err)
			}
			if !deliveryExists(t, durable, ref.DeliveryKey) {
				t.Fatal("active durable delivery expired before terminal transition")
			}
			failureOrdinal := 1
			if tc.failureFirst {
				failureOrdinal = 0
			}
			for ordinal := range registration.Members {
				outcome := backend.MemberTerminalSuccess
				if ordinal == failureOrdinal {
					outcome = backend.MemberTerminalFailure
				}
				if err := durable.RecordMemberTerminal(ctx, ref.DeliveryKey, ordinal, registration.Members[ordinal].TaskID, outcome, nil); err != nil {
					t.Fatal(err)
				}
			}
			// Exact repeats are idempotent and must not create a second terminal transition.
			if err := durable.RecordMemberTerminal(ctx, ref.DeliveryKey, failureOrdinal, registration.Members[failureOrdinal].TaskID, backend.MemberTerminalFailure, nil); err != nil {
				t.Fatal(err)
			}
			delivery := findDelivery(t, durable, ref.DeliveryKey)
			if delivery.CallbackState != backend.ChordSuppressed || delivery.TerminalOutcome != backend.CallbackTerminalFailure {
				t.Fatalf("failure receipts produced delivery = %#v", delivery)
			}
			if _, claimed, err := durable.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: "suppressed", Now: time.Now()}); err != nil || claimed {
				t.Fatalf("suppressed callback claim = %t, %v", claimed, err)
			}
			if tc.wantNoExpiry {
				if delivery.TerminalExpireAt != nil {
					t.Fatalf("negative retention expiry = %s, want nil", delivery.TerminalExpireAt)
				}
				if _, err := durable.CleanupTerminalChordDeliveries(ctx, time.Now().Add(365*24*time.Hour), 10000); err != nil {
					t.Fatal(err)
				}
				if !deliveryExists(t, durable, ref.DeliveryKey) {
					t.Fatal("negative-retention terminal delivery was removed")
				}
				return
			}
			if delivery.TerminalExpireAt == nil {
				t.Fatal("terminal delivery has no expiry")
			}
			if delta := delivery.TerminalExpireAt.Sub(delivery.TerminalAt); delta != tc.wantRetention {
				t.Fatalf("terminal retention = %s, want %s", delta, tc.wantRetention)
			}
			// SQL backends may persist timestamps at millisecond precision, so use
			// a one-second margin on either side of the semantic boundary.
			if _, err := durable.CleanupTerminalChordDeliveries(ctx, delivery.TerminalExpireAt.Add(-time.Second), 10000); err != nil {
				t.Fatal(err)
			}
			if !deliveryExists(t, durable, ref.DeliveryKey) {
				t.Fatal("terminal delivery removed before retention boundary")
			}
			if _, err := durable.CleanupTerminalChordDeliveries(ctx, delivery.TerminalExpireAt.Add(time.Second), 10000); err != nil {
				t.Fatal(err)
			}
			if deliveryExists(t, durable, ref.DeliveryKey) {
				t.Fatal("terminal delivery survived retention boundary")
			}
		})
	}
}

func runConcurrentClaimContract(t *testing.T, durable backend.DurableChordBackend, prefix string) {
	t.Helper()
	ctx := context.Background()
	registration := newRegistration(t, prefix+"-concurrent-claims", "concurrent-callback", 1)
	ref, err := durable.RegisterChord(ctx, registration)
	if err != nil {
		t.Fatal(err)
	}
	if err := durable.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	memberLeases := make(chan backend.ChordMemberLease, 8)
	memberErrs := make(chan error, 8)
	var memberWG sync.WaitGroup
	for index := 0; index < 8; index++ {
		memberWG.Add(1)
		go func(index int) {
			defer memberWG.Done()
			for attempt := 0; attempt < 10; attempt++ {
				lease, claimed, claimErr := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: ref.DeliveryKey, Ordinal: 0, Owner: fmt.Sprintf("member-%d", index), Now: now, LeaseDuration: time.Second})
				if claimErr != nil {
					if attempt == 9 {
						memberErrs <- claimErr
						return
					}
					time.Sleep(2 * time.Millisecond)
					continue
				}
				if claimed {
					memberLeases <- lease
				}
				return
			}
		}(index)
	}
	memberWG.Wait()
	close(memberLeases)
	close(memberErrs)
	for claimErr := range memberErrs {
		t.Fatalf("concurrent member claim: %v", claimErr)
	}
	var firstMember backend.ChordMemberLease
	memberWinners := 0
	for lease := range memberLeases {
		firstMember = lease
		memberWinners++
	}
	if memberWinners != 1 {
		t.Fatalf("concurrent member claim winners = %d, want 1", memberWinners)
	}
	secondMember, claimed, err := durable.ClaimMemberPublication(ctx, backend.ChordMemberClaim{DeliveryKey: ref.DeliveryKey, Ordinal: 0, Owner: "member-recovery", Now: now.Add(2 * time.Second), LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("expired member recovery = %#v, %t, %v", secondMember, claimed, err)
	}
	if err := durable.RecordMemberPublishOutcome(ctx, firstMember, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now}); !errors.Is(err, backend.ErrChordLeaseLost) {
		t.Fatalf("stale concurrent member owner error = %v", err)
	}
	if err := durable.RecordMemberTerminal(ctx, ref.DeliveryKey, 0, registration.Members[0].TaskID, backend.MemberTerminalSuccess, nil); err != nil {
		t.Fatal(err)
	}

	callbackLeases := make(chan backend.ChordCallbackLease, 8)
	callbackErrs := make(chan error, 8)
	var callbackWG sync.WaitGroup
	for index := 0; index < 8; index++ {
		callbackWG.Add(1)
		go func(index int) {
			defer callbackWG.Done()
			for attempt := 0; attempt < 10; attempt++ {
				lease, callbackClaimed, claimErr := durable.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: fmt.Sprintf("callback-%d", index), Now: now, LeaseDuration: time.Second})
				if claimErr != nil {
					if attempt == 9 {
						callbackErrs <- claimErr
						return
					}
					time.Sleep(2 * time.Millisecond)
					continue
				}
				if callbackClaimed {
					callbackLeases <- lease
				}
				return
			}
		}(index)
	}
	callbackWG.Wait()
	close(callbackLeases)
	close(callbackErrs)
	for claimErr := range callbackErrs {
		t.Fatalf("concurrent callback claim: %v", claimErr)
	}
	var firstCallback backend.ChordCallbackLease
	callbackWinners := 0
	for lease := range callbackLeases {
		firstCallback = lease
		callbackWinners++
	}
	if callbackWinners != 1 {
		t.Fatalf("concurrent callback claim winners = %d, want 1", callbackWinners)
	}
	secondCallback, claimed, err := durable.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: "callback-recovery", Now: now.Add(2 * time.Second), LeaseDuration: time.Second})
	if err != nil || !claimed {
		t.Fatalf("expired callback recovery = %#v, %t, %v", secondCallback, claimed, err)
	}
	if err := durable.RecordCallbackPublishOutcome(ctx, firstCallback, backend.ChordPublishOutcome{Kind: backend.ChordPublishOutcomeSucceeded, Now: now}); !errors.Is(err, backend.ErrChordLeaseLost) {
		t.Fatalf("stale concurrent callback owner error = %v", err)
	}
}

func newRegistration(t *testing.T, groupID, callbackID string, members int) backend.ChordRegistration {
	t.Helper()
	callback := task.NewSignature(callbackID, "callback")
	callback.Meta.Set(backend.DurableChordDeliveryKeyMeta, backend.ChordDeliveryKey(groupID, callbackID))
	callbackPayload, err := json.Marshal(callback)
	if err != nil {
		t.Fatal(err)
	}
	registration := backend.ChordRegistration{GroupID: groupID, GroupName: "group", Retention: 2, Callback: callbackPayload}
	for ordinal := 0; ordinal < members; ordinal++ {
		member := task.NewSignature(fmt.Sprintf("%s-member-%d", groupID, ordinal), "member")
		member.GroupID = groupID
		member.CallbackChord = nil
		member.Meta.Set(backend.DurableChordDeliveryKeyMeta, backend.ChordDeliveryKey(groupID, callbackID))
		member.Meta.Set(backend.DurableChordMemberMeta, true)
		member.Meta.Set(backend.DurableChordMemberOrdinal, ordinal)
		payload, err := json.Marshal(member)
		if err != nil {
			t.Fatal(err)
		}
		registration.Members = append(registration.Members, backend.ChordMemberRegistration{Ordinal: ordinal, TaskID: member.ID, Payload: payload})
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		t.Fatal(err)
	}
	return registration
}

func findDelivery(t *testing.T, durable backend.DurableChordBackend, deliveryKey string) backend.ChordDelivery {
	t.Helper()
	cursor := ""
	for {
		page, err := durable.ScanChordDeliveries(context.Background(), backend.ChordScan{Cursor: cursor, Limit: 10, Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		for _, delivery := range page.Deliveries {
			if delivery.DeliveryKey == deliveryKey {
				return delivery
			}
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	t.Fatalf("delivery %s not found", deliveryKey)
	return backend.ChordDelivery{}
}

func deliveryExists(t *testing.T, durable backend.DurableChordBackend, deliveryKey string) bool {
	t.Helper()
	cursor := ""
	for {
		page, err := durable.ScanChordDeliveries(context.Background(), backend.ChordScan{Cursor: cursor, Limit: 10, Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		for _, delivery := range page.Deliveries {
			if delivery.DeliveryKey == deliveryKey {
				return true
			}
		}
		if page.NextCursor == "" {
			return false
		}
		cursor = page.NextCursor
	}
}

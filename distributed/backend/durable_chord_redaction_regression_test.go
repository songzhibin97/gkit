package backend

import (
	"strings"
	"testing"
	"time"
)

func TestDurableChordStoredErrorsAreStructuredAndRedacted(t *testing.T) {
	secret := "synthetic-secret-token payload={credit_card:4111111111111111}"
	now := time.Now()
	delivery := ChordDelivery{
		DeliveryKey: "delivery",
		Members: []ChordMemberPublication{{
			Ordinal: 0,
			TaskID:  "member",
			Payload: []byte(`{"args":["private-payload"],"meta":{"token":"private-meta"}}`),
			State:   ChordMemberReady,
			Version: 1,
		}},
		CallbackState:   ChordReady,
		CallbackPayload: []byte(`{"args":["private-callback-payload"]}`),
		CallbackVersion: 1,
	}
	memberLease, claimed, err := ClaimChordMember(&delivery, ChordMemberClaim{DeliveryKey: delivery.DeliveryKey, Ordinal: 0, Owner: "member-owner", Now: now})
	if err != nil || !claimed {
		t.Fatalf("member claim = %t, %v", claimed, err)
	}
	if err := ApplyChordMemberPublishOutcome(&delivery, memberLease, ChordPublishOutcome{Kind: ChordPublishOutcomeUnknown, Now: now, Error: secret}); err != nil {
		t.Fatal(err)
	}
	if got := delivery.Members[0].LastError; got != "operation=member_publish category=unknown" {
		t.Fatalf("member stored error = %q", got)
	}

	callbackLease, claimed, err := ClaimChordCallback(&delivery, ChordCallbackClaim{DeliveryKey: delivery.DeliveryKey, Owner: "callback-owner", Now: now})
	if err != nil || !claimed {
		t.Fatalf("callback claim = %t, %v", claimed, err)
	}
	if err := ApplyChordCallbackPublishOutcome(&delivery, callbackLease, ChordPublishOutcome{Kind: ChordPublishOutcomeRejected, Now: now, Error: secret}); err != nil {
		t.Fatal(err)
	}
	if got := delivery.CallbackLastError; got != "operation=callback_publish category=rejected" {
		t.Fatalf("callback stored error = %q", got)
	}

	body := string(delivery.Members[0].Payload) + string(delivery.CallbackPayload)
	for _, forbidden := range []string{secret, "synthetic-secret-token", "credit_card", body} {
		if strings.Contains(delivery.Members[0].LastError, forbidden) || strings.Contains(delivery.CallbackLastError, forbidden) {
			t.Fatalf("stored error leaked %q", forbidden)
		}
	}
}

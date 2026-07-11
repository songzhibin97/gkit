# Durable chord callback delivery

## Summary

Group callbacks remain recoverable across publication ambiguity, backend failures, concurrent workers, rolling upgrades, and process restarts. Eligible callbacks use stable group-scoped identity and immutable payloads and provide at-least-once delivery without claiming exactly-once side effects.

## Behavior

1. Sending a group callback rejects a nil group-callback value, nil group, nil callback, empty group, nil member, empty group/callback/member identifier, or duplicate member identifier before registering or publishing anything. The returned error is identifiable as invalid chord input and names the invalid field or member.
2. Durable registration, including every member's stable identifier and immutable publication payload, completes before any member is published. Concurrent identical sends share one registration, and setup cleanup can affect only the registration version created and still owned by that send.
3. Once publication ownership has been acquired, an ordinary publication error or process exit never aborts the registration. Recovery retries the same identifier and payload; only an explicit rejection that guarantees no broker side effect can become a final member failure.
4. Group-scoped member completion receipts are the sole authority for callback readiness. A temporary receipt-write failure leaves the same member recoverable, and generic task status written by an older worker cannot make the callback ready.
5. Durable state has a stable group-scoped identity that is isolated from user task and group state. A storage backend rejects identifiers in any namespace it reserves before changing user or durable state, and pre-existing user state occupying a durable location is never overwritten during send or restart.
6. Concurrent workers and service instances permit at most one current publisher for each member or callback. Ownership expires and can be recovered, while a stale owner cannot finalize a newer owner's publication.
7. Every ordinary publication error is an unknown broker outcome. Recovery may publish the same identifier and payload again; only an explicitly typed no-side-effect rejection is definite.
8. Server startup scans and reconciles all nonterminal durable work before accepting new durable registration. Startup failure is observable, and a later healthy restart recovers registered work from backend state alone even when new registration is disabled.
9. Scheduled durable callbacks require cancellation-aware locking. Lock acquisition, retry waiting, sending, unlock cleanup, dispatcher work, and shutdown joining are bounded by server-owned cancellation; non-durable scheduling retains legacy-locker compatibility.
10. The group-scoped delivery identity is the authority for callback terminal state. Generic status or another group reusing the same callback task identifier cannot finalize, suppress, or move the wrong delivery backward.
11. Retention begins at terminal delivery time: zero means 3600 seconds, negative means no expiry, and positive values are retained unchanged. Active delivery state and evidence required for readiness do not expire under this policy.
12. Any final member failure durably suppresses the success callback; the callback is not published and does not remain indefinitely recoverable as ready work.
13. While durable backend state remains available and the broker eventually accepts work, every registered member and eligible all-success callback receives at least one publication and execution attempt with stable identity. Duplicate attempts remain possible and handlers must make external side effects idempotent.
14. Durable registration and strict backend enforcement are independent rollout controls. Older workers provide only the safety property that they cannot trigger the legacy callback path; upgraded workers are required for durable liveness, and disabling new registration does not stop recovery of existing records.
15. Existing backend implementations remain source-compatible. With registration disabled the legacy path is unchanged; with registration enabled, compatibility mode falls back for a backend without durable capability and strict mode rejects before publication.

## Non-goals

- Exactly-once publication, execution, or application side effects.
- Automatic idempotency for callback handlers.
- Reconstructing results that were never durably recorded.
- Retrofitting legacy groups that contain no recoverable callback payload.

# Durable chord callback delivery: technical design

## Context

[PRODUCT.md](./PRODUCT.md) defines the shipped at-least-once contract. Public validation and durable/legacy routing live in `distributed/service.go`; durable registration, dispatcher recovery, publication classification, and shutdown live in `distributed/durable_chord.go`; upgraded worker receipts and terminal recording live in `distributed/worker.go`. The optional capability and backend-independent state machine live in `distributed/backend/durable_chord.go`.

Redis stores legacy task status and group metadata under raw caller identifiers in `distributed/backend/backend_redis/redis.go`, while durable records and indexes use private keys in `distributed/backend/backend_redis/durable_chord.go`. SQL initializes the current task/group schema before durable tables in `distributed/backend/backend_db/db.go` and `distributed/backend/backend_db/durable_chord.go`. MongoDB uses a separate durable collection in `distributed/backend/backend_mongodb/durable_chord.go`.

This document describes the implementation that ships on the current branch, including the merged `master` schema contract; it is not a future design.

## Proposed changes

### Validation and immutable registration

`SendGroupCallbackWithContext` validates every pointer, identifier, group member, and duplicate before backend/controller calls. All validation errors wrap `backend.ErrChordInvalidInput` and include field or ordinal context. Durable send copies caller templates, applies the pre-publish hook once, clears the legacy callback pointer on member copies, freezes routing/metadata, serializes each payload once, and registers before claiming publication.

Registration ownership is an owner/version fence. Exactly one identical concurrent registration is the creator; attached callers cannot abort or reset it. Abort is idempotent only for the current created reference before any member lease. A member lease permanently closes the abort window.

### Redis namespace choice, compatibility, and rollback

Redis keeps the existing raw task/group key format; user keys are not encoded or migrated. Durable records and indexes retain the dedicated `gkit:chord:` key domain. Because raw identifiers could otherwise equal a private key, every Redis task/group raw-key write entry point rejects identifiers beginning with that reserved prefix before `SET`, `SETNX`, or `DEL`: group takeover/completion, all task-state updates, resets, durable registration setup, callback pending setup, and restart reconciliation all use the same check.

This choice preserves reads and writes for every historical raw identifier outside the newly reserved domain. Registration still claims its record with `SETNX`. If that key already contains a legacy task/group value, malformed data, or a durable record whose delivery identity does not match the key, registration fails closed with conflict, removes only its own pending index generation, and never overwrites the value. Restart scanning therefore cannot convert an occupied legacy key into durable state.

No data rewrite is required for deployment. Rollback first disables new durable registration and drains or preserves existing durable records; because ordinary user keys were never re-encoded, older readers continue to read all non-reserved historical keys. Operators must not create new `gkit:chord:` user identifiers during rollback because older binaries do not enforce the reservation. The durable record format and indexes are otherwise unchanged.

### Recovery, fencing, and terminal authority

The backend stores a group-scoped delivery key, definition hash, owner/version, immutable member payloads, publication leases/outcomes, receipts, frozen callback payload, terminal outcome, and terminal timestamps. Member and callback claims are compare-and-swap transitions. Nil publication records success; untyped errors, cancellation after dispatch, and outcome-write failure remain unknown; only `DefinitePublishRejection` may be final.

Member receipts validate delivery key, ordinal, task ID, outcome, and results. Generic status never substitutes for a receipt. The final successful receipt freezes callback arguments in member order; any failure receipt atomically suppresses the callback. Callback terminal writes use only the reserved delivery-key metadata and never infer authority from callback task ID.

Startup completes a paginated scan before durable admission. Periodic recovery handles setup, ready, leased, published, unknown, terminal, and suppressed records. Registration and recovery have independent enablement. Shutdown rejects admission, cancels scheduled registration and cleanup roots, stops scheduling, cancels dispatcher claims/publications, and joins all owned goroutines.

### SQL and MongoDB storage

The merged current schema keeps `GroupMeta.GroupID` at size 191 with unique index `uq_group_meta_group_id` and `Status.TaskID` at size 191 with unique index `uq_status_task_id`; the existing `StringSlice` scanner/versioned encoding remains intact. `BackendSQLDB.autoMigrate` migrates those task/group constraints before the durable chord tables. Fresh MySQL 8 and PostgreSQL 16 migrations must expose both exact lengths and unique indexes, and historical duplicate groups must fail migration without silent deletion.

SQL uses local transactions and conditional owner/version updates across delivery, publication, and receipt tables. MongoDB uses one durable document with unique/due/TTL indexes and compare-and-swap update filters. Broker publication remains outside either storage transaction.

## Testing and validation

Each PRODUCT behavior has exactly one primary regression test:

| Behavior | Primary proof | Mutation target |
| --- | --- | --- |
| 1 | `TestDurableChordValidationHasTypedFieldErrorsAndZeroSideEffects` | Replace a typed field error with a generic error or remove a guard. |
| 2 | `TestDurableChordAttachedRegistrationObservesCreatorAbort` | Allow an attached/stale owner to abort or reset another registration. |
| 3 | `TestDurableChordRestartRecoversOutcomeWriteFailureWithStablePayload` | Abort after ownership or regenerate member identity/payload on restart. |
| 4 | `TestDurableChordRetriesReceiptAndCallbackTerminalWriteFailures` | Treat generic task status as a receipt or stop after a receipt-write failure. |
| 5 | `TestRedisDurableChordNamespaceIsolatedAcrossSendWorkerAndRestart` | Remove the reserved-domain check or overwrite a legacy record collision. |
| 6 | `backend_redis.TestDurableChordContract` | Remove owner/version predicates from member or callback claims/outcomes. |
| 7 | `TestDurableChordPublishOutcomeClassification` | Classify an ordinary publication error as a definite rejection. |
| 8 | `TestDurableChordStartupRecoversEveryScanPageWhenRegistrationDisabled` | Skip a state/page or gate recovery on registration enablement. |
| 9 | `TestShutdownCancelsUnlockAlreadyInProgress` | Root unlock outside server cancellation or return before owned work joins. |
| 10 | `TestDurableCallbackWorkerUsesDeliveryKeyAndRetriesRemainNonterminal` | Finalize through callback task ID or generic status. |
| 11 | `backend_db.TestDurableChordContract/retention_zero` | Start retention at creation or interpret zero/negative incorrectly. |
| 12 | `backend_mongodb.TestDurableChordContract/retention_negative` | Ignore a failed receipt and make the callback ready. |
| 13 | `TestDurableChordEndToEndUsesReceiptsAndDeliveryKey` | Remove a recovery transition or regenerate an identity. |
| 14 | `TestDurableChordOldWorkerStatusDoesNotSatisfyReceipt` | Let an old worker trigger the legacy path or satisfy durable readiness. |
| 15 | `TestDurableChordCompatibilityMatrix` | Add methods to the base backend interface or couple strictness to registration. |

Supporting blocker gates are `TestRedisRawKeyWritersRejectDurableChordNamespace`, `TestRedisChordRegistrationFailsClosedOnLegacyRecordCollision`, `TestDurableChordCurrentMasterSchemaContract`, and `TestDurableChordCurrentMasterSchemaLiveSQL`. The namespace tests cover public send, worker status writes, legacy collision, and restart; generic-error, reserved-prefix, record-overwrite, size-191, and unique-index mutations must each fail.

Required execution gates:

- touched and full `distributed` packages under `go test -race`;
- `go vet ./distributed/...` and `git diff --check`;
- live Redis 7, MongoDB 7, MySQL 8, and PostgreSQL 16 backend contracts;
- idempotency, concurrent claim, owner/version fence, restart, and terminal-authority tests;
- combined-tree checks with PR #120, PR #122, and the current PR #126 head.

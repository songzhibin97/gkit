# Callback publication compensation technical specification

## Context

[PRODUCT.md](./PRODUCT.md) defines the observable contract. `distributed/service.go` currently writes `PENDING` before `Controller.Publish`, then uses unconditional `Backend.SetStateFailure` after a publish error. That can overwrite worker progress or a newer send of the same task ID. `distributed/backend/backend.go` has no conditional state primitive. The built-in implementations persist task state in SQL (`backend_db/db.go`), MongoDB (`backend_mongodb/mongo.go`), and Redis (`backend_redis/redis.go`). `distributed/worker.go` currently discards ordinary callback send errors.

The PR description refers to this specification directory, but it was absent before this remediation.

## Proposed changes

- Add an optional `backend.PublicationAttemptBackend` interface without changing the existing `backend.Backend` method set. It atomically records a private publication-attempt identifier with `PENDING` and conditionally changes only the matching `PENDING` attempt to `FAILURE`.
- Generate a cryptographically random attempt identifier in `Server.SendTaskWithContext` before any backend write. A test-only server hook makes the generation-failure path deterministic without mutable process-global state.
- Use the optional interface for built-in backends. Third-party backends that implement only `backend.Backend` keep `SetStatePending`; publish errors return without unsafe compensation.
- Keep attempt metadata backend-private:
  - SQL stores it in a private column on the status table and performs a conditional `UPDATE` by task ID, state, and attempt identifier.
  - MongoDB stores a private document field and uses a filtered `UpdateOne`. Its update document keeps `$setOnInsert.create_at` compatible with PR #120 while non-failure updates retain `error: ""`.
  - Redis stores a private JSON field and uses one Lua script to compare and update state while restoring the key's existing TTL.
- Legacy pending writes clear private attempt ownership so stale metadata cannot be reused. Reads through `task.Status` continue to ignore backend-private metadata.
- Route ordinary callback send errors through the worker's existing error handler or structured logger, including an `errors.Join` value returned by send compensation. Log context includes task and callback IDs only, never callback arguments.

## Testing and validation

1. Behavior §1 → `TestSendTaskAttemptIDGenerationFailureHasNoSideEffects` injects identifier generation failure and asserts no pending write or publish.
2. Behavior §2 → `TestSendTaskSuccessfulPublishLeavesOwnedPendingState` asserts pending-at-publish, a non-nil result, and no failure compensation.
3. Behavior §3 → `TestSendTaskPublishFailureConvergesOwnedPendingAttempt` asserts the neutral failure reason and wrapped primary publish error; reverting the conditional transition must fail it.
4. Behavior §4 → `TestSendTaskPublishFailurePreservesAdvancedState` covers received, started, retry, success, and failure before the publish error; `TestSendTaskAckLostRacePreservesWorkerState` races worker completion against compensation under `-race`.
5. Behavior §5 → `TestOldPublishFailureDoesNotClobberNewAttempt` blocks an old publish, starts a newer send with the same task ID, then releases old compensation and asserts the newer pending state survives.
6. Behavior §6 → `TestSendTaskUnsupportedBackendSkipsUnsafeCompensation` uses a backend that implements only `backend.Backend` and asserts the primary error plus zero failure writes.
7. Behavior §7 → `TestSendTaskPublishCompensationPreservesBothErrors` asserts `errors.Is` for both causes and contextual messages.
8. Behavior §8 → backend conformance tests `TestPublicationAttemptCompensation` in Redis, SQL/SQLite, and live MongoDB assert matching/non-matching CAS behavior; `TestPublicationAttemptCompensationLiveSQL` and `TestPublicationAttemptCompensationRealRedis` exercise configured live services. Redis additionally asserts unchanged TTL; SQL and MongoDB assert unchanged `create_at`, with `TestBuildTaskStatusUpdatePreservesCreateAtOnExistingRows` pinning the MongoDB update shape for the synthetic merge with `origin/pr/120`.
9. Behavior §9 → each backend's `TestPublicationAttemptCompensationLegacyRecordDoesNotMatch` creates a missing-attempt record and proves conditional failure is a no-op.
10. Behavior §10 → `TestNonFailureTransitionsClearPreviousError` in SQL, MongoDB, and Redis proves stale error text is cleared; the backend legacy-ownership and retention tests separately pin the required attempt and lifetime semantics.
11. Behavior §11 → `TestOrdinaryCallbackPublicationErrorsReachErrorHandler` covers success and error callbacks and asserts the handler receives both joined causes without payload text.

Verification runs include focused tests, `go test -race ./distributed/...`, backend integration harnesses (live MongoDB when available, SQL through the repository's SQLite harness, Redis through miniredis and a real Redis container when available), `go vet ./distributed/...`, `git diff --check`, an attempt-CAS mutation, and a synthetic merge with `origin/pr/120`.

## Reconciliation

Reconciled on 2026-07-12 against the implementation and tests in `distributed/service.go`, `distributed/worker.go`, and `distributed/backend/`; the final PR head and synthetic-merge results are verified during the publish gate rather than embedded as drift-prone SHAs here.

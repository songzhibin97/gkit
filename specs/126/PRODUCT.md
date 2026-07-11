# Callback publication compensation

## Summary

Single-task publication failures must not leave unreachable tasks pending or overwrite work that a broker already accepted. Compensation is safe only when it still owns the pending publication attempt, and callback dispatch failures must remain observable.

## Behavior

1. A publication attempt identifier is created before any task state is written or any publish is attempted; if identifier creation fails, the send returns that error with no backend or controller side effects.
2. A task is durably pending before publication. A successful publication returns an asynchronous result and does not mark the task failed.
3. When publication returns an error, compensation changes the task to failure with the neutral reason `task publication outcome unknown` only if the task is still pending for that exact publication attempt; the publication error remains the primary returned error.
4. Compensation never overwrites a task that has reached received, started, retry, success, or failure, including when that transition races with compensation.
5. A newer publication attempt using the same task ID is never changed by compensation from an older attempt.
6. A backend that does not support attempt-aware atomic compensation keeps the legacy pending-before-publish behavior; on publication failure it returns the publication error without attempting an unsafe state overwrite.
7. If attempt-aware compensation itself fails, the caller receives one error value that preserves both the publication and compensation causes.
8. SQL, MongoDB, and Redis backends apply the same attempt-aware pending and conditional-failure behavior without shortening result retention; SQL and MongoDB also preserve an existing task creation timestamp.
9. Existing task records without publication-attempt metadata never match attempt-aware compensation.
10. A later pending, received, started, retry, or success transition clears stale failure text while preserving the attempt and retention behavior required by that backend.
11. Publication or compensation failures from ordinary success and error callbacks are delivered to the worker's configured error handler, or its existing structured error path when no handler is configured; callback arguments and other sensitive payload data are not emitted.

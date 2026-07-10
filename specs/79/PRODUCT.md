# Distributed correctness hardening

## Summary

The distributed task APIs must preserve retry timing, routing, group publication, consumer lifecycle, retention, result polling, and metadata across their supported backends. Failures must return promptly and explicitly instead of panicking, deadlocking, silently losing queued work, or leaving callers with mutable partial results.

## Behavior

1. A newly created task signature uses a 60-second retry interval. Automatic retry scheduling always produces a non-negative future delay, including when a caller supplies an interval near the largest representable integer; arithmetic overflow must never turn a retry into an immediate or past-due task.
2. Sending a task group with a concurrency value less than one behaves as concurrency one. Cancellation is honored, and the call does not return while any publisher it started can still mutate the returned result slice.
3. Listing delayed tasks returns every queued delayed task without a storage-type error, ordered from the earliest scheduled task to the latest.
4. When the queue connection health check fails, starting a consumer never panics. With retry disabled it returns the connection-closed outcome; with retry enabled it invokes the configured retry behavior and reports that another attempt is allowed.
5. Every consumer attempt owns exactly one bounded set of queue and delayed-task producers. On any return path, that attempt cancels and joins its producers and active processors; it performs no further queue pops after returning, and a task popped but not handed to a processor is restored rather than silently lost.
6. Taking over a Redis-backed group identifier that already exists returns promptly with an identifiable conflict error. Each timed group invocation derives unique runtime group and task identifiers from the registered templates, so overlapping invocations neither wait on nor overwrite one another.
7. Mongo-backed result retention expires task and group records from their creation time. A zero retention value selects the documented default, a negative value disables expiration, and a positive value within the Mongo driver's supported TTL range expires both record types after that many seconds; an oversized value is rejected instead of overflowing. Index-setup failures are observable during construction, and the compatibility constructor never returns a usable backend that failed initialization.
8. Group publication initializes every member as pending before publishing any member. If pending initialization fails, no member is published. Once publication begins, the method joins all started publishers, returns a stable result slice, deterministically reports publication or cancellation errors, and identifies publication failures as publication failures.
9. A task whose router is unset is bound to the server's current consume queue at send time, for both individual and group sends. A caller-supplied non-empty router is preserved unchanged.
10. Result monitoring and blocking result retrieval return backend read errors to the caller instead of treating them as a still-pending task. Existing state inspection remains source-compatible, with an additive error-returning state inspection available to callers that need the failure.
11. Task metadata survives JSON serialization and deserialization and signature copying. The resulting metadata remains readable and writable, an empty or decoded metadata value is safe to use, and copying metadata does not copy synchronization state.

## Non-goals

- Issue #79 item 10 is not part of this change. Concurrent lifecycle writes for the same task identifier and a cross-backend monotonic state machine are not currently promised; retry transitions are not numerically monotonic. This change does not add compare-and-swap state transitions or alter Redis, MongoDB, or SQL state ordering.
- Issue #79 item 11 is not part of this change. The backend interface does not define a missing group-member status as either an error or an incomplete group, so this change does not normalize Redis missing-key behavior or change the SQL/MongoDB query semantics.
- No exported method signature is removed or changed. Where an existing signature cannot expose an initialization or read error, a compatible additive API is used.

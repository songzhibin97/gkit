# Reliable Redis task delivery

## Summary

Redis-backed consumers keep accepted tasks recoverable until processing is acknowledged, including consumer failures, lease expiry, and Redis command ambiguity. Existing controller, processor, producer, queue-name, and serialized-task contracts remain unchanged.

## Problem

Removing a task from the ready queue before processing creates a loss window: decoding, processing, Redis, or process failure can leave no durable indication that the task is unfinished. Reliable delivery closes that window with at-least-once recovery while keeping the current public API.

## Behavior

1. An accepted task remains recoverable in either ready or reserved state until its delivery is successfully acknowledged.
2. At any instant, only one unexpired delivery owns the right to process and finalize a reserved task.
3. A delivery is acknowledged only after processing succeeds, or when an unregistered task is explicitly configured to be ignored.
4. When processing returns an error, that error remains visible and the same task is retained as a deferred delivery for a later attempt.
5. A consumer may reclaim an abandoned delivery only after its visibility period expires; reclaiming preserves the exact task bytes.
6. A renewal changes the locally trusted delivery deadline only after Redis confirms it. A transport failure before Redis receives the renewal leaves both the trusted deadline and server-side visibility unchanged.
7. After a delivery expires, its token cannot renew, acknowledge, or release the task, and those rejected operations do not mutate it.
8. A failure after processing succeeds but before acknowledgement may cause another delivery, but every attempt preserves the original serialized task and task ID.
9. `Stop` waits for an active processor to return and then waits for its in-flight renewal to finish before finalization and shutdown complete. It does not close the caller-owned Redis client, and cannot guarantee termination if the processor never returns.
10. Acknowledgement confirmation is retained for 24 hours using Redis server time. Expired confirmation cleanup is bounded per operation, and the confirmation key has bounded lifetime after traffic stops.
11. Existing public APIs, serialized tasks, ready queues, delayed scheduling, pending-task inspection, and tagged, untagged, or malformed-brace Redis Cluster queue names continue to work without queue renaming.
12. A malformed serialized task remains recoverable and does not disappear merely because it cannot be decoded.
13. A failure at an internal transition boundary retains either a recoverable task copy or durable acknowledgement evidence; it never loses both.
14. Consecutive deferred failures increment their retained failure count and use deterministic jittered exponential delay: approximately 1, 2, 4, 8, 16, and 32 seconds, then a saturated 48-to-60-second range.
15. An empty queue backs off to a steady 0.8-to-1.2-second polling interval, while work published at steady idle is discovered within 1.2 seconds.
16. Token entropy failure or four consecutive token collisions returns an explicit error without mutating ready or reserved work. This applies both when claiming and when moving a delivery into deferred retry state.

## Non-goals

- Exactly-once processing.
- A dead-letter queue, maximum delivery count, or permanent rejection policy.
- Forcibly canceling a processor that does not return.
- Global ordering across consumers or storage backends.
- Recovery after Redis itself loses acknowledged data.
- New public timing, retry, lease, clock, or token configuration APIs.
- Replacing the existing list-backed queue with Redis Streams.

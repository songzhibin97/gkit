# Reliable Redis task delivery: technical design

## Context

`ControllerRedis.StartConsuming` owns the producer and processor lifetimes and joins them before returning (`distributed/controller/controller_redis/redis.go:123-196`). Delivery processing starts one renewal worker, joins it, and only then acknowledges or defers the task (`reliable_consumer.go:17-63`).

The reliable queue stores private lease state and executes atomic Redis transitions (`reliable_queue.go:19-95`). Claim, renewal, acknowledgement, release, and deferred retry are implemented by the production Lua sources and their Go response decoders (`reliable_queue.go:97-665`). Public construction and the caller-owned client contract remain in `redis.go:439-510`.

This document describes the implementation that ships for [PRODUCT.md](./PRODUCT.md), not a future design.

## Proposed changes

### Reservation state and key placement

The existing queue remains a LIST of the original serialized bytes. Private same-slot keys hold inflight envelopes, visibility deadlines, acknowledgement outcomes, a repair cursor, and a repair backlog. Tagged queues reuse their tag; other queue names derive a deterministic tag for their existing Redis Cluster slot. A digest of the full queue name keeps distinct queues separate.

An inflight envelope contains a saturated decimal failure count and the exact legacy payload. Claim and reconciliation validate all key types and arguments before their first write. Destination state is written before destructive source removal, and repair work is limited to 128 entries per claim. Oversized scan pages persist the unprocessed tokens in the repair backlog before advancing the cursor.

### Lease time and finalization

Lua uses Redis `TIME` for visibility and acknowledgement decisions. A successful claim or renewal returns both Redis server time and the confirmed deadline. The client converts their difference into a local monotonic deadline anchored before the request; `reliableQueue.renew` updates it only after a successful script response.

Processing owns one delivery-scoped renewal goroutine. Processor return cancels that goroutine and joins it before acknowledgement or deferred retry begins. Finalization has a private bounded timeout. Acknowledgement retries the same token during a private confirmation window so a committed command whose response was lost can be confirmed by its retained outcome.

Acknowledgement outcomes score at Redis `TIME + 24h`. One acknowledgement removes at most 128 expired outcomes and refreshes a 25-hour key TTL. Claim-side maintenance also performs bounded expiry cleanup and repairs a missing TTL from retained scores.

### Retry and token safety

Processing and decode failures move the envelope to a fresh reservation. The retained failure count increments and selects deterministic SHA-256 jitter over an exponentially increasing base, saturated at 60 seconds. Pre-processing cancellation uses immediate release instead.

Tokens contain 128 random bits. Claim, reclaim, and deferred retry make at most four generation attempts. Every moving transition verifies the candidate is absent from inflight, visibility, acknowledgement-outcome, and repair-backlog state before writing; exhaustion leaves the original ready item or reservation byte-for-byte intact.

### Shutdown and idle polling

`StopConsuming` marks the controller as stopping, cancels the consume attempt, and waits for `StartConsuming` to join producers and processors. It does not cancel a processor that is already running. When that processor returns, its renewal operation is canceled and joined before finalization. The borrowed Redis client remains open.

Empty claims back off from 25 milliseconds to a one-second base with deterministic plus-or-minus-20-percent jitter. Available work resets the counter. This implementation uses real timers; there is no fake-clock hook or public clock option.

## Testing and validation

Each PRODUCT behavior has one primary regression test:

| Behavior | Primary proof | Mutation target |
| --- | --- | --- |
| 1 | `TestClaimPersistsUntilSuccessfulAck` | Remove the inflight write or acknowledge before processing returns. |
| 2 | `TestConcurrentClaimHasSingleLeaseOwner` | Permit two claims to own the same ready item. |
| 3 | `TestAckOccursOnlyAfterProcessorSuccess` | Acknowledge processor failures or every unregistered task. |
| 4 | `TestProcessorErrorDefersDelivery` | Delete the reservation or swallow the processor error. |
| 5 | `TestAbandonedDeliveryReclaimedAfterVisibility` | Reclaim before expiry or fail to reclaim after expiry. |
| 6 | `TestRenewTransportFailureDoesNotAdvanceConfirmation` | Advance `confirmedUntil` after a `BeforeProcess` transport failure that never reaches Redis. |
| 7 | `TestExpiredTokenCannotFinalizeDelivery` | Validate token existence without checking the Redis deadline. |
| 8 | `TestCrashAfterProcessBeforeAckPreservesIdentity` | Re-serialize with a new ID or discard the reserved payload. |
| 9 | `TestStopJoinsHeartbeatAndOwnedDeliveries` | Skip the renewal join and begin ACK/return while the renew hook is blocked. |
| 10 | `TestReliableLuaRedis7AckOutcomeUsesRedisTimeAndBoundedCleanup` | Use a non-24-hour score, client time, or unbounded/absent expired-outcome cleanup. |
| 11 | `TestLegacyQueueAndClusterKeyCompatibility` | Change queue names, derive another slot, or key the bounded cache by full queue name. |
| 12 | `TestMalformedPayloadRemainsRecoverable` | Acknowledge or delete bytes after decode failure. |
| 13 | `TestLuaFailureBoundariesRetainRecoverableCopyOrAck` | Move a destructive write before its destination copy or remove ACK evidence. |
| 14 | `TestPermanentFailuresUseBoundedBackoff` | Return a fixed one-second delay, reset the persisted count, or remove saturation. |
| 15 | `TestIdleClaimRequestsAreBounded` | Keep a fixed tight poll interval, omit the cap/reset, or exceed the 1.2-second discovery bound. |
| 16 | `TestDeliveryTokenGenerationIsBounded` | Remove claim/defer collision prevalidation, retry forever, or overwrite a reservation. |

Behavior 15 is measured by Redis command-hook timestamps and real wall-clock deadlines in the test; it is not a fake-clock proof. The remaining unit tests use miniredis, deterministic token readers, and command hooks. Lua semantics, acknowledgement retention/cleanup, failpoint boundaries, stale-backlog repair, and Redis server time run against Redis 7. Cluster compatibility runs against three Redis 7 masters.

Required gates:

- `go test -race ./distributed/controller/controller_redis`
- focused `go test -race ./distributed/...`
- `go vet ./distributed/controller/controller_redis ./distributed/...`
- Redis 7 standalone live tests with `GKIT_REDIS_TEST_ADDR`
- three-master Redis 7 Cluster tests with `GKIT_REDIS_CLUSTER_TEST_ADDR`
- mutation checks for Behaviors 6, 9, 10, 14, and 16

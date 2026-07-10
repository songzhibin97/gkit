# Distributed correctness hardening: technical plan

## Context

[PRODUCT.md](./PRODUCT.md) defines the caller-visible contract. The current implementation violates it in these seams:

- Retry defaults and retry ETA arithmetic: `distributed/task/signature.go:48-71`, `distributed/retry/fibonacci.go:19-39`, `distributed/worker.go:115-130`.
- Group initialization and concurrent publication: `distributed/service.go:151-220`.
- Default routing and timed group template reuse: `distributed/service.go:120-140,298-353`, `distributed/task/signature.go:48-71`.
- Redis consumer lifecycle and delayed listing: `distributed/controller/controller_redis/redis.go:75-160,162-350` and `distributed/broker/broker.go:78-113`.
- Redis group takeover: `distributed/backend/backend_redis/redis.go:69-92`.
- MongoDB retention initialization: `distributed/backend/backend_mongodb/mongo.go:17-39,208-238`; record creation dates are `distributed/task/state.go:132-143` and `distributed/task/group.go:30-47`.
- Result polling: `distributed/backend/result/async_result.go:70-127`.
- Metadata serialization and copying: `distributed/task/meta.go:5-46`, `distributed/task/signature.go:74-85`, and the unexported-field behavior in `tools/deepcopy/deepcopy.go:87-95`.

The implementation remains scoped to the confirmed REAL items in issue #79. PRODUCT Non-goals preserve the current state-update and missing-status contracts.

## Proposed changes

### Behavior 1: retry units and overflow

- Change the `NewSignature` default in `distributed/task/signature.go` from a nanosecond-valued duration conversion to the integer value `60`, documented as seconds.
- Make `distributed/retry/fibonacci.go` saturate instead of overflowing when no larger Fibonacci value fits in `int`.
- Add a small retry-delay conversion helper in `distributed/worker.go` that clamps seconds to the largest positive `time.Duration` before multiplication. Store the clamped seconds back on the signature so subsequent retries remain safe.
- Keep the exported `RetryInterval int` and option signatures unchanged.

### Behaviors 2 and 8: group send ownership

- Rewrite `Server.SendGroupWithContext` in `distributed/service.go` around explicit ownership: normalize concurrency to at least one, finish all pending-state initialization before starting publication, start at most the requested number of publishers, and wait for every started publisher before inspecting results or returning.
- Store publication errors by task index and scan them in task order after joining. This removes channel/select timing from error precedence and gives deterministic results.
- Bind cancellation to scheduling new publishers while still joining publishers already started. Wrap errors with the operation and task identifier (`set state pending` versus `publish task`).
- On pending initialization failure, best-effort removal of the newly created group and statuses already initialized prevents the new fast-conflict behavior from making an immediate retry impossible. Cleanup failures are attached as context without hiding the initiating error.
- Track whether any publisher was admitted. If cancellation prevents every publisher from starting, use the same group/status rollback path so retrying the same group ID is safe.
- Once any publish attempt starts, retain the group and converge every failed or never-admitted member to `FAILURE` after all publishers join. Do not write the state of a publisher that returned success. Persist a bounded generic failure reason, while returning the original publish/cancellation error and joining any convergence-write errors so operational details are not swallowed or copied into result storage.
- Do not add state ordering or missing-status handling.

### Behavior 3: delayed listing

- Change `ControllerRedis.GetDelayedTasks` in `distributed/controller/controller_redis/redis.go` to read the delayed sorted set in ascending score order, then decode members exactly as today.
- Keep the exported controller interface unchanged.

### Behaviors 4 and 5: one owned consumer attempt

- In `ControllerRedis.StartConsuming`, check that a retry function exists before invoking it on health-check failure.
- Create a child context for each successful consumer attempt. Pass that context to queue popping, delayed popping, publication, and worker coordination instead of using an uncancelled attempt-independent context.
- Replace buffered prefetch with an owned handoff that can select on cancellation. If cancellation wins after a queue pop but before processor handoff, put the payload back at the front of its source queue.
- On processor, queue-producer, or delayed-producer failure, record the first concrete error, cancel the attempt, and join both producers and all active processors before returning. Error publication is buffered/non-blocking so shutdown cannot deadlock.
- Add context-taking internal pop helpers while retaining existing internal wrappers used by focused tests. `StopConsuming` continues to cancel the broker root context and waits for active attempts; no exported signature changes.
- Make `Publish` honor its supplied context. If a delayed payload is removed but cannot be republished because its attempt was cancelled, restore it with its original schedule before the attempt returns; restoration failure is surfaced.

### Behavior 6: group conflicts and timed identity

- Add an exported sentinel conflict error in `distributed/backend/backend_redis/redis.go`. `GroupTakeOver` performs one `SetNX`; a false result returns a wrapped sentinel containing the group ID instead of sleeping and retrying forever.
- Preserve the timed registration APIs and their `groupID`/signature arguments as templates. In `distributed/service.go`, each cron invocation generates one cryptographically random run suffix. The runtime group ID is the registered group prefix plus that suffix.
- Copy every signature before mutation. Runtime task IDs retain the registered ID as a readable prefix and append the run suffix plus a deterministic path/index; nested success/error/chord callback IDs receive distinct paths. This keeps caller templates unchanged, prevents collisions within a run, and makes overlapping runs independent without adding a new public option.
- Apply the same rule to timed group callbacks, including copying and re-keying the callback template. Non-timed group APIs preserve their exact IDs and existing uniqueness responsibility.

### Behavior 7: MongoDB retention and constructor compatibility

- Normalize MongoDB retention in `distributed/backend/backend_mongodb/mongo.go`: zero becomes the same 3600-second default used by Redis; any negative value means no TTL index; positive values are validated before conversion to the driver's integer width.
- Build separate, explicitly named single-field TTL index models on `create_at` for the task collection and the group collection. Do not create TTL options for negative retention.
- Add `NewBackendMongoDBE(client, resultExpire, options...) (backend.Backend, error)`, matching the repository's additive SQL constructor pattern. It returns index-creation errors.
- Keep `NewBackendMongoDB` source-compatible, mark it deprecated, implement it through the error-returning constructor, and return `nil` when initialization fails rather than returning a backend that only appears initialized.
- `SetResultExpire` normalizes the stored value for compatibility but does not claim to rebuild an already-created index because its interface cannot return an error. The constructor value is the retention authority for index creation; this limitation is documented rather than silently represented as a successful online reconfiguration.

### Behavior 9: late default routing

- Use an empty router as the `NewSignature` marker for “not explicitly set.”
- Immediately before each individual or group publication in `distributed/service.go`, replace only an empty router with `Server.config.ConsumeQueue`. Run the existing individual pre-publish hook first so a hook-provided router remains explicit.
- Never replace a non-empty router. Literal signatures with an empty router receive the same default behavior as constructor-created signatures.

### Behavior 10: result read errors

- Add `AsyncResult.GetStateWithError() (*task.Status, error)` in `distributed/backend/result/async_result.go`.
- Make `Monitor` use the additive method and propagate backend errors; `Get` and `GetWithTimeout` already return `Monitor` errors and therefore stop polling immediately.
- Keep `GetState() *task.Status` as a compatibility wrapper that returns the cached state and discards the error, preserving existing source behavior for direct callers.

### Behavior 11: metadata value ownership

- Implement JSON marshal/unmarshal on `task.Meta` in `distributed/task/meta.go` using a snapshot of the data map, never the mutex.
- Make `Set`, `Get`, and `Range` tolerate a nil receiver or nil internal map; initialize the map on the first writable operation. Use a read lock for read-only operations when safety is enabled.
- Normalize a signature's metadata safety after construction and JSON decoding so `MetaSafe` controls the reconstructed metadata.
- Extend `CopySignature` in `distributed/task/signature.go` with a graph walk that clones metadata values for the root and nested callbacks after the generic deep copy. Track visited signature nodes to preserve cycles and avoid copying lock state.

## Testing and validation

All new regression tests are written and run against the unmodified implementation first. The expected red output is retained in the task handoff, then the same commands must be green after implementation.

1. **Behavior 1** → `distributed/task/signature_issue79_test.go` asserts a default of 60 seconds; `distributed/retry/fibonacci_issue79_test.go` covers `math.MaxInt`; `distributed/retry_issue79_test.go` asserts extreme intervals produce a positive, bounded duration. Mutation: restore `int(time.Minute)` and remove the clamp independently; each relevant test must fail.
2. **Behavior 2** → `distributed/service_issue79_test.go` uses fake backend/controller implementations to prove zero concurrency completes, cancellation cannot return before a blocked publisher is released, and the result slice is stable under `go test -race`. Mutation: restore the `< 0` condition or return before `Wait`; the timeout/race test must fail.
3. **Behavior 3** → `distributed/controller/controller_redis/issue79_test.go` publishes multiple future tasks to miniredis and asserts `GetDelayedTasks` succeeds in ETA order. Mutation: replace the sorted-set read with the old list read or reverse ordering; the test must fail with WRONGTYPE/order mismatch.
4. **Behavior 4** → the same controller test file closes miniredis before `StartConsuming`; retry-disabled mode must return `controller.ErrorConnectClose` without panic. A custom retry callback proves retry-enabled mode is invoked. Mutation: make the callback unconditional; the no-retry test must panic/fail.
5. **Behavior 5** → `distributed/controller/controller_redis/consumer_lifecycle_issue79_test.go` makes one processor fail while another remains active, asserts `StartConsuming` joins processors and producers before return, asserts queue length no longer decreases after return, and asserts a payload cancelled between pop and handoff is restored. A retry-enabled test counts active producers across consecutive attempts, and a delayed-republish test verifies cancellation restores the original score. Run these under `-race`. Mutation: omit attempt cancellation, either join, either restore path, or supplied-context publication; the corresponding coordination assertions must fail.
6. **Behavior 6** → `distributed/backend/backend_redis/issue79_test.go` uses miniredis to assert duplicate takeover returns the sentinel within a short deadline. `distributed/service_timed_group_issue79_test.go` builds overlapping timed-run instances and asserts group IDs, top-level task IDs, nested callback IDs, and the group callback ID are unique while templates remain unchanged. Mutation: restore the SetNX loop or fixed IDs; deadline/uniqueness tests must fail.
7. **Behavior 7** → `distributed/backend/backend_mongodb/result_expire_issue79_test.go` tests pure index-model construction: task/group keys are `create_at`, zero normalizes to 3600, negative creates no TTL models, and positive values are retained. A disconnected client with a short server-selection timeout proves `NewBackendMongoDBE` returns an error and the legacy constructor returns nil. No external MongoDB is required. Mutation: key either model on `status`/`lock`, apply TTL for a negative value, or swallow the constructor error; the corresponding assertion must fail.
8. **Behavior 8** → fake-based service tests make pending initialization fail and assert publish count stays zero; concurrent publishers fail in a deliberately reversed completion order and the returned error still follows task order with `publish task` context. Deterministic miniredis tests cancel immediately after pending initialization and assert complete rollback plus same-ID retry, cancel after one publisher starts and assert all unadmitted members become terminal without overwriting the successful member, and mix one publish failure with one success to assert the group converges. A fake convergence-write failure proves both the original and cleanup errors remain discoverable. Mutation: continue after pending failure, select the first channel arrival, remove zero-admission rollback, or omit failure convergence; the corresponding test must fail.
9. **Behavior 9** → service tests cover individual and group signatures with an empty router under a custom consume queue, plus an explicit-router control. Mutation: restore the `gkit` constructor default or overwrite non-empty routers; the tests must fail.
10. **Behavior 10** → `distributed/backend/result/async_result_issue79_test.go` uses a backend that always errors. `Monitor`, `Get`, and `GetWithTimeout` must return that error immediately; `GetState` remains callable and `GetStateWithError` exposes it. Mutation: make `Monitor` call the compatibility wrapper; propagation tests must fail or time out.
11. **Behavior 11** → `distributed/task/meta_issue79_test.go` checks JSON round-trip, copy independence, nested callback metadata, zero-value metadata writes, nil-safe reads, and concurrent safe access under `-race`. Mutation: remove custom serialization, copy normalization, or lazy map initialization; value/panic/race assertions must fail.

Final verification:

- `gofmt` on every changed Go file.
- Focused miniredis/fake regression tests during development.
- Run the touched deterministic package suites under `-race`; run `backend_mongodb` with the issue-scoped pure/disconnected tests unless a live MongoDB is explicitly available. The repository's older Mongo integration tests perform lazy connection attempts and are not an offline verification gate.
- `go vet ./distributed/...`.
- `git diff --check` and a final traceability pass confirming every PRODUCT Behavior 1–11 has the test listed above.

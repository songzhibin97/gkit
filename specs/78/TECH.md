# Cache correctness and contracts: technical plan

## Context

GitHub issue #78 covers five implementation defects and three public-contract ambiguities in the cache packages.

- `cache/buffer/iobuffer.go:74-132` implements the synchronized pipe surface and its terminal broadcast.
- `cache/buffer/iobuffer.go:150-165` implements the empty and zero-length read contract.
- `cache/buffer/iobuffer.go:239-249` and `487-522` extend readable buffer length and identify the newly added range.
- `cache/local_cache/cache.go:1029-1106` owns deletion, capture snapshots, and off-lock callback invocation.
- `cache/local_cache/cache.go:1173-1190` collects expired keys and routes cleanup through a write-locked expiry recheck.
- `cache/local_cache/cache.go:24-52`, `cache/local_cache/option.go:24-30`, and `cache/local_cache/cache.go:1207-1211` define janitor startup and shutdown ownership.
- `cache/mbuffer/mcache.go:47-90` returns and recycles pooled byte slices without clearing them.

`specs/78/PRODUCT.md` is the source of truth for caller-observable behavior.

## Proposed changes

### Buffer behavior

- Update empty reads to check the actual exhausted state while preserving the zero-length-read exception.
- Broadcast a terminal pipe close to every condition waiter; keep ordinary writes as single-reader signals.
- After `Grow` extends readable length, zero exactly the appended range with a Go 1.20-compatible loop. Preserve the existing prefix and do not zero on `Write` or other paths that immediately overwrite the extension.

### Local-cache concurrency

- Add an expiration-specific deletion path that acquires the write lock, re-reads the current entry, and deletes only when that current entry is still expired.
- Snapshot the active capture handler while holding the cache write lock, release the lock, and invoke only that snapshot afterward.
- Make the internal map deletion primitive independent of whether a capture handler is installed, so deletion and callback ownership remain separate.
- Preserve off-lock capture invocation so reentrant handlers cannot deadlock.

### Contract documentation

- Document the four pipe methods that may be used concurrently and require caller synchronization for the rest.
- Document `Shutdown` ownership wherever a positive janitor interval is configured.
- Document that `Malloc` returns pooled bytes with unspecified contents.

No new exported type, feature flag, configuration option, finalizer, or synchronization abstraction is introduced.

## Testing and validation

1. Behavior §1 → `cache/buffer/issue78_test.go:TestIoBufferReadEmptyReturnsEOF`, covering construction and reset. Mutation: restore the old positive-offset guard; the new-buffer case must fail.
2. Behavior §2 → `cache/buffer/issue78_test.go:TestIoBufferZeroLengthReadReturnsNil`, covering empty and non-empty buffers plus non-consumption. Mutation: return EOF before the zero-length exception; the empty case must fail.
3. Behavior §3 → `cache/buffer/issue78_test.go:TestPipeCloseWakesAllBlockedReaders`, which waits until two readers are parked before closing. Mutation: replace broadcast with a single signal; one reader must remain blocked and the test must fail.
4. Behavior §4 → `cache/local_cache/issue78_test.go:TestIteratorDoesNotDeleteFreshReplacement`, which blocks the first eviction callback, installs fresh values for every observed expired key, then releases iteration. Mutation: delete without the write-locked expiry recheck; fresh replacements must disappear and the test must fail.
5. Behavior §5 → `cache/local_cache/issue78_test.go:TestCaptureConcurrentChangeAndDelete`, run with `-race`. Mutation: read the handler after unlocking; the race detector must report the read/write conflict.
6. Behavior §6 → existing `cache/local_cache/capture_test.go:TestCapture_ReentrantNoDeadlock`. Mutation: invoke capture while holding the write lock; the reentrant callback must time out.
7. Behavior §7 → `cache/buffer/issue78_test.go:TestIoBufferGrowZeroInitializesNewRegion`, using a fixed non-zero backing pattern to prove prefix preservation and zero initialization. Mutation: remove the zeroing loop; the test must observe the fixed pattern and fail.
8. Behavior §8 → `go doc ./cache/buffer NewPipe | rg -q 'Read, Write, CloseWithError, and Len'` and `go doc ./cache/buffer NewPipe | rg -q 'own synchronization'` verify the concurrent-safe subset and caller responsibility. Mutation: remove either contract sentence; its command must fail.
9. Behavior §9 → `go doc ./cache/local_cache NewCache | rg -q 'must call Shutdown'` and `go doc ./cache/local_cache SetInternal | rg -q 'must call Cache.Shutdown'` verify lifecycle ownership. Mutation: remove either requirement; its command must fail.
10. Behavior §10 → `go doc ./cache/mbuffer Malloc | tr '\n' ' ' | rg -q 'not *guaranteed to be zeroed'` verifies the pooled-memory contract despite `go doc` line wrapping. Mutation: remove the sentence; the command must fail.

Final verification:

```sh
gofmt -w cache/buffer/iobuffer.go cache/buffer/issue78_test.go cache/local_cache/cache.go cache/local_cache/issue78_test.go cache/local_cache/option.go cache/mbuffer/mcache.go
go test -race ./cache/buffer ./cache/local_cache ./cache/mbuffer
go vet ./cache/...
```

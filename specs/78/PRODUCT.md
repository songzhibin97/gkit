# Cache correctness and contracts

## Summary

Cache buffers and local caches remain live and deterministic at empty, close, reuse, and concurrent-update boundaries. Public documentation states the concurrency, shutdown, and memory-initialization responsibilities that callers must follow.

## Behavior

1. Reading an empty I/O buffer into a non-empty destination returns zero bytes and `io.EOF`, including immediately after construction or reset.
2. Reading with a zero-length destination returns zero bytes and no error, whether the I/O buffer is empty or contains data, and does not consume data.
3. Closing a pipe wakes every read that is already blocked waiting for data; after buffered data is exhausted, each read returns the terminal error, with a nil close error represented as `io.EOF`.
4. If an iterator observes an expired cache entry and a concurrent caller replaces that key with an unexpired value before eviction, the replacement remains present.
5. Deletion, expiration cleanup, and capture-handler replacement can run concurrently without a data race.
6. A capture handler runs after the cache lock is released and may safely call back into the same cache.
7. Growing an I/O buffer preserves all existing readable bytes and initializes only the newly readable region to zero.
8. A pipe supports concurrent calls to `Read`, `Write`, `CloseWithError`, and `Len`; callers must provide their own synchronization before mixing any other I/O-buffer method with concurrent pipe operations.
9. A cache configured with a positive janitor interval requires its caller to invoke `Shutdown` when the cache is no longer needed.
10. Memory returned by `Malloc` has the requested length and capacity but its byte contents are unspecified and are not guaranteed to be zeroed.

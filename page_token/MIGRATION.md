# Timestamp format migration

Tokens issued by `ForIndex` now encode the issue instant as `v2|<Unix nanoseconds>`.
The resource delimiter, `:<index>` suffix, salt options and AES-GCM envelope are
unchanged. TTL compares absolute instants, independent of the reader's local zone.

`GetIndex` also accepts the previous AES-GCM plaintext timestamp format
`2006-01-02 15-04-05`. With TTL enabled, this legacy timestamp is still interpreted
in the reader's `time.Local`, preserving the previous same-zone behavior. An old
token carries no issuer zone, so this fallback cannot correct cross-zone legacy
TTL. Keep legacy issuers and readers in the same zone until those tokens expire,
or invalidate the old tokens and restart pagination.

Readers from before this change reject the new format when TTL is enabled.
Readers with TTL disabled skip timestamp parsing and can still read its index;
that behavior does not validate token age. Coordinate reader upgrades before
routing new tokens to TTL-enabled readers, and account for this limit when
rolling back. No separate writer-format configuration is provided.

Verified against `token.go` and `timezone_test.go` on 2026-09-13. Legacy here means
the prior AES-GCM timestamp payload, not the earlier unauthenticated CBC format.

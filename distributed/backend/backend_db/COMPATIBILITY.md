# SQL group task ID encoding compatibility

Reconciled against `distributed/task/group.go` and the SQL backend tests on
2026-07-11.

`GroupMeta.TaskIDs` is stored in the SQL `group_meta.task_ids` `TEXT` column.
Historical rows use a comma-joined representation and are still read by
splitting on commas. New writes whose task IDs are comma-free and whose joined
bytes do not begin with the reserved marker keep those exact legacy bytes so
ordinary traffic remains readable during a rolling upgrade.

When any task ID contains a comma, or the legacy bytes would begin with the
reserved `gkit:string-slice:v1:` marker, new writers store that marker followed
by a JSON array of strings. Versioned reads require an array whose every element
is a JSON string. Malformed JSON, `null`, objects, numbers, and null elements are
reported as scan errors instead of being reinterpreted as legacy data.

## Upgrade boundary

The marker is a reserved namespace, not a collision-proof discriminator. A
single existing `TEXT` column cannot distinguish a historical task ID whose raw
legacy value begins with `gkit:string-slice:v1:` from a new versioned value.
Before deploying readers with this format, scan existing rows:

```sql
SELECT _id, id, task_ids
FROM group_meta
WHERE task_ids LIKE 'gkit:string-slice:v1:%';
```

For every match, recover the intended task ID list from an authoritative source
or backup and rewrite it as the versioned marker plus a JSON string array. An
invalid suffix will fail reads after upgrade; a suffix that is already a valid
JSON string array will be interpreted as versioned data. The compatibility test
`TestSQLGroupHistoricalMarkerRowsFollowReservedNamespace` pins both outcomes.

Historical comma-containing task IDs are also not recoverable automatically:
the old encoding maps multiple distinct ID lists to the same bytes. Do not guess
boundaries or bulk-split those rows as a repair. Reconstruct each affected list
from authoritative data and write the versioned representation.

No SQL schema migration is required. The scanner accepts `NULL`, `string`, and
`[]byte` driver values; ordinary historical comma-separated rows remain
readable. Old binaries cannot decode newly versioned comma-containing or
marker-prefixed rows, so do not submit those exceptional IDs until every SQL
backend process runs the new reader.

# Audit Log: Technical FAQ

## Design

### Why not just use the application log?
Application logs are for debugging. They are usually sampled, rotated, unstructured and writable by whoever runs the service. An audit log needs a fixed schema, a retention rule, and a guarantee that a successful change has a matching record.

### Why write the audit row in the same transaction?
So a change and its record exist together or not at all. Separate commits leave windows where one exists without the other. The cost is that an audit failure fails the operation, which is the intended behaviour for writes.

### Why is the rejected-attempt track "best effort"?
There is no business write to roll back with it, and failing a request because its denial could not be recorded would turn an audit outage into a login outage. The trade-off is that a denied attempt can be lost. Loss is counted in a metric so it is visible.

### Why not a message queue?
A queue adds a second system that can lose events and breaks the same-transaction guarantee. At this scale a synchronous insert is cheaper and simpler. A transactional outbox is the usual next step if events must go to other systems.

### Why no foreign keys?
The row must outlive the user or Idea it describes, and deleting an Idea must not delete or block its history.

### Why database time?
With several replicas, application clocks disagree. One clock gives one ordering.

## Granularity and cost

### Does more detail always cost performance?
Not linearly. The extra row in the same transaction usually costs little. The cost is mostly in indexes and large JSON columns. Keep `before` / `after` to changed fields, cap sizes, and index only what queries use.

### What do we do with high-frequency reads?
Do not write one row each. Record a metric, or one summary event per session and window. Record reads only for sensitive resources, such as an Admin viewing a single Idea or any slide download.

### One action touches several tables. How many events?
One, describing the business intent. Put the effect on related rows in `after`. Split only if an auditor will search for the related step on its own, or it has its own actor.

### What about cascading deletes?
Write one `IDEA_DELETE` event and put what was deleted with it in `before`. Because the child rows are gone, the audit row is the only history.

### How do we know it is detailed enough?
Take real scenarios, such as "who replaced these slides and with which file?", and check that one query answers each. A scenario you cannot answer means too little. A field no scenario uses means too much.

## Security

### What must never be stored?
Passwords, tokens, session IDs, CSRF values and file contents. The sanitizer redacts these keys by name. This is a safety net, not a substitute for not passing them in.

### How are log-injection attacks handled?
Control characters and line breaks are removed from client-supplied text, and values are cut to column limits.

### Can the log be tampered with?
Today the protection is a code convention plus a test, which stops the application from altering rows but not someone with database access. Stronger options: an insert-only database role for the application, a hash chain, signed checkpoints, and a copy in write-once storage. The hash-chain columns exist but nothing writes them.

### Does auditing a 404 leak whether a resource exists?
No. "Missing" and "not yours" are both audited as `DENIED`, so neither the response nor its timing differs. Only the Admin audit query can see the real reason.

### Who can read the log?
Admins, through the API. Every query is itself recorded as `AUDIT_QUERY`. There is no separate auditor role in v1, so an Admin can read records about their own actions.

## Operations

### How long is it kept?
At least 365 days, with no automatic purge in v1. For a table that grows, plan partitioning by time so old data can be dropped by partition rather than `DELETE`.

### How do we notice lost rejected-attempt rows?
Watch the audit-write-failures metric, which reads `Service.RejectedFailures()`.

### How do we follow one request across events?
Use `request_id`. Every row written during a request carries the same one.

### Why is paging by keyset, not offset?
Offset paging gets slower with depth and skips or repeats rows when new rows arrive. Keyset paging on `(occurred_at, id)` is stable and uses the index.

## Testing

### How do we prove "no record, no operation"?
Write an integration test that forces the audit insert to fail and asserts that the business change was rolled back. Add one per audited operation.

### How do we prove the table is append-only?
A test scans the Go sources for SQL that updates or deletes `audit_log`. It checks the application code, not the database.

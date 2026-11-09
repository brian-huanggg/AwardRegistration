# Audit Log: Architecture

## Two write tracks

The audit log has two tracks because two kinds of event have different failure requirements.

```
 successful write                      rejected attempt
 (track A)                             (track B)

 handler / service                     handler returns *web.Error
        |                                      |
 InTx( business write                   central error adapter
       + Audit.Record(tx) )                    |
        |                               Audit.RecordRejected
 one COMMIT, or one ROLLBACK                   |
                                        its own transaction
                                        (best effort, counted)
```

| | Track A: `Record` | Track B: `RecordRejected` |
|---|---|---|
| Outcomes | Any (normally `SUCCESS`) | `DENIED`, `FAILED`, `THROTTLED` |
| Transaction | The caller's transaction | Its own |
| If the audit write fails | The business operation rolls back (fail-closed) | The failure is logged and counted; the response is unchanged |
| Caller | The service that did the write | Only the central error adapter |

A download is served only after its audit row has been written.

Writing is synchronous. There is no message queue.

## Why the same transaction

If the audit row and the business change were committed separately, there would be a window where a change exists with no record, or a record exists for a change that never happened. With one transaction both either exist or neither does.

The trade-off is that audit availability is now tied to database availability for writes. That is the intended behaviour here: no record, no operation.

## Why a separate transaction for rejections

A rejected attempt has no business write to roll back with. It must also survive when the request fails, so it opens its own transaction on a context detached from the request. A client disconnect does not drop the row.

Because it is best effort, a failed track-B write increments a counter (`RejectedFailures`) that feeds the audit-write-failures metric, so loss is at least visible.

## Where rejections come from

Handlers return one error type carrying a status, a code and an audit reason. The central adapter in `internal/server` writes the problem+json response and calls `RecordRejected` when an audit reason is present. No other code calls it. `shared/web` defines the port, and `audit` implements it, so `shared` never depends on `audit`.

## What is and is not audited

| Audited | Not audited |
|---|---|
| Registration, login, logout | Ordinary list and read calls |
| Idea create, update, delete | Unauthenticated 401 (metric only) |
| Slide replace and download | Requests that match no route (metric only) |
| Team and membership changes | 409 and 422 input errors, except registration |
| Deadline change, Admin sync | `HEAD` requests (405, so they never write a download row) |
| Admin view of a single Idea | |
| Audit log queries | |
| Rejected attempts (`DENIED`) | |

Registration is the exception to "input errors are not audited": a 409 there would otherwise be a channel for enumerating employee IDs and emails.

### Uniform 404

A 404 on a single Idea is audited as `DENIED` both when the Idea does not exist and when it exists but is not the caller's. Auditing only one case would make their timing differ and break the uniform 404.

## Data model

One table, `audit_log`, created by migrations `000001` (skeleton) and `000002` (full columns).

| Column | Notes |
|---|---|
| `id` | Sequence; also the tie-breaker for ordering |
| `occurred_at` | Database default; the application never supplies time |
| `request_id` | UUID of the request |
| `actor_emp_no`, `actor_role` | `NULL` employee ID for `ANONYMOUS` and `SYSTEM` |
| `action`, `outcome` | Constrained value sets |
| `target_type`, `target_id` | `USER`, `IDEA`, `DEADLINE`; `DEADLINE` has no ID |
| `method`, `route`, `status`, `error_code` | `route` is the template, never the raw path |
| `ip`, `user_agent` | `INET6`; user agent cut to 256 characters |
| `submitted_emp_no` | Sanitized ID typed into a failed login or registration |
| `before_json`, `after_json` | Changed fields only |
| `prev_hash`, `hash` | Reserved for the optional hash chain; nothing writes them yet |

Design choices:

- **No foreign keys.** A row must outlive the user or Idea it describes.
- **Database time only.** Rows from different replicas order consistently.
- **Indexes** on `(occurred_at, id)`, `(actor_emp_no, occurred_at, id)`, `(request_id)` and `(target_id, occurred_at, id)`. Each extra index slows every write, so only the indexes the query uses are kept.

## Integrity

- The table is **append-only by convention**: application code only inserts. A test scans all `.go` files under `internal/` for SQL that updates or deletes `audit_log`.
- There is **no database-level protection yet**. Someone with direct write access to the table could change it.
- An optional hash chain is reserved. How to seal rows under concurrent writes is an open decision (OI-30), deferred until that work starts.

## Reading

`GET /admin/audit-logs` (Admin only, **planned: the endpoint is in the API contract but has no handler yet**).

- Filters: `from`, `to` (half-open), `actorEmpNo`, `action` (multiple), `outcome`, `targetType`, `targetId`, `requestId`, `ip`.
- Keyset pagination with `cursor` and `limit` (default 50, maximum 100), ordered by `occurredAt`, `id` descending. The condition is written out as `occurred_at < ? OR (occurred_at = ? AND id < ?)` because row comparison is not used.
- Every query is itself audited as `AUDIT_QUERY`, with the filters used.

## Retention

At least 365 days. There is no automatic purge in v1.

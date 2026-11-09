# Authorization & Uniform 404: Architecture

## Where each check runs

```text
Request
  │
  ▼
Request ID, recover, access log          middleware (outermost)
  ▼
413  body limit                          middleware
  ▼
401  session                             session middleware
  ▼
403  CSRF (writes)                       CSRF middleware, never reads the body
  ▼
404  can the caller see the resource?    service, one visibility query
  ▼
403  role                                service
  ▼
403  state (submission open?)            service pre-check
  ▼
400/422 validation                       body is decoded only now
  ▼
Transaction: lock row, re-check role and state, write, audit row, commit
```

Two rules shape this:

1. **Everything before validation happens before the multipart body is read.** An unauthorised caller cannot make the server buffer a 5 MiB upload.
2. **Role and state are checked twice.** The first check is a cheap read. The second runs inside the write transaction after the row lock, because the world may have changed between the two (deadline passed, leader changed).

Exception: `POST /join-requests` names its target in the body, so it validates the body first and looks up afterwards.

## The visibility query

All reads of an Idea go through one SQL predicate, `VisibleWhere`. No other code decides who may see an Idea.

```sql
(? = 1                                   -- caller is Admin
 OR i.leader_id = ?                      -- caller is the Leader
 OR EXISTS (SELECT 1 FROM idea_member m  -- caller is an ACCEPTED Member
            WHERE m.idea_id = i.id AND m.user_id = ? AND m.status = 'ACCEPTED'))
```

`GetVisible` runs `WHERE i.id = ? AND <VisibleWhere>`. "No row" is the only result for both a missing and a hidden Idea, so the code **cannot** tell them apart even by accident. `ListVisible` uses the same predicate, so the list and the detail never disagree.

## One error type, one adapter

```text
service  ── returns *web.Error{Status, Code, AuditReason} ──▶  error adapter
                                                                │
                                      writes problem+json ◀─────┤
                                      RecordRejected(DENIED) ◀──┘ if AuditReason set
```

Handlers do not write audit rows for refusals. They attach an `AuditReason` (`NOT_FOUND`, `ROLE`, `STATE`, `CSRF`) to the error. The central adapter writes the response and the `DENIED` row. This keeps the two paths identical for every 404.

## Audit track for refusals

A refusal cannot be written inside the business transaction, because the transaction rolls back. So denials use **track B**: `RecordRejected` opens its own short transaction. It uses a context that survives client disconnect, and it is best effort (a failure increments a metric, it does not change the response).

## What a 404 covers

| Case | Response | Audited |
|---|---|---|
| Idea does not exist | 404 `NOT_FOUND` | DENIED, reason `NOT_FOUND` |
| Idea exists, caller may not see it | 404 `NOT_FOUND` | DENIED, reason `NOT_FOUND` |
| Malformed Idea ID in path | 404 `NOT_FOUND` | DENIED, `targetId` null, route template recorded |
| Unknown path, with a session | 404 `NOT_FOUND` | Not audited (metric only) |
| Unknown path, no session | 401 | Not audited |

The malformed-ID case matters: answering `400` for a bad UUID but `404` for a good unknown one would let a caller probe the format. One rule covers all Idea paths.

## Router details that affect authorization

- chi gives a problem+json `404` and `405` through hooks, so unknown routes look like every other error.
- `HEAD` on the slide download answers `405` (explicit route plus a test).
- The generated server is the standard, non-strict one. A strict server would decode JSON before the handler, so a malformed body could answer `400` before `404` or `403`, which breaks the check order.

## Test architecture

| Test | What it proves |
|---|---|
| `testdata/permission-matrix.json` driver | Every cell of the matrix returns its status |
| Hidden vs missing, per endpoint | Same body (except `requestId`), same `DENIED` rows |
| Timing test (planned, Q7-02) | Latency of the two cases is statistically the same, audit write included |
| Check-order tests | A request failing two checks reports the earlier one |

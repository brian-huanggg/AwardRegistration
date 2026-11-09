# Transactions & Consistency: Architecture

## Where it lives

```text
shared/db      InTx, Outcome, error helpers (1205, 1213, 1062, CHECK)
   ▲
   │ every module that writes
idea  team  account  deadline  auth  audit
   │
   └──► storage.PendingDeletion   (REPLACED / DELETED / UPLOAD_ABORTED)
```

`shared/db` knows no business concept (D32). Services open the transaction. Handlers contain no SQL and no transaction code.

## The shape of every write

```text
validate input        (no database, no storage)
pre-check role/state  (cheap, may answer early)
upload file           (only slide writes)
┌──────────── InTx: READ COMMITTED, 10 s timeout ───────────┐
│ lock row  (FOR UPDATE)                                    │
│ re-check role and state   ← authoritative                 │
│ change rows                                               │
│ INSERT pending_file_deletion (replace / delete only)      │
│ audit Record (track A)                                    │
└───────────────────────────────────────────────────────────┘
COMMIT
then: delete old object / compensate new object
```

Nothing slow happens between the lock and the commit. No MinIO call, no network call to another service.

## Locks

| Case | Lock | Why |
|---|---|---|
| Approve, reject, remove, leave | Idea row `FOR UPDATE` | Capacity (≤ 3 Members) spans two tables (D8) |
| Replace slides, delete Idea | Idea row `FOR UPDATE` | Two replacements must not read the same old key and orphan one file |
| Apply to a team | none | The primary key `(idea_id, user_id)` is the guard |
| Deadline check | none | A race with a deadline change is rare and accepted (D7) |
| Clean-up job (next phase) | `SELECT ... FOR UPDATE SKIP LOCKED` | Several replicas may drain the table |

The lock order is always: **Idea row first**, then dependent rows. One order means no deadlock between two team operations.

## Role and state: checked twice

The first check (`Precheck`) runs before the body is read and keeps the error order of the PRD. The second (`AssertOpenInTx`) runs **inside the transaction** and is the authoritative one. Its `tx` parameter makes it impossible to call outside a write transaction.

## InTx classification

| Where it failed | Outcome |
|---|---|
| `BeginTx` failed | `RolledBack` |
| `fn` returned an error or panicked | `RolledBack` |
| `Commit` failed, SQLState class `23` (constraint) or `40` (rollback) | `RolledBack` |
| `Commit` failed any other way | **`Unknown`** |

`database/sql` gives one generic error for every commit failure, so `InTx` reads the driver error to decide. Every doubtful case is `Unknown`: mistaking a rollback for unknown only delays a clean-up, but the reverse loses data.

## Context and timeout

`InTx` runs on `context.WithoutCancel(ctx)` with its own 10 s timeout. `database/sql` rolls a transaction back when its context ends. Without this, a client that disconnects after the upload would cause a rollback, and the new file would be orphaned. Request values (request ID, principal) are kept.

## Lock contention

A lock-wait timeout (1205) or deadlock (1213) from `Begin`, the function or `Commit` becomes `503 SERVICE_UNAVAILABLE` and increments a metric. `InTx` always calls `Rollback` itself, because MariaDB rolls back only the **statement**, not the transaction, on 1205.

## Multiple replicas

Replicas share one database. Transactions and the Idea lock are the only coordination for writes. Background jobs (session purge, Vault sync) run on every replica with random jitter; there is no leader, so each job must be idempotent. No `job_lease` table exists in v1 (D26).

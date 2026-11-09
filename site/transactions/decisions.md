# Transactions & Consistency: Decision Thinking

Each entry says what was chosen, what was rejected, and why. *(documented)* means the reason is written in the decision log, PRD or ADR. *(inferred)* means it is a reading of the design.

## C1. Audit success row in the business transaction *(documented, D2)*

**Chosen:** the audit row is inserted in the same transaction as the change.
**Rejected:** a message queue or an async write.
**Why:** fail-closed. A change without its audit row must not exist. The audit table is in the same database, so no new system is needed. An async write would lose events when it fails.

## C2. Rejected attempts in a separate transaction *(documented, D3)*

**Chosen:** `DENIED` rows (track B) use their own transaction.
**Why:** the business transaction is rolled back on a refusal, and that would erase the audit row with it.

## C3. No distributed transaction *(documented, D4)*

**Chosen:** no two-phase commit across MariaDB and MinIO, no replication management, no off-site copies.
**Why:** platform concern, out of scope. The system uses ordering and compensation instead (C6 to C8).

## C4. `READ COMMITTED` *(documented, D7)*

**Chosen:** `READ COMMITTED`, `NOW(6)` read inside the transaction, deadline read without a shared lock.
**Why:** the race between a write and a deadline change is rare and acceptable. A lock on the deadline row would make every write wait on one row.
**Detail:** the level is set on every `BeginTx`, so it does not depend on server defaults or on variable names that differ between MariaDB versions (OI-04).

## C5. One Idea-row lock for team and slide writes *(documented, D8)*

**Chosen:** `SELECT ... FOR UPDATE` on the Idea row.
**Why:** the capacity rule spans two tables, which the database cannot enforce. For slides, two concurrent replacements must not read the same old key, or one object is orphaned.
**Rejected:** a lock table, optimistic retry. *(inferred)* One row lock is the least code and the fewest places to get the order wrong.

## C6. Classify by where the failure happened *(documented, D44, ADR G7)*

**Chosen:** failure before `Commit`, or SQLState class `23`/`40`, is a known rollback. **Every other commit failure is `Unknown`.**
**Why:** conservative. Mistaking a rollback for unknown delays a clean-up. Mistaking an unknown for a rollback deletes a file an Idea may now reference.

## C7. Keep the new object when the outcome is unknown *(documented, D36)*

**Chosen:** do not delete; record `UPLOAD_ABORTED` with a grace period (10 minutes, OI-19).
**Why:** "A lost `COMMIT` acknowledgement must never leave an Idea pointing at a deleted file." The job deletes only if no `idea` row references the key.

## C8. Record replaced and deleted keys in the same transaction *(documented, PRD §6.8, D25)*

**Chosen:** `REPLACED` and `DELETED` rows are inserted in the change transaction. The object is deleted after commit.
**Why:** the row exists exactly when the change committed. Deleting an Idea succeeds even if MinIO is down.

## C9. Validate before upload; upload before the transaction *(documented, D37)*

**Chosen:** all text and file checks finish before MinIO is called. The upload happens before `BeginTx`. The old-object delete happens after commit.
**Why:** a rejected request creates no object, and no MinIO call runs while a lock is held. A slow storage call would otherwise hold the Idea lock.

## C10. `InTx` ignores client cancellation *(documented, ADR G8)*

**Chosen:** `context.WithoutCancel` plus a 10 s timeout.
**Why:** `database/sql` rolls back when its context ends. A client disconnect after the upload must not become a rollback. The timeout keeps a stuck database from holding locks forever.

## C11. Explicit `Rollback`, and a recovered panic *(documented in code comments)*

**Chosen:** `InTx` always calls `Rollback` after a failure and rolls back before re-panicking.
**Why:** MariaDB rolls back only the statement on a lock-wait timeout (1205), so the locks would stay. A panic must not leave a transaction on a pooled connection.

## C12. Lock contention is 503, not 409 or 500 *(documented, ADR G7)*

**Chosen:** 1205 and 1213 map to `503 SERVICE_UNAVAILABLE` with a metric.
**Why:** it is a temporary overload, and the client may retry. *(inferred)* A 409 would suggest the client's data is wrong.

## C13. No job lease table *(documented, D26)*

**Chosen:** background jobs are idempotent and run on every replica; `SKIP LOCKED` for the orphan job.
**Why:** a lease table adds a table and a failure mode for little gain at this scale.

## Known limits

| Limit | Source |
|---|---|
| A crash between upload and transaction leaves an untracked orphan | PRD §10 |
| The clean-up job is not built; `pending_file_deletion` rows stay until it exists | PRD §10 |
| A write racing a deadline change may land on either side | D7 |
| Pool sizes and `wait_timeout` are set with the platform | OI-04 |

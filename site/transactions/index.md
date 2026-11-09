# Transactions & Consistency: Concepts

## What this topic covers

Every write in the system must leave the data in a state that is **true**: a change never exists without its audit row, a team never has more than four people, and an Idea never points at a file that is gone. Two things threaten that:

1. Several requests change the same rows at the same time.
2. A write touches **two systems** (MariaDB and MinIO) that cannot commit together.

This topic explains the rules that handle both.

## One transaction per write

Every write runs inside one database transaction, opened by one helper, `db.InTx`. Inside it the service:

1. locks what it needs,
2. re-checks role and state,
3. changes the rows,
4. writes the audit row (track A),
5. commits.

If any step fails, **all of it** is undone. The audit row and the business change succeed or fail together.

## Isolation and locking

| Rule | Value |
|---|---|
| Isolation level | `READ COMMITTED`, set on every `BeginTx` |
| Lock for team operations | The Idea row, `SELECT ... FOR UPDATE` |
| Lock for the deadline check | None (accepted race) |
| Clock | `NOW(6)` of the database, read inside the transaction |
| Lock wait timeout (1205) / deadlock (1213) | `503 SERVICE_UNAVAILABLE` |

## Three outcomes

After a write, the code knows one of three things:

| Outcome | Meaning |
|---|---|
| `Committed` | `COMMIT` succeeded. |
| `RolledBack` | The change is known **not** to be applied. |
| `Unknown` | `COMMIT` failed in a way that may still have been applied. |

`Unknown` is rare, but it is the case that decides how files are handled.

## Database and file storage

MariaDB and MinIO cannot commit together, and distributed transactions are out of scope (D4). The system keeps them consistent with three rules:

- **Validate before upload.** A rejected request never creates a file.
- **Upload before the transaction; delete after it.** No MinIO call happens while a lock is held.
- **Record every key that might become an orphan** in `pending_file_deletion`. Replaced and deleted keys are written in the same transaction as the change.

| Outcome | What happens to the **new** file |
|---|---|
| `Committed` | Kept. The old file is deleted after commit (best effort). |
| `RolledBack` | Deleted at once. If that fails, its key goes to `pending_file_deletion` as `UPLOAD_ABORTED`. |
| `Unknown` | **Not deleted.** Its key goes to `pending_file_deletion` with a 10 minute grace period. The user gets `503`. |

Why keep the file when unsure: if `COMMIT` succeeded but the answer was lost, deleting the file would leave the Idea pointing at nothing. Keeping an orphan costs a little disk; losing a referenced file loses a submission.

## Accepted trade-offs

- A write that races a **deadline change** may land on either side. The deadline row is read without a shared lock (D7).
- A crash between the upload and the transaction leaves an orphan that no table records (PRD §10).
- The clean-up job that drains `pending_file_deletion` is the next phase. The rows are written in v1.

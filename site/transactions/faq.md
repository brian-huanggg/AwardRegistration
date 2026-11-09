# Transactions & Consistency: Technical FAQ

## Concepts

### Which isolation level is used?
`READ COMMITTED`. It is set on every `BeginTx`, not left to the server default.

### Why not `SERIALIZABLE` or a lock everywhere?
The rules that need exclusion use one explicit lock, the Idea row. Everything else is protected by constraints (primary and unique keys, `CHECK`). That is less locking and fewer waits.

### Why is there no transaction around "upload + database"?
MariaDB and MinIO cannot commit together. The system orders the steps (upload, transaction, delete) and records keys that might be orphaned, so the data stays consistent without a distributed transaction.

### What is `Unknown`?
`COMMIT` failed in a way that may still have been applied, for example a lost connection after the server accepted it. The code cannot tell, so it behaves as if both were possible.

## Behaviour

### What does the user see on `Unknown`?
`503 SERVICE_UNAVAILABLE`: "The result of the request is unknown; check your Idea before retrying." The user should look at the Idea first.

### Is a new file ever deleted when the outcome is unknown?
No. Its key is recorded as `UPLOAD_ABORTED` with a 10 minute grace period. The job removes it later only if no Idea references it.

### What happens on a deadlock?
The statement fails, `InTx` rolls back and the user gets `503`. A retry normally works. All team writes lock the Idea row first, so two team operations cannot deadlock each other.

### What if MinIO is down when an Idea is deleted?
The delete still succeeds. The key is already in `pending_file_deletion` as `DELETED`. The object is removed later.

### What if the client disconnects mid-request?
The transaction still finishes. `InTx` does not use the request's cancellation. The post-commit steps also ignore it.

### Can a write exist without its audit row?
No. The audit row is in the same transaction. If the audit insert fails, the whole write rolls back.

### Can a refusal be lost from the audit log?
Rarely. Track B is best effort: if its own transaction fails, a metric is incremented and the response is still sent.

## Implementation

### How do I add a new write?
Use `db.InTx`. Lock first, re-check with `AssertOpenInTx`, change, `Audit.Record`, return. Handle `db.Unknown`. See the checklist in the Implementation page.

### Why does `AssertOpenInTx` take a `*sql.Tx`?
So it cannot be called outside a write transaction. The compiler enforces it.

### Why check state twice?
`Precheck` keeps the error order of the PRD and rejects early. `AssertOpenInTx` is the real check, made inside the transaction.

### Where do I put a MinIO call?
Never inside the `InTx` function. Upload before it, delete after it.

### How do I recognise a duplicate key?
Use `db.DuplicateKey(err)`. It returns the key name, for example `uq_idea_leader_name`. Decide the error code from the name.

### Which tests prove the outcome rules?
`TestInTxOutcomes` for the classification, and `TestInTxLostCommitAgainstMariaDB` against a real MariaDB for the lost-acknowledgement case.

# Transactions & Consistency: Implementation

Snippets are shortened from the Go code. Names are real.

## `db.InTx`

```go
type Outcome int
const (Committed Outcome = iota; RolledBack; Unknown)

const TxTimeout = 10 * time.Second

func InTx(ctx context.Context, db *sql.DB,
    fn func(ctx context.Context, tx *sql.Tx) error) (Outcome, error) {

    txCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), txTimeout)
    defer cancel()

    tx, err := db.BeginTx(txCtx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
    if err != nil { return RolledBack, classify(err) }

    if err := runFn(txCtx, tx, fn); err != nil {
        _ = tx.Rollback()                    // always explicit
        return RolledBack, classify(err)
    }
    if err := tx.Commit(); err != nil {
        switch sqlStateClass(err) {
        case "23", "40": return RolledBack, classify(err)
        default:         return Unknown, classify(err)
        }
    }
    return Committed, nil
}
```

`runFn` recovers a panic, rolls back, and panics again. A panic therefore never leaves a transaction and its locks on a pooled connection.

`classify` turns 1205 and 1213 into a `*web.Error` with code `SERVICE_UNAVAILABLE` and keeps the driver error in `Cause`. Every other error passes through unchanged.

## Error helpers (`shared/db/mysqlerr.go`)

| Helper | Detects |
|---|---|
| `IsContention` | 1205 lock wait, 1213 deadlock |
| `DuplicateKey` | 1062, and returns the violated key name (for example `uq_idea_leader_name`) |
| `IsCheckViolation` | 4025 (MariaDB) or 3819 (MySQL) |
| `sqlStateClass` | first two characters of the SQLState |

Callers decide meaning from the **key name**, not the message text. The table is the last line of defence; the service checks first so the user gets a clear code.

## Pattern: a plain write (team approve, shortened)

```go
out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
    // 1 lock
    // 2 re-check role and state
    if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
        return markDenied(err, audit.JoinRequestApprove, id, reasonState)
    }
    // 3 change rows
    // 4 audit, same tx
    return s.Audit.Record(ctx, tx, audit.Event{ /* ... */ })
})
if out == db.Unknown { /* 503: check before retrying */ }
return err
```

`markDenied` attaches the audit action, reason and target to the error. The central adapter then writes the `DENIED` row in its **own** transaction (track B), because this one is rolled back.

## Pattern: replace slides (shortened)

```go
// before: validate file; Objects.Put(newKey)         ← upload first
out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
    leader, old, err := store.LockSlides(ctx, tx, id)  // FOR UPDATE, reads old key
    // not found → 404 · not leader/admin → 403 · AssertOpenInTx → 403
    store.SwitchSlides(ctx, tx, id, next)              // version = version + 1
    pendingID, err = s.Replaced.RecordReplaced(ctx, tx, old.ObjectKey)
    return s.Audit.Record(ctx, tx, /* SLIDES_REPLACE */)
})
if out == db.Committed {
    s.Replaced.DeleteAfterCommit(ctx, pendingID, oldKey)   // best effort
    return got, nil
}
s.Aborter.AbortUpload(ctx, next.ObjectKey, out)            // RolledBack or Unknown
if out == db.Unknown { return 503 }
return err
```

Idea delete follows the same shape with `RecordDeleted` and no new object.

## `storage.PendingDeletion`

| Method | When | Where |
|---|---|---|
| `RecordInTx(tx, key, REPLACED/DELETED)` | in the change transaction | same tx; any other reason is a programming error |
| `DeleteAfterCommit(id, key)` | after `Committed` | delete object, then the row; failure is logged and the row stays |
| `AbortUpload(key, outcome)` | after a non-commit | `RolledBack`: delete now, else record due at once. `Unknown`: record due after `UploadAbortedGrace` |
| `RecordUploadAborted(key, outcome)` | inside `AbortUpload` | own transaction, own 5 s timeout, `context.WithoutCancel` |

```go
const UploadAbortedGrace = 10 * time.Minute  // Proposed, OI-19

INSERT INTO pending_file_deletion (object_key, reason, next_attempt_at)
VALUES (?, ?, NOW(6) + INTERVAL ? MICROSECOND)
```

The grace period is longer than the maximum lock wait plus transaction time, so the job never races a commit the server is still finishing.

## Clean-up rules (next phase)

| `reason` | Job action |
|---|---|
| `REPLACED` | Delete the object. Nobody references the old key. |
| `DELETED` | Delete the object. |
| `UPLOAD_ABORTED` | Delete only if `next_attempt_at` passed **and** no `idea.slides_object_key` equals the key. If an Idea references it, the commit did succeed: drop the row, keep the object. |

## Tests that pin behaviour

| Test | Pins |
|---|---|
| `TestInTxOutcomes` | which failure gives which outcome |
| `TestInTxUsesReadCommitted` | isolation level set on every transaction |
| `TestInTxLockWaitRollsBackWholeTx` | explicit rollback after 1205 |
| `TestInTxContentionMapsTo503` | 1205 and 1213 give 503, others unchanged |
| `TestInTxSurvivesCallerCancel` | client disconnect does not roll back |
| `TestInTxHasOwnTimeout` / `TestInTxTimeoutExpires` | 10 s bound |
| `TestInTxPanicRollsBackAndRepanics` | no open transaction after a panic |
| `TestInTxLostCommitAgainstMariaDB` | real lost `COMMIT` acknowledgement is `Unknown` |
| `TestInTxConnectionCutBeforeCommit` | connection cut before commit |

## Checklist for a new write

- [ ] Does it run inside `db.InTx`? No manual `BeginTx`.
- [ ] Does it lock the Idea row first if it touches a team or slides?
- [ ] Does it call `AssertOpenInTx` inside the transaction, not only `Precheck`?
- [ ] Is the audit `Record` in the same transaction?
- [ ] Does it do any MinIO call **inside** the function? It must not.
- [ ] If a file key can be orphaned, is it recorded in `pending_file_deletion`?
- [ ] Does the caller handle `db.Unknown` with a `503`?

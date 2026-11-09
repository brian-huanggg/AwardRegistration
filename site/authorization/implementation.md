# Authorization & Uniform 404: Implementation

Code lives in `backend/internal/idea/` (service and handlers) and `backend/internal/idea/internal/store/` (SQL).

## 1. The single visibility predicate

`store/store.go`:

```go
const VisibleWhere = `(? = 1 OR i.leader_id = ?
	OR EXISTS (SELECT 1 FROM idea_member m WHERE m.idea_id = i.id
	           AND m.user_id = ? AND m.status = 'ACCEPTED'))`

func VisibleArgs(viewer uuid.UUID, isAdmin bool) []any  // [0|1, viewer, viewer]
```

`PENDING` is excluded by `m.status = 'ACCEPTED'`, which is why an applicant sees nothing.

## 2. One query, one error

```go
func GetVisible(ctx, q, id, viewer, isAdmin) (Idea, error) {
	err := q.QueryRowContext(ctx, selectIdea+` WHERE i.id = ? AND `+VisibleWhere, ...).Scan(...)
	if errors.Is(err, sql.ErrNoRows) {
		return Idea{}, ErrNotFound   // missing and hidden are the same value
	}
	...
}
```

The service maps `ErrNotFound` to `web.NewError(web.CodeNotFound, "")`. The detail string is empty on purpose, so the body cannot differ.

## 3. Check order in a handler (delete as the example)

`delete.go`:

```go
got, err := s.Get(ctx, id, caller)                          // 404: visibility
if err != nil { return markDenied(err, audit.IdeaDelete, id, auditReasonHidden) }

if !caller.IsAdmin() && got.LeaderID != caller.UserID {     // 403: role
	return markDenied(web.NewError(web.CodeForbidden, deleteForbidden), ..., auditReasonRole)
}
if err := markDenied(s.Gate.Precheck(ctx, caller), ..., auditReasonState); err != nil {
	return err                                              // 403: state
}

db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
	leader, text, err := store.LockText(ctx, tx, id)           // lock the row
	// ... re-check existence, role and state INSIDE the transaction
	s.Gate.AssertOpenInTx(ctx, tx, caller)
	// ... delete, write the DELETED and IDEA_DELETE audit rows
})
```

The pre-check gives a fast refusal. The in-transaction check is authoritative.

## 4. markDenied and the audit reason

`markDenied(err, action, id, reason)` wraps an error with an `AuditReason`. It does not write anything. The central adapter in `internal/server` sees the reason and calls `RecordRejected`. Reasons:

```go
const auditReasonHidden = "NOT_FOUND"  // one value for missing AND hidden (handler.go)
// ROLE, STATE, CSRF for the others
```

Because both cases share one reason, an administrator reading the log cannot separate them either. This is deliberate; see [Decision Thinking](./decisions) Z3.

## 5. Admin views are audited on success

`View` for an Admin runs the read and `Audit.Record(IDEA_VIEW)` in the **same transaction**. If the audit insert fails, the read fails. Ordinary users' views are not audited.

## 6. The permission matrix as data

`testdata/permission-matrix.json` has one row per operation:

```json
{
  "id": "set-deadline", "operation": "setDeadline",
  "method": "PUT", "path": "/api/v1/admin/deadline",
  "body": { "deadline": "2090-01-01T00:00:00Z" },
  "expect": { "visitor": {"status": 401}, "user": {"status": 403}, "admin": {"status": 200} }
}
```

A driver test iterates every row and every role. Adding an endpoint without adding a row fails review (PR rule: every touched matrix cell gets a test).

## 7. Tests that pin the uniform 404

From `download_integration_test.go` and `handler_integration_test.go`:

- Request a **hidden** Idea and a **missing** Idea.
- Compare the bodies: same `code`, same `detail`, same length; only `requestId` differs.
- Count audit rows: `outcome = 'DENIED' AND status = 404 AND error_code = 'NOT_FOUND'` equals 2.
- Pending applicant, non-member and leader-of-another-Idea are each tested against `GetVisible` and must get `NOT_FOUND`.

## 8. Adding a new endpoint: checklist

1. Read the resource only through `GetVisible` / `ListVisible`.
2. Order checks: visibility → role → state pre-check → validate body.
3. Re-check role and state in the transaction after the row lock.
4. Attach `AuditReason` to every refusal; never call `RecordRejected` directly.
5. Add the row to `permission-matrix.json` and a hidden-vs-missing test.
6. If a new error code appears, update the catalogue, `error-codes.json`, `docs/API/error-codes.md` and both dictionaries.

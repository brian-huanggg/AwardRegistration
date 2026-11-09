# Team & Join Request: Implementation

Code lives in `backend/internal/team/`. SQL is in `internal/store/store.go`. All snippets are shortened.

## The table

```sql
CREATE TABLE idea_member (
    idea_id     UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    status      VARCHAR(8)  NOT NULL DEFAULT 'PENDING',
    accepted_at DATETIME(6) NULL,
    created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (idea_id, user_id),
    CONSTRAINT fk_idea_member_idea FOREIGN KEY (idea_id) REFERENCES idea (id) ON DELETE CASCADE,
    CONSTRAINT ck_idea_member_status CHECK (status IN ('PENDING', 'ACCEPTED')),
    CONSTRAINT ck_idea_member_accepted CHECK ((status = 'ACCEPTED') = (accepted_at IS NOT NULL)),
    KEY ix_idea_member_user (user_id, status)
);
```

- The primary key is the duplicate guard.
- `ON DELETE CASCADE` removes requests and Members when the Idea is deleted.
- The second `CHECK` ties `accepted_at` to `ACCEPTED`, so the two cannot disagree.
- Times come from the database clock.

## Apply

The handler runs role and state checks **before** it reads the body, then validates and calls the service:

```go
func (s *Service) AuthorizeApply(ctx context.Context, caller security.Principal) error {
	if caller.IsAdmin() {
		return markDenied(web.NewError(web.CodeForbidden, "Admins cannot apply to join an Idea."), auditReasonRole)
	}
	return markDenied(s.Gate.Precheck(ctx, caller), auditReasonState)
}
```

Inside `Apply`, the order is: validate body (422) → find the Idea (404) → own Idea (422) → transaction. The insert's errors map to codes:

```go
at, err = store.InsertPending(ctx, tx, got.ID, caller.UserID)
switch {
case err == nil:
case db.IsForeignKeyViolation(err):
	// The Idea was deleted after the lookup.
	return markDenied(web.NewError(web.CodeNotFound, ""), auditReasonHidden)
default:
	if _, dup := db.DuplicateKey(err); !dup { return err }
	return s.existing(ctx, tx, got.ID, caller.UserID, err) // 409
}
```

Two races are closed by the database, not by code: two parallel applies hit the primary key, and an Idea deleted after the lookup hits the foreign key (answered as the uniform 404).

## Validation

`leaderEmpNo` must be exactly six ASCII digits, with nothing trimmed. `ideaName` goes through the shared name clean-up (trim, collapse whitespace) and a 75-character limit. All field problems are reported together, in form order.

## Approve

```go
leader, err := s.Ideas.LockLeader(ctx, tx, id)   // lock the Idea row
// leader != caller → 403 ROLE; AssertOpenInTx → 403 STATE
row, err := store.LockByEmpNo(ctx, tx, id, empNo) // no row → 404
if row.Status != store.Pending { return 409 JOIN_REQUEST_NOT_PENDING }
n, _ := store.CountAccepted(ctx, tx, id)
if n >= maxMembers { return 409 TEAM_FULL }       // maxMembers = 3
store.Accept(ctx, tx, id, row.UserID)
// audit JOIN_REQUEST_APPROVE with before/after status
```

The Leader and the state are checked a second time here, after the lock, because they could have changed since the pre-check.

## Remove and Leave

Remove deletes either a pending request or an approved Member. The action name in the audit row depends on what was deleted:

```go
action := audit.MemberRemove
if row.Status == store.Pending { action = audit.JoinRequestReject }
```

Leave does the same with `MEMBER_LEAVE` and `JOIN_REQUEST_WITHDRAW`. The Leader's own employee ID matches no `idea_member` row, so Remove answers 404 for it.

## Audit payloads

| Action | `before` | `after` |
|---|---|---|
| `JOIN_REQUEST_CREATE` | none | `{status: PENDING}` |
| `JOIN_REQUEST_APPROVE` | `{empNo, status: PENDING}` | `{empNo, status: ACCEPTED}` |
| `JOIN_REQUEST_REJECT`, `MEMBER_REMOVE` | `{empNo, status}` | none |
| `JOIN_REQUEST_WITHDRAW`, `MEMBER_LEAVE` | `{status}` | none |

## Tests that pin the behaviour

| Test | What it proves |
|---|---|
| `TestApplyConcurrent` | Eight parallel applies: one succeeds, seven get `JOIN_REQUEST_EXISTS`, one audit row |
| `TestApproveConcurrentNeverExceedsThree` | Eight parallel approvals: exactly three succeed, five get `TEAM_FULL`, three audit rows |
| `TestApplyIdeaDeletedAfterLookup` | Idea deleted mid-request answers the uniform 404 |
| `TestApplyRoleAndState`, `TestApproveRoleAndState`, `TestRemoveState`, `TestLeaveState` | Role and state refusals and their audit rows |
| `TestValidate` | Field problems and their order |

The integration tests use a real MariaDB (testcontainers).

## Checklist for a new team action

1. Resolve the Idea through the `idea` module, never by reading its table.
2. Take the Idea row lock before reading or changing any `idea_member` row.
3. Re-check role and state after the lock.
4. Return a `*web.Error` with an audit action and reason; do not write the DENIED row yourself.
5. Write the success audit row in the same transaction.
6. Add tests for the permission-matrix cells you touched.

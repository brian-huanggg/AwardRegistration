# Team & Join Request: Architecture

## Module position

The `team` module owns the `idea_member` table. It does **not** read the `idea` table itself. It asks the `idea` module through a small interface.

```text
team ──► idea     FindByLeader, Get, LockLeader
team ──► audit    Record (track A, in the transaction)
team ──► deadline SubmissionGate (Precheck, AssertOpenInTx)
```

The import-graph test and `depguard` enforce this direction. `team` never imports `idea`'s internals.

## One lock serializes the team

The rule "approved Members ≤ 3" spans two tables (`idea` and `idea_member`). The database cannot enforce it, so the application does, under **one lock: the Idea row, `SELECT ... FOR UPDATE`** (D8).

```text
Leader A approves X ─┐
Leader A approves Y ─┼─► each lock the Idea row first ─► one at a time
Member Z leaves     ─┘      count ACCEPTED ─► change ─► audit ─► commit
```

Approve, reject, remove and leave all take this lock. Concurrent approvals therefore see each other's result, and the count never passes three. **Apply does not take the lock**, because it does not touch capacity.

## Apply: the primary key is the guard

Apply inserts one `PENDING` row. The primary key `(idea_id, user_id)` allows one row per person per Idea, so the database itself rejects a second request or a request from an existing Member. The service reads the existing row afterwards to pick the right code: `ALREADY_MEMBER` for `ACCEPTED`, `JOIN_REQUEST_EXISTS` for `PENDING`.

## Pipeline per action

| Step | Apply | Approve / Remove | Leave |
|---|---|---|---|
| Before the body | Admin → 403, state pre-check | none (no body) | none |
| Resolve target | find Idea by Leader + name | `Get` Idea as caller (404 if hidden) | does the caller have a row? (404 if not) |
| Role | not the Leader (422) | caller is the Leader (403) | none |
| State pre-check | yes | yes | yes |
| In the transaction | `AssertOpenInTx`, insert | lock Idea, re-check Leader and state, lock row | lock Idea, re-check state, lock own row |
| Audit (track A) | `JOIN_REQUEST_CREATE` | `JOIN_REQUEST_APPROVE`, `_REJECT`, `MEMBER_REMOVE` | `JOIN_REQUEST_WITHDRAW`, `MEMBER_LEAVE` |

## Check order matters for secrecy

Leave checks "do I have a row?" **before** the deadline state. If it checked the state first, a closed window would answer `403 SUBMISSION_CLOSED` to a stranger and so confirm the Idea exists. Checking the row first gives the stranger a uniform 404.

Approve and Remove use `Ideas.Get`, which applies the visibility rule. A caller who cannot see the Idea gets 404. A Member or Admin who can see it but is not the Leader gets 403.

## Refusals and audit

Role, state and not-found refusals return a `*web.Error` carrying an audit action, reason and target. The central error adapter writes the `DENIED` audit row in its own transaction (track B). Success writes its audit row inside the same transaction as the change (track A), so a write never exists without its audit row.

| Reason | Meaning |
|---|---|
| `ROLE` | Wrong role for the action |
| `STATE` | Outside the `OPEN` window |
| `NOT_FOUND` | Missing or hidden; one reason for both |

## Unknown commit outcome

If the `COMMIT` fails with an unknown result, the handler answers `503` and tells the user to check before retrying. No storage is involved here, so nothing needs cleanup.

# Authorization & Uniform 404: Technical FAQ

## Concepts

### Why 404 and not 403 for an Idea I cannot see?
`403` would confirm the Idea exists. See [Decision Thinking](./decisions) Z1.

### When is 403 used then?
When the caller can already see the resource but may not do the action: a Member edits, a non-Leader deletes, a User queries the Audit Log, an Admin creates an Idea.

### What is the check order?
401 → CSRF 403 → 404 → role 403 → state 403 → validation (400 / 422). A request that fails two checks reports the earlier one.

### Is hiding the button enough?
No. Every permission is enforced on the server.

## Uniform 404

### What must be identical between "missing" and "hidden"?
Body (except `requestId`), headers, response time and the audit row.

### Why audit a request for a missing Idea?
So the two paths do the same database work. If only "hidden" wrote an audit row, it would be slower, and timing would reveal which is which.

### Does the audit log reveal which Ideas exist?
No. Both cases store the reason `NOT_FOUND`.

### What about `GET /ideas/not-a-uuid`?
Also 404 and also audited as `DENIED`, with `targetId` null and the route template recorded (D56).

### What about a completely unknown path?
With a session: 404 `NOT_FOUND`, not audited. Without a session: 401.

### Why does a pending applicant get 404 on the Idea?
A `PENDING` request grants no access. The applicant sees their request in "my join requests" with minimal Idea information.

## Roles and matrix

### Where is the full matrix?
PRD §7 and `testdata/permission-matrix.json`.

### Can an Admin edit an Idea's text?
No (`403`). An Admin can view, replace slides, delete and read the Audit Log. An Admin cannot create Ideas.

### Why does `*` appear in the matrix?
The action is allowed only while submissions are `OPEN`. Otherwise `403` with `SUBMISSION_NOT_OPEN` or `SUBMISSION_CLOSED`. Admins are exempt from the state check on delete and slide replace.

### Are Admin views audited?
Yes. An Admin viewing a single Idea writes `IDEA_VIEW` in the same transaction as the read. Ordinary views are not audited.

## Implementation

### How do I make sure a new endpoint is safe?
Read only through `GetVisible` / `ListVisible`, follow the check order, re-check in the transaction, attach an `AuditReason` to every refusal and add a matrix row. The full checklist is in [Implementation](./implementation) §8.

### Where do `DENIED` rows come from?
The central error adapter writes them (track B, its own transaction) when an error carries an `AuditReason`. Handlers never call `RecordRejected`.

### What if the DENIED write fails?
The response is unchanged. A metric is incremented. A refusal never turns into a 500 because of audit.

### Why not return a different error for a database failure during the check?
A failed visibility query is an internal error: `500` with `INTERNAL_ERROR` and `requestId` only. It never falls back to "allow".

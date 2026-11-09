# Team & Join Request: Technical FAQ

## Concepts

### How does someone join a team?
They send a join request with the Leader's employee ID and the Idea name. The Leader approves or rejects it. There are no invitations.

### How big can a team be?
One Leader and up to three approved Members, so four people.

### Does a pending request use a seat?
No. Only approved Members count. Capacity is checked when the Leader approves.

### Can I apply to a full team?
Yes. The Leader cannot approve you until a seat frees up (`409 TEAM_FULL`).

### Can I apply again after a reject?
Yes, at once. The rejected row was deleted.

### Can a Leader leave?
No. A Leader ends the team by deleting the Idea.

### Can an Admin join a team?
No. `POST /join-requests` answers `403 FORBIDDEN`.

## Behaviour

### What can an applicant see?
Their own pending requests through `GET /join-requests`. They cannot open the Idea; `GET /ideas/{id}` returns 404.

### What if I apply to an Idea that does not exist?
`404 NOT_FOUND`, the same as any missing resource.

### Which error do I get for a request that is already pending?
`409 JOIN_REQUEST_EXISTS`. If you are already approved, `409 ALREADY_MEMBER`.

### Why `422` for my own Idea but `409` for a duplicate?
`CANNOT_JOIN_OWN_IDEA` is a rule about the input pair. The 409 codes describe a conflict with existing state.

### Do these actions work after the deadline?
No. All of them are allowed only in `OPEN`. Before the window: `SUBMISSION_NOT_OPEN`. After: `SUBMISSION_CLOSED`.

## Implementation

### How is "at most three Members" kept under concurrency?
Every approval locks the Idea row first, counts approved rows, then changes. Two approvals cannot count at the same time.

### How are two simultaneous applies handled?
The primary key `(idea_id, user_id)` lets one insert win. The others get `409 JOIN_REQUEST_EXISTS`. Only one audit row is written.

### What if the Idea is deleted while I apply?
The foreign key fails, and the service answers the uniform 404.

### Why does the audit log have different actions for reject and remove?
The action follows what was deleted: a `PENDING` row is a reject or withdraw, an `ACCEPTED` row is a remove or leave.

### Why is the Leader checked twice on approve?
Once before the transaction for a cheap early answer, and once after the Idea row lock, where the answer is reliable.

### Where does a DENIED audit row for a team action come from?
The service returns an error carrying the audit action and reason. The central error adapter writes the row in its own transaction. Service code never writes it directly.

### What if the commit result is unknown?
The API answers `503` and asks the user to check their requests or the Idea before retrying.

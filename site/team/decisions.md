# Team & Join Request: Decision Thinking

This page records **why** the team model looks the way it does. Entries marked *(documented)* follow reasons written in the design documents (decision-log numbers in brackets). Entries marked *(inferred)* are the likely reason read from the design; confirm them with the owner before quoting them as fact.

## A framework for team decisions

1. **Who has the final say?** The Leader decides; the system only enforces limits.
2. **Which rule spans two tables?** The database cannot guard it; a lock and a test must.
3. **What is the cheapest model that works?** One table, few states.
4. **What does a refusal reveal?** See [Authorization & Uniform 404](../authorization/).

## Decisions

### T1. Join requests replace invitations *(documented, D18)*

| | |
|---|---|
| Options | Leader invites by employee ID; applicant requests by Leader ID + Idea name |
| Choice | Applicant requests |
| Why | Less code. Invitations were our own design, not an organizer requirement. It also removes employee-ID lookup of other people. |
| Cost | The Leader can be flooded; the apply call leaks Idea existence (T7) |

### T2. One row is both request and membership *(documented, D19)*

| | |
|---|---|
| Options | Separate request and member tables; one table with a status |
| Choice | `idea_member` with `PENDING` / `ACCEPTED` |
| Why | Approve is one `UPDATE`. The primary key gives the duplicate guard for free. |
| Cost | The "Applicant has no access" rule must live in the visibility predicate, not in the table shape |

### T3. Exits delete the row *(documented, D19)*

| | |
|---|---|
| Choice | Reject, withdraw, remove and leave delete the row; history is in the Audit Log only |
| Why | No `REJECTED` or `LEFT` states to filter everywhere. Re-applying needs no special case. |
| Cost | The current state cannot answer "who was ever a Member". Only the audit log can. |

### T4. Idea row lock serializes team changes *(documented, D8)*

| | |
|---|---|
| Question | How to keep approved Members ≤ 3 under concurrency? |
| Choice | `SELECT ... FOR UPDATE` on the Idea row, then count, then change |
| Why | The rule spans two tables, so no `CHECK` can enforce it. The lock is cheap because a team has few writers. |
| Proof | `TestApproveConcurrentNeverExceedsThree` |

### T5. Capacity is checked at approval, not at apply *(documented, PRD §6.7)*

| | |
|---|---|
| Choice | Pending requests take no seat and are not capped; a full team still accepts applications |
| Why | A seat can free up, and the Leader may prefer a later applicant. Capacity is a Leader-side decision. |
| Cost | A person may wait on a team that never has room |

### T6. The primary key is the duplicate guard *(documented, D19)*

Apply does not read first and then insert. It inserts and lets the key reject. This closes the race between two parallel applies without a lock. The service reads the row only afterwards, to choose between the two 409 codes.

### T7. Existence leak accepted *(documented, D21)*

A logged-in user can confirm an Idea exists by applying with its Leader ID and name. The cost of closing it is a lookup design that does not fit "ask by name". The risk is low, so it is accepted and recorded in PRD §10.

### T8. Leave checks the row before the state *(documented, in code comment)*

A closed window must not tell a stranger the Idea exists. So Leave answers 404 if the caller has no row, and only then reports `SUBMISSION_CLOSED`.

### T9. Leader cannot leave; delete the Idea instead *(documented, PRD §6.7)*

A team without a Leader has no owner for approvals. Rather than define a hand-over, the design gives one way to end a team.

### T10. Role and state re-checked inside the transaction *(documented)*

The pre-check is a cheap early answer. The in-transaction check, after the lock, is the real one. Between them the Leader could have changed, the Idea could have been deleted, or the deadline could have passed.

## Known limits

- No cap on pending requests, so a Leader can be flooded. Accepted (D21).
- A pending-request badge for Leaders needs an endpoint that is not yet defined (PRD §10, item 6).
- Rate limit on applying (30 per minute per user) is listed in PRD §10; this page does not check that it is built.

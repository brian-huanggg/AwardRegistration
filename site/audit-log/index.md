# Audit Log: Concepts

An audit log is an append-only record of **who did what, to which resource, when, from where, and with what result**. It exists so that, after the fact, someone can answer questions such as:

- Who replaced the slides of this Idea, and what was the file?
- Which Admin viewed or downloaded which Idea last week?
- Did anyone try to reach a resource that was not theirs?
- When was this person removed from the team?

An audit log is not an application log. Application logs help engineers debug. An audit log is evidence for accountability: it must be complete for the events in scope, hard to alter, and readable by the right people.

## The five questions every event answers

| Question | Fields in this project |
|---|---|
| Who | `actor_emp_no`, `actor_role` (`USER`, `ADMIN`, `ANONYMOUS`, `SYSTEM`) |
| What | `action`, `target_type`, `target_id`, `before_json`, `after_json` |
| When | `occurred_at` (database time, UTC) |
| Where | `ip`, `user_agent`, `method`, `route`, `request_id` |
| Result | `outcome` (`SUCCESS`, `FAILED`, `DENIED`, `THROTTLED`), `status`, `error_code` |

## How detailed should it be?

Decide the granularity from the questions the audit must answer, not from what is possible to record.

| Tier | Examples | How it is written |
|---|---|---|
| Must record, same transaction | State changes, logins, downloads | The operation fails if the audit row cannot be written |
| Must record, best effort | Denied or throttled attempts | Separate transaction; failures are counted in a metric |
| Aggregate only | Ordinary list calls, health checks | Metrics, not audit rows |

Rules of thumb:

- Record the **business intent**, not the database mechanism. One user action is one event, not one event per table touched.
- Store only the **changed fields** in `before` and `after`.
- Store **references, not content**: file name, size and SHA-256 for a slide, never the file itself.
- Keep the event **self-explanatory**. The audit table has no foreign keys, because the user or Idea it describes may later be deleted.
- Use one `request_id` to tie together everything a single request caused.

### Related actions: one event or several?

Ask three questions:

1. Will an auditor search for this on its own? (`MEMBER_REMOVE` is, so it is its own action.)
2. Does it have its own actor or authorization decision?
3. Will the history exist **only** in the audit log? In this system membership rows are deleted, so the audit row is the only history.

If the answer to all three is no, fold it into the main event's `before` / `after`, or do not record it.

## Enterprise practice compared with this project

| Topic | Common enterprise practice | This project |
|---|---|---|
| Write consistency | Transactional outbox, or an agent that ships events | Same-transaction insert (track A) |
| Tamper evidence | Insert-only DB role, hash chain or signed checkpoints, WORM copy | Code scan forbids `UPDATE` / `DELETE`; `prev_hash` and `hash` columns are reserved but not written |
| Off-site copy | Streamed to a SIEM or write-once storage | Out of scope in v1 (PRD) |
| Reading | UI, SIEM, export, dedicated auditor role | `GET /admin/audit-logs` API for Admins (planned; no UI) |
| Alerting | Rules on anomalies | None in v1 |
| Retention | Set by regulation, partitioned, tiered | At least 365 days, no automatic purge |

See [Architecture](./architecture) for how the write path works, [Implementation](./implementation) for the code, and [Decision Thinking](./decisions) for why it is built this way.

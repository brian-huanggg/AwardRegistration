# Audit Log: Decision Thinking

This page records **why** the audit log looks the way it does. For each decision it lists the question, the options, the choice and what it costs. Use the same pattern when a new audit question comes up.

## A framework for audit decisions

Work through these in order.

1. **What question must the audit answer?** Write the question as a sentence ("who replaced these slides?"). If no one will ask it, do not record it.
2. **What is the failure cost of a missing record?** If a missing record means an unaccountable change, the write must be fail-closed. If it only means a lost note about a refused request, best effort is enough.
3. **Will the history exist anywhere else?** If the source row is deleted or overwritten, the audit row must carry enough to rebuild the state.
4. **Does recording it create a new risk?** Secrets, personal data, and enumeration channels all count.
5. **What does it cost per request?** Rows, indexes and JSON size. Measure before cutting detail.
6. **Can a test enforce it?** A rule that only lives in a document will drift. Prefer a rule a test or the compiler checks.

## Decisions

### D1. Same transaction for successful writes

| | |
|---|---|
| Question | Must a change and its record always agree? |
| Options | Same transaction; separate commit after the change; asynchronous queue or outbox |
| Choice | Same transaction (track A), fail-closed |
| Why | A separate commit leaves a window with a change and no record. A queue adds another system that can lose events. |
| Cost | Database availability limits write availability. A bug in the audit insert blocks the business operation. |

### D2. Separate, best-effort transaction for rejections

| | |
|---|---|
| Question | What happens when a denied attempt cannot be recorded? |
| Options | Fail the request; ignore silently; record best effort and count failures |
| Choice | Best effort in its own transaction, with a failure counter |
| Why | There is no business write to roll back with. Failing the request would turn an audit fault into a user-facing outage. |
| Cost | A denied attempt can be lost. The counter makes the loss visible but does not recover the row. |

### D3. Synchronous writes, no message queue

| | |
|---|---|
| Choice | Insert directly |
| Why | The scale is small, and a queue would break D1. |
| Revisit when | Events must reach other systems, or audit insert latency shows up in request latency. An outbox is the next step. |

### D4. Database time is the only clock

| | |
|---|---|
| Why | With several replicas, application clocks disagree and rows would not order consistently. |
| Cost | Tests that need a specific time must set it in the database. |

### D5. No foreign keys from the audit table

| | |
|---|---|
| Why | A row must outlive the user or Idea it describes. A foreign key would block or cascade the deletion. |
| Cost | The database will not catch a wrong `target_id`. The `insert` function checks the action, outcome, target type and actor role instead. |

### D6. Uniform 404 is audited as DENIED for both cases

| | |
|---|---|
| Question | Should "does not exist" and "not yours" be audited differently? |
| Options | Audit only "not yours"; audit both the same way; audit neither |
| Choice | Audit both as `DENIED` |
| Why | Auditing only one case makes the two cases take different time, and a caller could use that difference to tell them apart. |
| Cost | Real "not found" requests, such as a mistyped link, also appear as `DENIED`. The Admin query can still tell them apart by reason. |

### D7. Input errors are not audited, except registration

| | |
|---|---|
| Question | Should every 409 and 422 be a row? |
| Choice | No. Registration is the exception: `AUTH_REGISTER` `FAILED` is written for 400, 409 and 422. |
| Why | Most input errors are noise and would flood the table. A 409 on registration reveals that an employee ID or email is taken, so it is an enumeration channel worth watching. |
| Cost | Repeated bad input elsewhere is invisible to the audit log. It remains visible in metrics. |

### D8. Do not audit ordinary reads

| | |
|---|---|
| Choice | Record only an Admin viewing a single Idea, slide downloads and audit queries. |
| Why | Lists and polling would dominate the table and answer no audit question. |
| Cost | "Who looked at the list?" cannot be answered. Add specific events if that becomes a requirement. |

### D9. Store changed fields, references and no secrets

| | |
|---|---|
| Choice | `before` / `after` hold changed fields only. Files are recorded as name, size and SHA-256. Secret keys are redacted by name. Oversize JSON is replaced by a marker. |
| Why | It keeps rows small and keeps passwords, tokens, session IDs and file contents out of a table that many people can read. |
| Cost | The audit row cannot recreate an old file. The hash only proves which file it was. |

### D10. Append-only enforced by a test, not by the database

| | |
|---|---|
| Choice | A test scans the Go sources for SQL that updates or deletes `audit_log`. |
| Why | It is cheap and runs in CI. |
| Cost | It does not stop someone with direct database access. This is the weakest part of the design. |
| Stronger options | Insert-only database role for the application; hash chain; signed checkpoints; copy to write-once storage. |

### D11. Hash chain is deferred

| | |
|---|---|
| Choice | The `prev_hash` and `hash` columns exist, nothing writes them. |
| Why | A chain forces rows to be sealed in order. With several replicas that means either a lock on the chain head for every write, or a background sealer. Which one to use is open item OI-30 and is decided in an ADR when the work starts. |
| Cost | No tamper evidence in v1. |

### D12. Keyset pagination for reading

| | |
|---|---|
| Why | Offset paging slows down with depth and repeats or skips rows when new rows arrive. Keyset on `(occurred_at, id)` is stable and index-friendly. |
| Cost | No "jump to page N". |

### D13. No off-site copy and no automatic purge in v1

| | |
|---|---|
| Why | Out of scope for the first release. Retention is at least 365 days. |
| Cost | The log shares a failure domain with the application database. Recovery depends on database backups. |

## Choosing between options: a worked example

Suppose a new feature lets an Admin export all Ideas.

1. **Question:** "Who exported the data, when, and with what filter?"
2. **Failure cost:** High. The export leaves the system. Use track A, and write the row before streaming.
3. **Other history:** None. Record it in the audit log.
4. **New risk:** The row must not contain the data itself. Store the filter and the row count.
5. **Cost:** One row per export. Negligible.
6. **Test:** Assert that a failed audit insert prevents the export, and that a non-Admin gets `DENIED`.

## Known gaps and open items

| Gap | Where it is tracked |
|---|---|
| Hash chain sealing under concurrent writes | OI-30 |
| `GET /admin/audit-logs` has no handler yet | `docs/PR-plan.md` |
| No database-level append-only protection | Design option, not yet planned |
| No separate auditor role | Not planned |
| No off-site copy | Excluded by the PRD |

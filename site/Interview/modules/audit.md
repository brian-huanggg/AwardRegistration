# Audit Questions

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-11 |
| Prefix | `AUDT` |
| Status | **[Proposed]**: all questions are `Draft` until a second person reviews them with their rubric |

Questions about the audit log: the two write tracks, what counts as a rejected attempt, append-only enforcement, keyset paging and the optional hash chain. The file holds prompts, follow-ups and sources only. Rubrics live in the private answer key (see the [question template](../Rubric-template.md)).

Main sources: `docs/PRD.md` §6.11, §9 #5, §10, `docs/Architecture/Backend.md` §7, D2, D3, D13, D23, D48, D53.

| ID | Title | Level | Roles |
|---|---|---|---|
| `AUDT-L1-01` | What goes in an audit row | L1 | Junior / Senior |
| `AUDT-L1-02` | Audit log or application log | L1 | Junior / Senior |
| `AUDT-L2-01` | Two recording functions | L2 | Junior / Senior |
| `AUDT-L2-02` | Page through the log | L2 | Junior / Senior |
| `AUDT-L3-01` | Success row in the business transaction | L3 | Senior / Architect |
| `AUDT-L3-02` | Rejected attempts in their own transaction | L3 | Senior / Architect |
| `AUDT-L3-03` | What to audit and what not | L3 | Senior / Architect |
| `AUDT-L4-01` | The audit table is down | L4 | Senior / Architect |
| `AUDT-L4-02` | Append-only without database grants | L4 | Senior / Architect |
| `AUDT-L4-03` | Hash chain under concurrent writes | L4 | Senior / Architect |

---

### AUDT-L1-01 — What goes in an audit row

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §6.11 |

**Prompt**

An Admin replaces the slides of an Idea. Name the facts the audit row must hold, and name three things that must never appear in any audit row.

---

### AUDT-L1-02 — Audit log or application log

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §6.11, `docs/Architecture/Backend.md` §7 |

**Prompt**

The backend already writes logs with `log/slog`. A colleague asks why the project also needs an audit table. Explain the difference in purpose, in who reads each one, and in what happens when writing fails.

---

### AUDT-L2-01 — Two recording functions

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | `docs/Architecture/Backend.md` §7, D2, D3, D53 |

**Prompt**

The audit service has two functions: one for successful writes and one for rejected attempts. Write the signatures in Go. Explain how the type of the first function stops a developer from calling it outside a transaction, and who is allowed to call the second.

**Follow-ups**

1. The client disconnects while the rejected-attempt row is being written. What do you do so that the row is not lost?
2. How does a handler tell the central error adapter that a rejected attempt must be recorded?
3. The second function fails. What does the caller see, and what does the operator see?

---

### AUDT-L2-02 — Page through the log

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.11, `docs/Architecture/Backend.md` §2.3 (rule R3), §7 |

**Prompt**

`GET /admin/audit-logs` returns rows ordered by time and id, newest first, with a cursor. Write the SQL for the next page given a cursor that holds a timestamp and an id. Say what the response looks like and what the server answers to a broken cursor.

**Follow-ups**

1. Why does the project use keyset paging and not `OFFSET`?
2. Why is the condition written out as `occurred_at < ? OR (occurred_at = ? AND id < ?)` and not as a row comparison?
3. The cursor is a `base64url` string. Is it a security control? Explain.

---

### AUDT-L3-01 — Success row in the business transaction

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D2, PRD §6.11, `docs/Architecture/Sequence.md` §3 |

**Prompt**

A team proposes to publish audit events to a message queue after each successful write, so that the main request is faster and the audit store can be scaled separately. The project instead writes the audit row in the same database transaction as the business change. Argue for one design.

**Follow-ups**

1. Which failure leaves a business change without an audit record in the queue design, and how often would you tolerate it for a competition system?
2. What does "fail-closed" mean for a user whose audit write fails?
3. What is the performance cost of the in-transaction design, and what would make you revisit it?

---

### AUDT-L3-02 — Rejected attempts in their own transaction

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D3, D53, `docs/Architecture/Backend.md` §7, §2.1 |

**Prompt**

A user without permission calls an endpoint, and the request is rejected with 403. The business transaction rolls back. The project still records the attempt. Explain where and how this row is written, and why it cannot share the transaction of the business code.

**Follow-ups**

1. The error-handling code lives in `shared`, which may not import `audit`. How does it still trigger the write?
2. If this write fails, the request is still answered with 403. Why is "best effort" the right choice here and "fail-closed" the right choice for successful writes?
3. Which inputs would an attacker use to flood the audit table with rejected-attempt rows, and what limits this?

---

### AUDT-L3-03 — What to audit and what not

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 8 |
| Status | Draft |
| Sources | D48, D56, PRD §6.11 |

**Prompt**

A request answers 404 for an Idea that does not exist. Another request answers 404 for an Idea that exists but belongs to someone else. A third request answers 422 for an invalid name. Which of these are audited, which are not, and what principle separates them?

**Follow-ups**

1. What breaks if only the "not yours" case is audited?
2. Registration is the one 409/422 case that is audited. Why?
3. A request to a route that does not exist: is it audited? What do you use instead?

---

### AUDT-L4-01 — The audit table is down

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D2, D3, PRD §6.11, `docs/Architecture/Sequence.md` §6 |

**Prompt**

The audit table becomes unwritable for five minutes (for example the table is locked by a migration). Describe what each of these operations does during that time: a user creates an Idea, a user downloads slides, a logged-in user is denied access to an Idea, and a login attempt fails.

**Follow-ups**

1. Why does the download wait for the audit row before the first byte?
2. A download is audited, then storage fails. Is the audit log wrong? What do you say to an auditor?
3. How would you reduce the blast radius of this outage without giving up fail-closed for successful writes?

---

### AUDT-L4-02 — Append-only without database grants

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | PRD §10, §6.11, §9 #5 |

**Prompt**

The audit log must be append-only, but the application uses one database account that also holds DDL rights, and `TRIGGER` is not available. Explain what you can and cannot guarantee, how the project enforces append-only in this situation, and what threats remain.

**Follow-ups**

1. How does the automated test enforce the rule, and what does it miss?
2. Who can still change a row, and what evidence would exist afterwards?
3. What is the first change you would ask the platform team for if you could get one?

---

### AUDT-L4-03 — Hash chain under concurrent writes

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 15 |
| Status | Draft |
| Sources | D23, PRD §9 #5, §10 |

**Prompt**

An optional feature adds `prevHash` and `hash` (SHA-256) to each audit row so that tampering becomes visible. Two transactions on two replicas insert audit rows at the same time. Explain what makes this hard, which design choices you have, and what the chain does and does not prove.

**Follow-ups**

1. `audit_log.id` is a sequence and rows commit out of order. How does that break "the previous row" and what do you do about it?
2. Which option serializes the writers, what does it cost, and what is the alternative of sealing rows afterwards in batches?
3. Someone with full database access rewrites the whole table and recomputes every hash. What is detected?

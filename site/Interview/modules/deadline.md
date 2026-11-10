# Deadline Questions

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-11 |
| Prefix | `DEAD` |
| Status | **[Proposed]**: all questions are `Draft` until a second person reviews them with their rubric |

Questions about the submission deadline, the derived submission state, the `SubmissionGate` and database time. The file holds prompts, follow-ups and sources only. Rubrics live in the private answer key (see the [question template](../Rubric-template.md)).

Main sources: `docs/PRD.md` §5, §6.9, D7, D34, `docs/Architecture/Backend.md` §2.1, `docs/Architecture/Sequence.md` §3–§4.

| ID | Title | Level | Roles |
|---|---|---|---|
| `DEAD-L1-01` | Three states, one stored value | L1 | Junior / Senior |
| `DEAD-L1-02` | Which clock decides | L1 | Junior / Senior |
| `DEAD-L2-01` | Compute the state from the deadline | L2 | Junior / Senior |
| `DEAD-L2-02` | Validate a deadline update | L2 | Junior / Senior |
| `DEAD-L3-01` | Clock source across replicas | L3 | Senior / Architect |
| `DEAD-L3-02` | Check before and inside the transaction | L3 | Senior / Architect |
| `DEAD-L3-03` | Read the deadline without a lock | L3 | Senior / Architect |
| `DEAD-L4-01` | The upload that crosses the deadline | L4 | Senior / Architect |
| `DEAD-L4-02` | Admin moves the deadline during a rush | L4 | Senior / Architect |
| `DEAD-L4-03` | Forgetting the check in a new endpoint | L4 | Senior / Architect |

---

### DEAD-L1-01 — Three states, one stored value

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §5 |

**Prompt**

The system shows users that submission is "not open", "open" or "closed". The database holds only one deadline value in one row. Explain how the system gets from that value to the three words, and what the system shows before any deadline has ever been set.

---

### DEAD-L1-02 — Which clock decides

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §5, D7, `docs/Architecture/Architecture.md` |

**Prompt**

A user's laptop clock is 10 minutes behind. The countdown on the page reads 5 minutes left, and the user clicks "Submit". Which clock decides whether the submission is accepted, and why is that choice made for this system?

---

### DEAD-L2-01 — Compute the state from the deadline

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 10 |
| Status | Draft |
| Sources | PRD §5, §6.9, D7 |

**Prompt**

Write the Go function that returns the submission state. Its inputs are the current time and a deadline that may be missing. Then describe how `GET /deadline` produces its response body and which fields it contains.

**Follow-ups**

1. At the exact instant `now == deadline`, what state does your function return? Show the comparison that gives this result.
2. The deadline is stored in UTC with second precision, and the database time has microseconds. Which problem can this cause at the boundary, and how do you handle it?
3. Why does the response contain the server time, and what does the front end do with it?

---

### DEAD-L2-02 — Validate a deadline update

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.9, `docs/API/error-codes.md` |

**Prompt**

An Admin calls `PUT /admin/deadline` with a JSON body. Write the validation for the `deadline` field and say which HTTP status and error code each invalid input produces. Consider a missing field, `null`, a value without a time zone offset, a year of 1999, and a date in the past.

**Follow-ups**

1. Why is a date in the past accepted, and what does it do to the system?
2. Why can the Admin not clear the deadline?
3. The request carries `"deadline": "2026-12-01T10:00:00.789Z"`. What is stored, and what does the audit record contain?

---

### DEAD-L3-01 — Clock source across replicas

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D7, PRD §5, `docs/Architecture/Backend.md` §2 (`clock/`) |

**Prompt**

The backend runs on three replicas behind a load balancer. A developer proposes to call `time.Now()` in Go to decide whether the deadline has passed, because it is simpler and faster than a query. What do you answer, and what do you use instead?

**Follow-ups**

1. Two replicas have clocks that differ by 3 seconds. Describe one concrete user-visible failure.
2. Using the database time costs a query. How do you avoid a second round trip in the write path?
3. Tests need to check the behaviour at the deadline. How do you design the clock so that tests control time without breaking the rule?

---

### DEAD-L3-02 — Check before and inside the transaction

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D34, PRD §5, §6.1, `docs/Architecture/Sequence.md` §1, §3 |

**Prompt**

Every state-limited write checks the deadline twice: once early in the request, and once inside the database transaction. A reviewer says the second check is redundant and asks you to remove one of them. Which one would you remove, if any, and what do you lose?

**Follow-ups**

1. Which error does a user see for a closed deadline when the request also has a validation error? Why does that rule need the early check?
2. Give an input where the early check passes and the second check fails.
3. How does the API of the gate make it impossible to call the second check outside a transaction?

---

### DEAD-L3-03 — Read the deadline without a lock

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 8 |
| Status | Draft |
| Sources | D7, PRD §5 |

**Prompt**

Inside a write transaction, the system reads the deadline row without a shared lock, under `READ COMMITTED`. An engineer wants `SELECT … FOR SHARE` on that row so that no write can slip past a deadline change. Evaluate this proposal for this system.

**Follow-ups**

1. What does the shared lock do to an Admin who updates the deadline while many users upload?
2. Which writes can still land on the "wrong" side of a deadline change without the lock, and why does the product accept this?
3. What would have to be true about the business rules for you to accept the lock?

---

### DEAD-L4-01 — The upload that crosses the deadline

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | PRD §5, D36, `docs/Architecture/Sequence.md` §2, §3 |

**Prompt**

The deadline is 12:00:00. A leader on a slow connection starts uploading a 5 MiB file at 11:59:50. The body is fully received at 12:00:03. What happens to this request, and what state do the database and object storage hold afterwards?

**Follow-ups**

1. Where exactly in the flow is the 403 raised, and what cleans up the object that was already uploaded?
2. The user did the work in time and is angry. Someone proposes to check the deadline when the request starts. What does this change for the system and for fairness between users?
3. The transaction has not committed yet when the deadline passes, but the check inside it ran at 11:59:59. What is the outcome, and do you accept it?

---

### DEAD-L4-02 — Admin moves the deadline during a rush

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D7, PRD §5, §6.9, `docs/Architecture/Backend.md` §7 |

**Prompt**

Five minutes before the deadline, an Admin moves it forward by one hour. At the same moment 40 users are submitting. Some users received 403 `SUBMISSION_CLOSED` a few seconds earlier because the Admin first moved the deadline into the past by mistake. Describe what the system guarantees, what it does not, and what the audit log shows.

**Follow-ups**

1. Can one user's request see the old deadline while another user's request sees the new one at the same moment? Is this a bug?
2. A write racing with the update lands on either side. How would you make that statement testable?
3. How does an investigator later prove which requests were rejected because of the mistaken change?

---

### DEAD-L4-03 — Forgetting the check in a new endpoint

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D34, PRD §5, §6.1, `docs/Architecture/Backend.md` §11 |

**Prompt**

A developer adds a new user-facing write endpoint, "update team description", and forgets to call the deadline gate. All unit tests pass. How can this reach production without anyone noticing, and what mechanisms in the project do you build or rely on so that this defect is found before release?

**Follow-ups**

1. Which of the existing write endpoints are exempt from the deadline, and how does a test express an exemption without hiding a bug?
2. Design a test that fails when any new state-limited endpoint lacks the check.
3. Could the gate be applied in a middleware instead of in each service? State the cost and the risk of that choice.

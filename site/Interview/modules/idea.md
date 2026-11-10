# Idea Questions

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-11 |
| Prefix | `IDEA` |
| Status | **[Proposed]**: all questions are `Draft` until a second person reviews them with their rubric |

Questions about the Idea lifecycle: create, edit, delete, visibility, the uniform 404 and the list endpoint. The file holds prompts, follow-ups and sources only. Rubrics live in the private answer key (see the [question template](../Rubric-template.md)).

Main sources: `docs/PRD.md` §6.6, §7, `docs/Architecture/Backend.md` §8, `docs/Architecture/Sequence.md` §3–§5, D8, D9, D22, D56.

| ID | Title | Level | Roles |
|---|---|---|---|
| `IDEA-L1-01` | Who may see an Idea | L1 | Junior / Senior |
| `IDEA-L1-02` | Idea identity | L1 | Junior / Senior |
| `IDEA-L2-01` | Validate the create form | L2 | Junior / Senior |
| `IDEA-L2-02` | List with filters and paging | L2 | Junior / Senior |
| `IDEA-L3-01` | 404 and not 403 | L3 | Senior / Architect |
| `IDEA-L3-02` | One visibility query | L3 | Senior / Architect |
| `IDEA-L3-03` | Slide columns on the Idea row | L3 | Senior / Architect |
| `IDEA-L4-01` | Timing of the uniform 404 | L4 | Senior / Architect |
| `IDEA-L4-02` | Delete during a join approval | L4 | Senior / Architect |
| `IDEA-L4-03` | Duplicate name race | L4 | Senior / Architect |

---

### IDEA-L1-01 — Who may see an Idea

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §6.6, §7 |

**Prompt**

A registered user who is not in a team opens the URL of an Idea that exists. What does the user get? Name everyone who can see that Idea, and say what each group can do with it.

---

### IDEA-L1-02 — Idea identity

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §6.6, §6.1 |

**Prompt**

Two different leaders both create an Idea called "Green Packaging". Can this happen? Can the same leader create two Ideas with that name? Which value do join requests use to find the Idea, and what kind of identifier appears in the URL?

---

### IDEA-L2-01 — Validate the create form

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 15 |
| Status | Draft |
| Sources | PRD §6.2, §6.6, `docs/Architecture/Sequence.md` §3 |

**Prompt**

Write the validation for `POST /ideas` with a name, a summary, three enum fields and a slide file. Describe which limits apply to the name and summary, how the length is counted, and which field values the client must not control.

**Follow-ups**

1. A client sends a `leader` field in the form. What happens to it?
2. Why is a name with `/` or `:` rejected, and what other limit comes from the expected download filename?
3. How are the errors of several fields returned, and how many errors does one field report?

---

### IDEA-L2-02 — List with filters and paging

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 15 |
| Status | Draft |
| Sources | PRD §6.6 (Idea list) |

**Prompt**

Write the query behind `GET /ideas` with a name search `q`, an organization filter and paging. A search for `100%` must find names that contain the text `100%` and not every name. Describe the response body and what the server returns for page 999 when only 3 pages exist.

**Follow-ups**

1. How do you escape `%` and `_` in the user's search text?
2. A filter value is invalid. Does the server ignore it? Why?
3. Name ordering uses `utf8mb4_bin`. What is the visible effect for Chinese names, and why was this accepted?

---

### IDEA-L3-01 — 404 and not 403

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D9, D56, PRD §6.1, `docs/Architecture/Backend.md` §8 |

**Prompt**

A user requests an Idea they may not see. Many APIs answer 403 for this case. This project answers the same 404 that it gives for a missing Idea. Argue for this choice and describe what the user and the Admin see in each design.

**Follow-ups**

1. What does an attacker learn from a 403 on `GET /ideas/{id}` when IDs are random UUIDs, and when they are sequential numbers?
2. A malformed ID like `abc` is requested. Which response does it get, and what is written to the audit log?
3. What is the cost of the uniform 404 for support staff who need to explain "why can't I open my Idea"?

---

### IDEA-L3-02 — One visibility query

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D9, `docs/Architecture/Backend.md` §8, §2.1 |

**Prompt**

Two designs for reading an Idea: (A) load the Idea by ID, then check in Go whether the actor may see it; (B) one SQL query that includes the visibility condition and returns nothing when the actor may not see the Idea. The project uses B and exposes it as the only way to read an Idea. Compare them.

**Follow-ups**

1. What difference in behaviour or timing can the two designs show for "missing" and "not yours"?
2. How does the code make sure a developer cannot read an Idea by a path that skips the visibility condition?
3. The same fragment is used by the list query. What is the benefit, and what is the risk when someone edits it?

---

### IDEA-L3-03 — Slide columns on the Idea row

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 8 |
| Status | Draft |
| Sources | D22, PRD §6.6 |

**Prompt**

The slide metadata (object key, extension, size, SHA-256, update time) is stored as non-null columns in the `idea` table. The alternative is a separate `slide` table with a one-to-one link. Compare the two for this system.

**Follow-ups**

1. Which business rule does the non-null column enforce without any application code?
2. What does the create flow have to do to make this possible, and what does that imply for the order of upload and insert?
3. When would you move to a separate table, for example if an Idea could hold several files?

---

### IDEA-L4-01 — Timing of the uniform 404

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D9, D48, D56, PRD §6.11 |

**Prompt**

Two requests go to `GET /ideas/{id}`. In one, the Idea does not exist; in the other, it exists but the caller is not allowed to see it. The body and status are identical. An attacker measures response times over many requests. List every place where the two paths could differ, and explain how the design closes each one.

**Follow-ups**

1. The audit write is part of this: why must both cases write the same audit row?
2. Which differences remain between an anonymous caller and an authenticated one, and are they a leak?
3. How would you test that the paths do not diverge?

---

### IDEA-L4-02 — Delete during a join approval

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D8, `docs/Architecture/Sequence.md` §5, PRD §6.6–§6.7 |

**Prompt**

A Leader approves a join request at the same moment as the Leader's own second browser tab deletes the Idea. Describe the possible outcomes, what the database guarantees, and what each caller receives. Include what happens to the slide file.

**Follow-ups**

1. Which lock serializes the two operations, and which row does it lock?
2. What happens to member and pending-request rows when the Idea is deleted, and where is the record of who was in the team?
3. Storage is down during the delete. What does the Leader receive and where does the file key go?

---

### IDEA-L4-03 — Duplicate name race

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.6, `docs/Architecture/Sequence.md` §3, D36 |

**Prompt**

A Leader double-clicks "Submit" and two identical create requests reach two replicas. Both pass the optional read-only duplicate check. Both upload their file. Describe what each request does from there, what the database holds at the end, and what the client sees.

**Follow-ups**

1. Which constraint is authoritative, and what is the read-only pre-check for?
2. What happens to the second request's uploaded object?
3. Case-sensitive names and clean-up: does `"Green  Idea"` with two spaces collide with `"Green Idea"`? Explain which step decides.

# Storage Questions

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-11 |
| Prefix | `STOR` |
| Status | **[Proposed]**: all questions are `Draft` until a second person reviews them with their rubric |

Questions about slide upload, download, MinIO and storage consistency. The file holds prompts, follow-ups and sources only. Rubrics live in the private answer key (see the [question template](../Rubric-template.md)).

Main sources: `docs/Architecture/Sequence.md`, D5, D36–D38, D47, D50.

| ID | Title | Level | Roles |
|---|---|---|---|
| `STOR-L1-01` | Where the slide bytes go | L1 | Junior / Senior |
| `STOR-L1-02` | Object key for a replaced file | L1 | Junior / Senior |
| `STOR-L2-01` | Order of work in an upload handler | L2 | Junior / Senior |
| `STOR-L2-02` | Receiving a large body safely | L2 | Junior / Senior |
| `STOR-L3-01` | Download through the backend | L3 | Senior / Architect |
| `STOR-L3-02` | Replace while storage is down | L3 | Senior / Architect |
| `STOR-L3-03` | Audit row before the first byte | L3 | Senior / Architect |
| `STOR-L4-01` | Upload commit fails | L4 | Senior / Architect |
| `STOR-L4-02` | Two leaders replace at once | L4 | Senior / Architect |
| `STOR-L4-03` | Storage fails in the middle of a download | L4 | Senior / Architect |

---

### STOR-L1-01 — Where the slide bytes go

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | D5, `docs/Architecture/Architecture.md` |

**Prompt**

A team member opens the page of their Idea and clicks "Download slides". The browser receives a file. Describe which systems the bytes pass through between object storage and the browser.

---

### STOR-L1-02 — Object key for a replaced file

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | D50, `docs/Architecture/Sequence.md` §1 (S4) |

**Prompt**

A leader uploads `report.pptx` for an Idea, then uploads a different `report.pptx` for the same Idea one hour later. Both files have the same name. How does the system name the two objects in storage, and what do they contain in their names?

---

### STOR-L2-01 — Order of work in an upload handler

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | D37, `docs/Architecture/Sequence.md` §1 (S2), §3 |

**Prompt**

You write the handler for `POST /ideas` (multipart: text fields and one slide file). The handler must check the session, the role, the submission deadline, the text fields, the file type and size, and it must store the file and insert a database row. Write the steps in the order you would run them and say why each step sits where it sits.

**Follow-ups**

1. A request has a valid file but an invalid Idea name. Which steps ran, and what exists in storage afterwards?
2. Where do you generate the Idea ID, and why does the order of your steps force that choice?
3. You can run the file validation either before or after the database transaction starts. Which do you choose, and what does it cost to choose the other?

---

### STOR-L2-02 — Receiving a large body safely

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | D38, `docs/Architecture/Sequence.md` §1 (S1, S3) |

**Prompt**

The upload limit is 5 MiB, and 200 users may upload at the same time. You must know the byte count and the SHA-256 of each file before you can validate it and write an audit record. Show how you read the request body in Go so that memory stays bounded and the temporary data is always removed.

**Follow-ups**

1. A client sends a 2 GiB body. Where in your code does the request stop, and how much of the body did you read?
2. The handler panics halfway. How do you make sure the temporary file does not stay on disk?
3. Why compute the SHA-256 while reading, and not in a second pass over the file?

---

### STOR-L3-01 — Download through the backend

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D5, D47, `docs/Architecture/Sequence.md` §6 |

**Prompt**

The team considers two designs for slide download. In design A, the backend opens a stream from object storage and copies it to the client. In design B, the backend checks access and returns a short-lived signed URL, and the browser fetches the file from object storage. The files are at most 5 MiB. Which design do you pick for this system, and what would change your choice?

**Follow-ups**

1. In design B, a user loses access to the Idea after the URL was issued. What happens, and who can still download?
2. Which audit guarantee is easy in design A and hard in design B?
3. At which file size or request rate does design A start to hurt, and what do you change first?

---

### STOR-L3-02 — Replace while storage is down

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D36, `docs/Architecture/Sequence.md` §2, §4, §5 |

**Prompt**

Object storage is unavailable for 20 minutes. During this time, a leader deletes an Idea, and another leader tries to replace the slides of their own Idea. What does each user see, and what state do the database and storage end up in when storage comes back?

**Follow-ups**

1. Why can delete succeed while replace cannot, even though both touch an object?
2. For the delete, where is the key recorded, and in which transaction? What goes wrong if you record it after the commit?
3. Someone proposes to make delete fail with 503 when storage is down, "to keep both systems in step". Argue for or against.

---

### STOR-L3-03 — Audit row before the first byte

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 8 |
| Status | Draft |
| Sources | D47, `docs/Architecture/Sequence.md` §6 |

**Prompt**

Every slide download must leave an audit row. A developer writes the download handler so that it streams the file first and writes the audit row when the stream ends. Review this order. What can go wrong, and what order do you want instead? What cost does your order have?

**Follow-ups**

1. Storage fails right after the audit row is committed. The user got a 503. Is the audit log wrong? Defend your answer.
2. A router maps `HEAD` requests to the `GET` handler. What happens with your design, and how do you prevent it?
3. A client on a slow connection takes two minutes to read 5 MiB. Which resource does your design hold during this time, and which one should it not hold?

---

### STOR-L4-01 — Upload commit fails

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D36, D44, `docs/Architecture/Sequence.md` §2, §4, `docs/Architecture/Backend.md` §4.4 |

**Prompt**

A team leader replaces the slide file of their Idea. The backend has already stored the new file in object storage. It then runs `COMMIT` for the database transaction that points the Idea to the new file, and `COMMIT` returns an error. What does the backend do with the new object and the old object, and what does the user see?

**Follow-ups**

1. How do you tell the different kinds of `COMMIT` error apart?
2. The database connection drops during `COMMIT`, so you cannot know whether it succeeded. What is the safe choice, and why?
3. Two replicas of the backend run at the same time. Who cleans up objects that are no longer needed, and how do you stop two replicas from doing the same work?

---

### STOR-L4-02 — Two leaders replace at once

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D36, `docs/Architecture/Sequence.md` §4 |

**Prompt**

An Idea has one slide file, key `K0`. An Admin and the Leader both upload a new file for this Idea in the same second. Both uploads reach object storage before either database transaction starts. Walk through what the database and storage hold after both requests finish, and show that no object is lost or left without an owner.

**Follow-ups**

1. What would go wrong if both transactions read `K0` without a lock and both queued `K0` for deletion?
2. Your transaction holds a row lock. Which calls must never run while it is held, and what breaks if one does?
3. The version column also changes on replace. Does it replace the row lock here? Explain what each one protects.

---

### STOR-L4-03 — Storage fails in the middle of a download

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D47, D50, `docs/Architecture/Sequence.md` §6 |

**Prompt**

The backend has sent `200` and the response headers for a 5 MiB download. After 2 MiB, the connection to object storage breaks. The backend can no longer change the status code. What does the backend do, and how does the client find out that the file is incomplete?

**Follow-ups**

1. Which response header makes this detectable, and what happens if the backend leaves it out?
2. The backend retries the storage read automatically after the break. Why is this a risky choice for the bytes already sent?
3. How do you make this failure visible to operators when the client sees only a broken download?

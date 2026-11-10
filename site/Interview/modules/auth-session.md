# Authentication and Session Questions

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-11 |
| Prefix | `AUTH` |
| Status | **[Proposed]**: all questions are `Draft` until a second person reviews them with their rubric |

Questions about login, the session, CSRF protection, login throttling and logout. The file holds prompts, follow-ups and sources only. Rubrics live in the private answer key (see the [question template](../Rubric-template.md)).

Main sources: `docs/PRD.md` §6.4–§6.5, `docs/Architecture/Backend.md` §6, D24, D40–D43.

| ID | Title | Level | Roles |
|---|---|---|---|
| `AUTH-L1-01` | One answer for every failed login | L1 | Junior / Senior |
| `AUTH-L1-02` | What the cookie holds | L1 | Junior / Senior |
| `AUTH-L2-01` | Validate a session in one statement | L2 | Junior / Senior |
| `AUTH-L2-02` | Compare tokens and secrets | L2 | Junior / Senior |
| `AUTH-L3-01` | Counters in the database | L3 | Senior / Architect |
| `AUTH-L3-02` | A CSRF token bound to the session | L3 | Senior / Architect |
| `AUTH-L3-03` | Stored session identifier | L3 | Senior / Architect |
| `AUTH-L4-01` | Account enumeration by timing | L4 | Senior / Architect |
| `AUTH-L4-02` | Lockout used against a victim | L4 | Senior / Architect |
| `AUTH-L4-03` | Two requests, one expiring session | L4 | Senior / Architect |

---

### AUTH-L1-01 — One answer for every failed login

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §6.5 |

**Prompt**

A visitor sends a login request with an employee ID that does not exist. A second visitor sends a correct employee ID with a wrong password. A third visitor sends the ID of a disabled account. What does each visitor receive, and why is it designed this way?

---

### AUTH-L1-02 — What the cookie holds

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | `docs/Architecture/Backend.md` §6 |

**Prompt**

After login, the browser holds a session cookie. Describe what the cookie value is, how the server creates it and which cookie attributes you set. Say what each attribute protects against.

---

### AUTH-L2-01 — Validate a session in one statement

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 15 |
| Status | Draft |
| Sources | D42, PRD §6.5, `docs/Architecture/Backend.md` §6 |

**Prompt**

Write the middleware step that decides, for each request, whether the session is valid. The rules: idle timeout 30 minutes, absolute timeout 8 hours, the account must still be enabled, and all time values come from the database. Show the SQL and the Go code that interprets the result.

**Follow-ups**

1. Why is validation and idle refresh one statement and not a `SELECT` followed by an `UPDATE`?
2. The system refreshes `last_access_at` at most once per 60 seconds. What does this save, and what does it do to the real idle timeout?
3. How does your code know the statement matched a row, and which driver setting does it depend on?

---

### AUTH-L2-02 — Compare tokens and secrets

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 10 |
| Status | Draft |
| Sources | `docs/Architecture/Backend.md` §6 (CSRF middleware), D40 |

**Prompt**

The CSRF middleware receives a token from a header and a token from a cookie, and it must verify the HMAC inside the token. Write the comparison code in Go. State which standard library functions you use and why a plain `==` is not acceptable.

**Follow-ups**

1. What are the two comparisons the middleware makes, and which secret does each one need?
2. The HMAC key is rotated. How does the system keep users who hold tokens signed with the old key from failing?
3. Which request methods does the middleware check, and which does it skip?

---

### AUTH-L3-01 — Counters in the database

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D24, PRD §6.5, `docs/Architecture/Backend.md` §6 |

**Prompt**

Login throttling needs a failure counter per employee-ID string and a failure rate per source IP. A colleague wants to keep both in the memory of each backend replica, because it is the simplest option. The project stores them in two database tables instead. Evaluate both options.

**Follow-ups**

1. Why does the per-key rule need a lock table, and the per-IP rule an event table? Describe the difference between "5 consecutive failures" and "30 failures in a sliding window".
2. The attacker sends requests to three replicas in turn. What happens with in-memory counters?
3. Under which conditions would you move the counters to Redis, and what would you lose?

---

### AUTH-L3-02 — A CSRF token bound to the session

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D40, D43, PRD §6.5 |

**Prompt**

`GET /auth/csrf` is public: anyone, including an attacker, can obtain a token from it. The project still treats the token as protection. Explain how this can work. Your answer must cover what the signature covers, what the attacker can do with a token they fetched themselves, and what happens to the token at login and logout.

**Follow-ups**

1. What does an unbound HMAC token prove, and why is that not enough here?
2. A user logs out in one tab. Another tab still holds the old token. What does the client do?
3. Where in the middleware chain does the CSRF check sit relative to authentication and body reading, and what error precedence results?

---

### AUTH-L3-03 — Stored session identifier

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 8 |
| Status | Draft |
| Sources | `docs/Architecture/Backend.md` §6 |

**Prompt**

The `session` table stores `SHA-256(id)` and never the session ID that is in the cookie. A reviewer says that hashing a random 256-bit value adds no security and wastes CPU. What do you answer?

**Follow-ups**

1. Why is a fast hash acceptable here, while a password needs bcrypt?
2. A database backup leaks. What can the attacker do with the `session` table?
3. How many bytes of randomness does the ID have, and which source does the code use? What changes if a developer uses a time-seeded generator?

---

### AUTH-L4-01 — Account enumeration by timing

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.4, §6.5, `docs/Architecture/Backend.md` §6 (Passwords), D51 |

**Prompt**

The login endpoint returns the same 401 for every failure. An attacker still measures response times and finds which employee IDs have accounts. Explain how the timing leak can appear in the login handler, how the system closes it, and what an Admin account changes about the problem.

**Follow-ups**

1. What does the handler do for an unknown account, a disabled account and an input longer than 72 bytes?
2. The system rejects stored Admin hashes whose bcrypt cost is not 10. What attack does this prevent?
3. A throttled attempt does not run bcrypt at all. Does this create a new timing signal? Reason about what the attacker learns.

---

### AUTH-L4-02 — Lockout used against a victim

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.5, `docs/Architecture/Backend.md` §6 (throttling) |

**Prompt**

An attacker wants to stop a specific employee from logging in on the last day before the deadline. They know the employee ID. Using the throttling rules of this system, describe what the attacker can do, what the victim experiences, and what the system does to limit the damage.

**Follow-ups**

1. During a lock, the victim enters the correct password. What does the response contain, and does the lock get longer?
2. Counters are kept per submitted string, even when the account does not exist. Why, and what attack would an account-only counter allow?
3. The system sits behind a proxy. How does it find the client IP, and what happens to the per-IP limit if you trust the wrong number of hops?

---

### AUTH-L4-03 — Two requests, one expiring session

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D42, PRD §6.5, `docs/Architecture/Backend.md` §6, §9 |

**Prompt**

A session is 29 minutes and 59 seconds idle. Two requests from the same browser arrive at two different replicas within the same second. In the same second, an Admin's account is removed from the directory and the sync disables it. Walk through the outcomes for both requests, and say which guarantees the design gives about each one.

**Follow-ups**

1. What makes the idle refresh safe when two replicas run it at the same time?
2. A disabled account has an open session. Which statement or step makes the very next request fail, and does it need a separate revoke call?
3. Logout is idempotent. Describe its behaviour with a valid session, with an expired session and without any cookie, and what still must be valid in the last case.

# Account Questions

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-11 |
| Prefix | `ACCT` |
| Status | **[Proposed]**: all questions are `Draft` until a second person reviews them with their rubric |

Questions about self-registration, the password policy, the Admin directory (Vault sync) and the `SessionRevoker` boundary between `account` and `auth`. The file holds prompts, follow-ups and sources only. Rubrics live in the private answer key (see the [question template](../Rubric-template.md)).

Main sources: `docs/PRD.md` §4, §6.3–§6.4, `docs/Architecture/Backend.md` §2.1, §6, §9, D30, D51.

| ID | Title | Level | Roles |
|---|---|---|---|
| `ACCT-L1-01` | Where Admin accounts come from | L1 | Junior / Senior |
| `ACCT-L1-02` | Password storage | L1 | Junior / Senior |
| `ACCT-L2-01` | Order of registration checks | L2 | Junior / Senior |
| `ACCT-L2-02` | Password policy with every violated rule | L2 | Junior / Senior |
| `ACCT-L3-01` | Why `account` and `auth` are two modules | L3 | Senior / Architect |
| `ACCT-L3-02` | Registering with an Admin's employee ID | L3 | Senior / Architect |
| `ACCT-L3-03` | Replicas syncing the same directory | L3 | Senior / Architect |
| `ACCT-L4-01` | Admin removed from Vault | L4 | Senior / Architect |
| `ACCT-L4-02` | Directory entry collides with a User | L4 | Senior / Architect |
| `ACCT-L4-03` | Duplicate registration race | L4 | Senior / Architect |

---

### ACCT-L1-01 — Where Admin accounts come from

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §4 |

**Prompt**

A new colleague needs Admin rights. There is a registration page on the site. Can they become an Admin through it? Explain where Admin accounts come from and what data they have.

---

### ACCT-L1-02 — Password storage

| | |
|---|---|
| Level | L1 |
| Roles | Junior / Senior |
| Time | 3 |
| Status | Draft |
| Sources | PRD §6.4, `docs/Architecture/Backend.md` §6 |

**Prompt**

How does the system store a registered user's password, and why does it not use a plain SHA-256 hash? State one limit that bcrypt has on the input and where the system handles it.

---

### ACCT-L2-01 — Order of registration checks

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.3 |

**Prompt**

Write the service method that registers a user from an employee ID, a name, an email and a password. Describe the order of the checks, and say which errors the caller receives together in one response and which come alone.

**Follow-ups**

1. A user sends an employee ID of five digits and a password that violates two rules. What does the response contain?
2. Where do you check that the employee ID is not taken, and why not before the format checks?
3. The email is not part of the audit record. Which fields are, and why?

---

### ACCT-L2-02 — Password policy with every violated rule

| | |
|---|---|
| Level | L2 |
| Roles | Junior / Senior |
| Time | 12 |
| Status | Draft |
| Sources | PRD §6.4 |

**Prompt**

Write the function that checks a password against the policy and returns every violated rule, not only the first. The policy includes a blocklist of common passwords and rules about sequences and about containing the employee ID.

**Follow-ups**

1. How do you load the common-password list so that it ships inside the binary, and how do you compare case-insensitively?
2. Why does the response list all violated rules for the password, while other fields report only their first error?
3. A password such as `1234` breaks several rules with one substring. How does your function report this?

---

### ACCT-L3-01 — Why `account` and `auth` are two modules

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D30, `docs/Architecture/Backend.md` §2.1, `docs/Architecture/Backend.md` §2.4 |

**Prompt**

Registration and the Admin directory sit in `account`. Login, sessions and CSRF sit in `auth`. When an Admin's hash changes, `account` must delete that Admin's sessions, but `account` is not allowed to import `auth`. Show how the code is arranged so that this works, and argue whether the split is worth the trouble.

**Follow-ups**

1. Where is the interface declared, who implements it, and who wires the two together?
2. How does the project check that no one adds the forbidden import later?
3. What is the alternative that merges both modules, and what is its cost for the Vault uncertainty?

---

### ACCT-L3-02 — Registering with an Admin's employee ID

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 8 |
| Status | Draft |
| Sources | PRD §4, `docs/API/error-codes.md` |

**Prompt**

An attacker registers with the employee ID of a real Admin before the Admin has ever logged in, hoping to take over the identity. How does the system handle the request, and where does the Admin's ID live so that the check can work?

**Follow-ups**

1. Which status and error code does the attacker receive, and is it the same as for any other taken ID? Why?
2. Admin accounts are synced every 60 seconds. Can a registration slip in between a Vault change and the next sync? What happens then?
3. Admins have no email and no password policy. Where does this difference come from, and what does it do to the login code path?

---

### ACCT-L3-03 — Replicas syncing the same directory

| | |
|---|---|
| Level | L3 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | PRD §4, `docs/Architecture/Backend.md` §9, D10, D25 |

**Prompt**

Three replicas each read the Vault directory at startup and every 60 seconds. Describe what can go wrong when all three apply the changes to the same `app_user` rows, and what the design must guarantee so that the result is the same as with one replica.

**Follow-ups**

1. Two replicas see a changed hash at the same time. Walk through the writes and the session revocations.
2. Vault is unreachable for 10 minutes. What happens to existing Admin sessions and to Admin login?
3. The file-based directory replaces Vault in local and test environments. What must the interface guarantee so that this stays safe?

---

### ACCT-L4-01 — Admin removed from Vault

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | PRD §4, `docs/Architecture/Backend.md` §6, §9, D30 |

**Prompt**

At 10:00:00 an Admin is removed from Vault while they are logged in and are downloading Ideas. The sync runs at 10:00:40. Describe exactly when the Admin loses access, which two mechanisms contribute, and what an auditor finds in the audit log afterwards.

**Follow-ups**

1. Why does the system check "account enabled" on every request and not only revoke sessions at sync time?
2. The sync disables the account but the session deletion fails. What does the Admin's next request do?
3. The same person is added to Vault again a week later. What happens to the old account, and what is re-created?

---

### ACCT-L4-02 — Directory entry collides with a User

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | D51, PRD §4 |

**Prompt**

A person registered as a normal User and became a team Leader. Later their employee ID appears in the Vault Admin list. What does the sync do with this entry, what does it do with the other entries in the same run, and why?

**Follow-ups**

1. Why does the system not convert the User into an Admin?
2. Where do operators see this conflict, and what do they have to do to resolve it?
3. Which alternatives exist (reject the whole run, convert, merge roles) and what breaks in each of them?

---

### ACCT-L4-03 — Duplicate registration race

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 10 |
| Status | Draft |
| Sources | PRD §6.3, `docs/Architecture/Backend.md` §10 |

**Prompt**

Two requests register the same employee ID at the same moment, on two replicas. Both pass the application-level uniqueness check. What guarantees that exactly one account exists, what does the second caller see, and what is written to the audit log for each request?

**Follow-ups**

1. Which database constraint settles the race, and how does the Go code recognise this error?
2. The unique key and the application check give two ways to produce `EMP_NO_TAKEN`. Why keep both?
3. How does the collation of the employee-ID column affect which two inputs count as the same ID?

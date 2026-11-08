# ESG Idea Contest — Product Requirements Document
| | |
|---|---|
| Version | 1.7 (draft for review) |
| Date | 2026-11-08 |
| Language | English (all repo-internal documents are English; the public `site/` is bilingual zh-TW / en) |
| Status markers | **[Decided]** agreed in discussion · **[Proposed]** drafted by the author, awaiting confirmation · **[Open]** unresolved (tracked in `docs/Open-items.md`) |

Normative words MUST / MUST NOT / SHOULD are used in the RFC 2119 sense.

Related documents: technical design in `docs/Architecture/` (`Architecture.md`, `Backend.md`, `Frontend.md`, `Sequence.md`); API contract in `docs/API/` (`api-endpoints.md`, `error-codes.md`, `response-bodies.md`); unresolved questions in `docs/Open-items.md`, referred to here as **OI-nn**.

## 1. Overview
A large enterprise wants to collect ESG (Environmental, Social, Governance) ideas from its internal IT employees and run a contest to select the best ones. Employees form teams of 1–4 people. The core object is the **Idea**. One team is exactly one Idea; the team leader is the proposer.
### 1.1 Goals
1. An employee can register with an employee ID (6 digits), a name and an email, then submit an Idea.
2. Administrators can see and manage every Idea.
3. The Audit Log records registrations, logins, Idea writes, slide downloads and denied attempts.
### 1.2 Non-goals
Judging, scoring, voting and ranking are **not** part of this system (see §3.2).

## 2. Glossary
| Term | Meaning |
|---|---|
| Idea | The core object. One Idea = one team = one slide file. |
| User | An account with role `USER`, created by self-registration. |
| Admin | An account with role `ADMIN`, provisioned from HashiCorp Vault only. |
| Leader | The User who created an Idea. A *relation* to one Idea, not an account role. |
| Member | A User whose join request was approved by the Leader. A *relation* to one Idea. |
| Join request | A User's request to join an Idea, addressed by the Leader's employee ID plus the Idea name. Stored as an `idea_member` row with status `PENDING`. |
| ESG direction | One of five themes (§6.6). |
| Expected filename | The filename the slides must carry, built from Idea data (§6.8). |
| Submission state | `NOT_OPEN`, `OPEN` or `CLOSED`, derived from the deadline and database time. |

## 3. Scope
### 3.1 In scope
- Self-service register, log in, log out; available in every submission state.
- Idea create / view / edit / delete by the Leader only. Members can view and download.
- Joining a team by **join request**: a registered User enters the Leader's employee ID and the Idea name; the Leader approves or rejects. The Leader can remove members; a Member can leave; an applicant can withdraw. **[Decided]**
- One slide file per Idea (PDF or PPTX); re-upload overwrites; the filename is checked.
- A system-wide submission deadline with a countdown. After it, uploads, edits, leader deletes and all team actions are blocked; viewing and downloading remain possible.
- Admin: set the deadline, see and operate every Idea (view, download, delete, replace slides), query the Audit Log through the API.
- Admin accounts exist only through Vault sync, never through registration.
- Audit Log of writes, logins, downloads and denied attempts.
- Build, CI/CD (GitHub Actions) and deployment to the company Kubernetes DEV environment via Argo CD.
- Every functional requirement in this document, without exception.
### 3.2 Out of scope
- 2FA, MFA, password reset, email verification, company SSO, CAPTCHA, OTP.
- **Changing one's own password, name or email.** A Leader's name in the expected filename is therefore fixed. **[Decided]**
- Audit Log front-end UI (API only); off-site copy of the Audit Log.
- Super Admin, break-glass procedure, user management by Admins.
- Soft delete, restore, recycle bin.
- Slide version history, multiple files per Idea, online preview or editing.
- Per-year events, archiving, multiple contests.
- Trusted-device cookie; registration rate limit.
- Judging, scoring, voting, ranking.
- Leadership transfer (a Leader can only dissolve the team).
- Native mobile apps (responsive web only).
- Leader-initiated invitations and employee-ID lookup (replaced by join requests, §6.7); a cap on pending join requests per Idea. **[Decided]**
- HTTP `Range` and `HEAD` on slide downloads (§6.8).
- Message queues, distributed transactions, database replication or cross-site backup managed by this project.

## 4. Users and Roles
### 4.1 Actors
| Actor | Definition |
|---|---|
| Visitor | Not logged in. |
| User | Logged in with role `USER`. |
| Applicant | A User with a `PENDING` join request on one Idea. Has no access to that Idea. |
| Leader | A User related to one Idea as its proposer. |
| Member | A User related to one Idea as an approved team member. |
| Admin | Logged in with role `ADMIN`. |
### 4.2 Role rules **[Decided]**
- Only two account roles exist: `USER` and `ADMIN`. Leader, Member and Applicant are relations to **one Idea**.
- A User can lead several Ideas and be a Member of others at the same time.
- An Admin can never be a Leader, Member or Applicant.
- Admin accounts are read from Vault at startup and re-synced every 60 seconds. They use the same login page and endpoint as Users.
- Removing an admin from Vault disables the account and deletes its sessions. Every authenticated request also checks that the account is still enabled.
- Registering with an Admin's employee ID is rejected with 409 `EMP_NO_TAKEN`.
- Admin data in Vault: `empNo`, `name`, bcrypt hash. Admins have no email and the password policy does not apply to them.

Vault sync rules **[Proposed]** (mechanism in `docs/Architecture/Backend.md` §9):
- An entry whose employee ID belongs to a self-registered User is **skipped** (error log, metric, `ADMIN_SYNC` FAILED); other entries sync normally. The User is never converted.
- An Admin removed and later added again is **re-enabled**.
- Name and hash are updated when they change in Vault; a changed hash also revokes that Admin's sessions.
- A hash with bcrypt cost other than 10 is **rejected** (entry skipped), because a different cost changes login timing and defeats the dummy-hash design (§6.4). Owner confirmation: OI-13.
- If a sync fails after startup, existing Admins are kept and an alert metric is raised; if the first read fails, the service is not ready (OI-40).
- The Admin directory sits behind an interface with a Vault implementation and a file-based implementation for local and test profiles only. The Vault authentication method is open (OI-01, OI-41).

## 5. Submission Lifecycle
States are **derived**, never stored: `state = f(databaseNow, deadline)`.

| State | Condition |
|---|---|
| `NOT_OPEN` | No deadline has ever been set. |
| `OPEN` | `now < deadline` |
| `CLOSED` | `now >= deadline` |

Rules:
- An Admin can change the deadline but can **not** clear it.
- Registration, login, logout, viewing and downloading are never limited by the deadline.
- "Now" is the **database server time** when the request is processed. All replicas use the same time source. There is no grace period.
- User writes (Idea create/edit/delete, slide upload, join request, withdraw, approve, reject, remove, leave) are allowed only in `OPEN`. In `NOT_OPEN` they fail with 403 `SUBMISSION_NOT_OPEN`; in `CLOSED` with 403 `SUBMISSION_CLOSED`.
- Admins are not limited by the deadline for the operations they may perform.
- The authoritative check happens **inside the write transaction, after the full request body has been received**; an upload that starts before the deadline but finishes after it is rejected. A read-only pre-check may run earlier to keep the error precedence of §6.1, but never replaces it.
- The deadline row is read without a shared lock; a write racing with a deadline change may land on either side. **[Decided]**

## 6. Functional Requirements
> Functional requirements are verified at API level. The Web UI MUST exist and work, but is not part of the functional acceptance checks.
### 6.1 Conventions **[Decided unless noted]**
- Base path `/api/v1`. The front end is served from the **same origin** (the reverse proxy routes `/` to the front end and `/api` to the backend); no CORS.
- JSON field names are `camelCase`. Timestamps are ISO 8601 UTC with `Z` and **millisecond precision**. **[Proposed]**
- Resource IDs are random UUIDs; auto-increment IDs are never exposed, except `audit_log.id` (a sequence needed by the hash chain, §9 #5), visible to Admins only.
- Employee IDs are always strings (`089290` keeps its leading zero).
- Every write request (including login, register, logout) is CSRF-protected (§6.5).
- Check order: **401 → CSRF 403 → 404 → role 403 → state 403 → validation errors**.
- **Resources addressed by the body** are validated before the lookup. **[Proposed]**
  - `POST /join-requests`: 401 → CSRF 403 → role 403 (Admin) → state 403 → 400 / 422 → 404 → 409 / 422 (`CANNOT_JOIN_OWN_IDEA`).
  - `POST /ideas` has no lookup: 401 → CSRF 403 → role 403 → state 403 → 400 / 422 / 413 / 415 → 409 `IDEA_DUPLICATE`.
- **Exception:** a transport-level size limit (reverse proxy or servlet container) may answer 413 `FILE_TOO_LARGE` **before any other check**. Oversize bodies are not read to completion to produce an earlier 401, 403 or 404. **[Decided]**
- **Missing versus empty.** **[Proposed]**
  - A required field that is absent, JSON `null`, or a missing multipart part → 400 `MALFORMED_REQUEST`.
  - A field present but empty after clean-up (§6.2) → 422 `REQUIRED`.
  - A JSON value of the wrong type (`"empNo": 89290`) → 400 `MALFORMED_REQUEST`; type coercion is disabled.
- Unknown JSON fields and query parameters are **ignored**; duplicate JSON keys → 400 `MALFORMED_REQUEST`. **[Proposed]**
- 422 takes precedence over 409: a uniqueness conflict is reported only when every field is valid. **[Proposed]**
- Unknown paths: without a valid session → 401; with a session → 404 `NOT_FOUND`. Wrong method → 405 `METHOD_NOT_ALLOWED`; unsupported `Accept` → 406 `NOT_ACCEPTABLE`; both as problem+json (OI-15). **[Proposed]**
- List endpoints enforce a maximum page size of 100; larger values are rejected with 422, not truncated.
### 6.2 Text clean-up and character counting
Front end and back end MUST apply the same rules and share one set of test cases (`testdata/text-cleanup.json`).

1. Apply Unicode **NFC** normalization.
2. Whitespace means the Unicode `White_Space` property (25 code points: U+0009–U+000D, U+0020, U+0085, U+00A0, U+1680, U+2000–U+200A, U+2028, U+2029, U+202F, U+205F, U+3000). Language-built-in `trim`/`isWhitespace` MUST NOT be used.
3. **Person name and Idea name** (also the `ideaName` of a join request): trim, then collapse every whitespace run (including line breaks and tabs) into one ASCII space.
4. **Summary:** convert CRLF and CR to LF, then trim. Interior whitespace is kept; line breaks are allowed.
5. Reject remaining control characters (`Cc`) with 422 `INVALID_FORMAT`; in the summary only LF is permitted. **[Proposed: error code]**
6. Length is counted in **Unicode code points**.
7. The stored, counted and filename values are the same cleaned-up value.
8. Person names and Idea names MUST NOT contain `/ \ : * ? " < > |` (they become part of the expected filename) → 422 `INVALID_NAME_CHARS`.
9. Zero-width characters are not whitespace and are not rejected.

**Error reporting per field [Proposed]:** errors of all fields are collected in one response, but each field reports only its **first** error, in the order `REQUIRED` → `INVALID_FORMAT` (control character) → `INVALID_NAME_CHARS` → `OUT_OF_RANGE` (length). The password is the exception (§6.4). The order is part of `testdata/`.
### 6.3 Registration and account
- A visitor registers with **employee ID + name + email + password**, through both the Web UI and the API.
- Employee ID: exactly 6 ASCII digits (`[0-9]{6}`).
- Name: 1–50 code points after clean-up. Not unique.
- Email:
  - trimmed (§6.2 whitespace set); **no whitespace anywhere** in the rest **[Decided]**;
  - printable ASCII only (U+0021–U+007E) **[Proposed]**, so lower-casing is the ASCII mapping and front end and back end cannot disagree; owner confirmation: OI-14;
  - exactly one `@`, a non-empty local part and a non-empty domain (a dot is not required);
  - at most 254 characters; lower-cased before storage;
  - unique (byte-exact after lower-casing, `utf8mb4_bin`). No verification email is sent.
- Employee ID and email MUST be unique → 409 `EMP_NO_TAKEN` or 409 `EMAIL_TAKEN`.
- Registration does **not** log the user in and sets no session cookie. It has no time window and no rate limit.
- Validation order: format → password policy → uniqueness. Format and password-policy errors are returned together in one 422 (`CONTAINS_EMP_NO` is skipped when the employee ID is malformed); uniqueness is checked only when all fields are valid. **[Proposed]**

Acceptance:
- *Given* a registered employee ID, *when* the same ID registers again, *then* 409 `EMP_NO_TAKEN`.
- *Given* a successful registration, *then* no session cookie is set and the UI shows the login page.
- *Given* an email containing a space (`a b@x.com`), *then* 422 with field `email`.
### 6.4 Password policy
| Rule | Requirement | Rule id |
|---|---|---|
| Length | 12–64 code points | `LENGTH` |
| Character set | Printable ASCII only (U+0020–U+007E) | `CHARSET` |
| Complexity | At least 3 of 4 classes: upper, lower, digit, symbol (any printable non-alphanumeric ASCII, space included) | `COMPLEXITY` |
| Blocklist | Not in the bundled Top-10,000 common-password list (case-insensitive) | `COMMON_PASSWORD` |
| Keyboard runs | No run of 4+ adjacent keys on a keyboard row (`qwer`, `asdf`, `1234`), case-insensitive, forward or reverse | `KEYBOARD_RUN` |
| Sequences | No alphabetical or numeric sequence of 4+ (`abcd`, `4321`), case-insensitive, ascending or descending | `SEQUENCE` |
| Repeats | No character repeated 3+ times in a row | `REPEAT` |
| Personal data | Must not contain the employee ID | `CONTAINS_EMP_NO` |

- A violation returns 422 `PASSWORD_POLICY`; `errors[]` lists **every** violated rule (field `password`, `rule` = rule id), in table order.
- Storage: bcrypt, cost 10 only. Unknown, malformed or disabled accounts still run a bcrypt comparison against a dummy hash so response time does not reveal whether an account exists.

Rule details **[Proposed]**:
- Keyboard rows are the four rows of US QWERTY (``1234567890-=``, `qwertyuiop[]`, `asdfghjkl;'`, `zxcvbnm,./`). A shifted symbol counts as its key (`!@#$` is the same run as `1234`).
- Sequences do not wrap (`9012`, `xyza` are fine). One substring may violate several rules (`1234`). Blocklist: SecLists 10k (MIT licence), recorded next to the file.
### 6.5 Authentication and session
Mechanism (session filter, atomic validate-and-refresh, CSRF token format, cookie attributes, signing keys): `docs/Architecture/Backend.md` §6.

**Login**
- `POST /auth/login` with employee ID + password **in the JSON body only**.
- An unknown or badly formatted employee ID, a wrong password and a disabled account all return the same 401 `INVALID_CREDENTIALS`.
- A missing field, wrong JSON type or malformed JSON → 400 `MALFORMED_REQUEST`.

**Throttling** (counters live in MariaDB so all replicas share them: `login_lock`, `login_attempt`)
- Per submitted employee-ID string (counted even if the account does not exist; control characters stripped, cut to 32 chars): 5 consecutive failures, each within 15 minutes of the previous one, lock the key for a fixed 15 minutes → 429 `LOGIN_THROTTLED` with `Retry-After` (seconds).
- The 5th failure returns 401 and sets the lock; 429 starts with the 6th attempt. **[Proposed]**, OI-12.
- During a lock a correct password still returns 429 and does not extend the lock. A successful login clears the counter.
- Per source IP: 30 failures within a sliding 15-minute window → 429 `LOGIN_THROTTLED`.
- A throttled attempt does not run bcrypt and is not counted as an IP failure. **[Proposed]**
- `Retry-After` **[Proposed]**: key lock `locked_until − now`; IP limit the time until the oldest failure leaves the window; both apply: the larger; minimum 1 s.
- The client IP comes from `X-Forwarded-For`, trusting a configurable number of proxy hops (default 1). Admins are throttled the same way.

**Session**
- Sessions are server-side in MariaDB, in the project's own table storing only a SHA-256 of the session ID. **[Decided]** Implemented by an own session middleware (ADR-0001 G11).
- The cookie holds only an opaque session ID: `HttpOnly; Secure; SameSite=Lax; Path=/`.
- Idle timeout 30 minutes (refreshed by authenticated requests); absolute timeout 8 hours. Validation and idle refresh are one atomic operation using database time.
- **Refresh at most once per 60 seconds** **[Proposed]**, OI-11: `last_access_at` is written only when older than 60 s, so the effective idle timeout is 29–30 minutes.
- The account role is read from the account on every request, not copied into the session.
- A new session ID is issued at every login (no fixation). Several concurrent sessions per account are allowed.
- `POST /auth/logout` invalidates the session immediately and returns 204. It is idempotent and also returns 204 without a session, provided the CSRF check passes.
- `GET /auth/me` returns employee ID, name, email (null for Admin) and role; 401 without a valid session.
- All timeouts, lock windows and counters use database time.

**CSRF** (signed double-submit token, **bound to the session** **[Decided]**)
- `GET /auth/csrf` returns a token in the body and sets a matching cookie. Every write MUST send it in `X-CSRF-Token`. Mismatch or absence → 403 `CSRF_INVALID`.
- Accepted only if the cookie and header are both present and equal, the signature verifies, and the binding matches the request (session-bound token with a valid session, anonymous token without one).
- Effect: a token minted by an attacker, anonymously or for the attacker's own session, cannot be used against a victim's session.
- The token changes at login and logout; the client refetches it, and retries once on a stale token. A logout without a session still requires a valid anonymous token.
- The cookie is `__Host-` prefixed and HttpOnly; the front end reads the token from the response body. **[Proposed]**
### 6.6 Ideas
| Form field | API name | Rule |
|---|---|---|
| Category | `category` | `COOL_IDEA` or `OUTSTANDING_IMPACT`; required |
| Org | `org` | One of `AAID, BSID, ICSD, TSID, PLED, DEID`; required |
| ESG direction | `direction` | One of the five values below; required |
| Lead proposer | read-only | Taken from the session; any client-sent value is **ignored** |
| Idea name | `name` | 1–75 code points after clean-up; no `/ \ : * ? " < > \|`; expected filename ≤ 255 UTF-8 bytes |
| Idea summary | `summary` | 1–300 code points after clean-up |
| Slides | `slides` | One PDF or PPTX file, mandatory (§6.8) |

| Code | Label |
|---|---|
| `GREEN_MANUFACTURING` | Green Manufacturing |
| `RESPONSIBLE_SUPPLY_CHAIN` | Building a Responsible Supply Chain |
| `INCLUSIVE_WORKPLACE` | Creating a Healthy and Inclusive Workplace |
| `TALENT_DEVELOPMENT` | Talent Development |
| `CARING_FOR_DISADVANTAGED` | Caring for the Disadvantaged |

Official zh-TW labels and full Org names are to be supplied by the owner (OI-17).

Rules:
- Team size: 1 Leader + 0–3 Members (maximum 4). Members are **not** part of the create form; they join through join requests (§6.7).
- **Create** is one `multipart/form-data` request with all fields and the slides. It is atomic: there is no Idea without slides. Slide metadata (object key, extension, size, SHA-256, update time) is stored on the Idea row as non-null columns. **[Decided]**
- **Edit** (`PUT /ideas/{id}`, full replacement of category, org, direction, name, summary) is Leader-only. Slides change through the slides endpoint.
- Idea names are **not globally unique**, but a Leader cannot have two Ideas with the same `name` (case-sensitive, after clean-up) → 409 `IDEA_DUPLICATE`. The pair (leader employee ID, name) identifies an Idea for join requests. **[Decided]**
- **Delete** (dissolve) removes the Idea, its member and pending-request rows and its slides.
- If the expected filename exceeds 255 UTF-8 bytes, create/edit returns 422 `FILENAME_TOO_LONG`; length is computed with `.pptx` because the file type is unknown at edit time. **[Proposed]**
- Changing `name`, `org` or `category` after slides exist is allowed. The slide filename is checked only at upload; downloads use the *current* expected filename.
- An Idea is visible only to its Leader, approved Members and Admins; everyone else gets 404. Approved members are listed to everyone who can see the Idea; pending requests to the Leader and Admins only.

Idea list (`GET /ideas`):
- Paging: `page` (1-based, default 1), `size` (default 20, max 100). Below 1 → 422 `OUT_OF_RANGE`; non-numeric → 422 `INVALID_FORMAT`. A page beyond the last returns 200 with empty `items`. **[Proposed]**
- Filters: `org`, `category`, `direction` (invalid value → 422 `INVALID_ENUM`, never ignored); `leaderEmpNo`; `q` (case-insensitive "name contains", `%` and `_` escaped); `createdFrom`, `createdTo` (ISO 8601 instants with offset or `Z`, half-open `[createdFrom, createdTo)`; dates without time are rejected). **[Proposed]**
- Sort: `sort` = `createdAt` (default) or `name`; `order` = `asc` or `desc` (default `desc` for `createdAt`, `asc` for `name`); `id` is the final tie-breaker. Name ordering is by code point (`utf8mb4_bin`), not zh-TW collation (§10). **[Proposed]**
- Result: `{items, page, size, total}`. Users see their own Ideas (led or member of); Admins see all.
### 6.7 Team members and join requests **[Decided]**
- A membership record is an `idea_member` row with status `PENDING` or `ACCEPTED`. Reject, withdraw, remove and leave **delete the row**; history lives only in the Audit Log.
- **Apply:** `POST /join-requests` with `{ "leaderEmpNo", "ideaName" }`. The Idea is resolved by exact match of the Leader's employee ID and the cleaned-up Idea name. No match → 404 `NOT_FOUND`, identical to any other missing resource. The caller MUST be a User (Admin → 403 `FORBIDDEN`), MUST NOT be the Leader (422 `CANNOT_JOIN_OWN_IDEA`), MUST NOT already be a Member (409 `ALREADY_MEMBER`) and MUST NOT already have a pending request (409 `JOIN_REQUEST_EXISTS`). Precedence: §6.1.
- **Capacity:** approved members ≤ 3. Pending requests do not use capacity and are not capped; applying to a full team is allowed. Approving re-checks capacity inside the transaction under a row lock on the Idea → 409 `TEAM_FULL`.
- **Approve:** the Leader calls `POST /ideas/{id}/members/{empNo}/approve`. A row not `PENDING` → 409 `JOIN_REQUEST_NOT_PENDING`; no row → 404.
- **Reject or remove:** the Leader calls `DELETE /ideas/{id}/members/{empNo}` (works on `PENDING` and `ACCEPTED`). No row, including the Leader's own employee ID → 404. **[Proposed]**
- **Withdraw or leave:** the caller calls `DELETE /ideas/{id}/members/me` on their own row. A Leader cannot leave; they dissolve the team by deleting the Idea.
- After a reject or withdraw the User may apply again immediately. **[Proposed]**
- Applicants see their own pending requests with minimal Idea information (including the Idea ID needed to withdraw) through `GET /join-requests`. They cannot open the Idea or its slides: `GET /ideas/{id}` returns 404.
- A User may hold pending requests on several Ideas and be a Member of several.
- All these actions are allowed only in `OPEN`.
- A logged-in User who knows a Leader's employee ID and an Idea name can learn that the Idea exists. Accepted trade-off (§10).

Acceptance:
- *Given* an Idea with 3 approved members, *when* the Leader approves a fourth pending request, *then* 409 `TEAM_FULL`.
- *Given* a pending request, *when* the applicant calls `DELETE /ideas/{id}/members/me`, *then* 204, the row is gone and a new request is accepted.
### 6.8 Slides
- PDF or PPTX only; one file per Idea; maximum 5 MiB (5,242,880 bytes). The application enforces the exact boundary with a problem+json 413. Outer layers use looser limits (request 5 MiB + 64 KiB, ingress 6 MiB) and their 413 may not be JSON. **[Proposed: outer limits]**
- **Expected filename**, built by string concatenation and compared by string equality, never parsed with a regular expression:

```
{I|O}_{org}_{leaderName}_{leaderEmpNo}_{ideaName}.{pdf|pptx}
```

  - `I` for `COOL_IDEA`, `O` for `OUTSTANDING_IMPACT`.
  - Examples: `I_BSID_王小明_089290_成立供應商永續學院.pdf`, `O_AAID_莊育成_054616_AI秘密武器智能對話.pptx`.
  - The uploaded filename is NFC-normalized before comparison (macOS names are often NFD; this case MUST be in `testdata/`); the extension is compared case-insensitively.
  - The filename is compared exactly as sent: path components are not stripped, so a name carrying a path fails with `FILENAME_MISMATCH`. Multipart filenames are decoded as UTF-8. **[Proposed]**
- Validation, in this order:
  1. Exactly one file part; a missing or repeated `slides` part → 400 `MALFORMED_REQUEST`.
  2. Size → 413 `FILE_TOO_LARGE`.
  3. Not empty → 422 `EMPTY_FILE`.
  4. Magic bytes match and the extension agrees → 415 `UNSUPPORTED_FILE_TYPE`.
     - PDF: `%PDF-` at offset 0.
     - PPTX: a ZIP whose central directory contains `[Content_Types].xml` and `ppt/presentation.xml`; only the central directory is read. **[Proposed: `ppt/presentation.xml`]**
  5. Expected filename → 422 `FILENAME_MISMATCH`; the response carries the expected name.
  - Validation stops at the first failing step. When a text field is invalid, the file is not validated. No antivirus or macro scanning.
- **All text-field and file validation completes before the file is stored in MinIO**, so a rejected request never creates a storage object. **[Decided]**
- Leader and Admin can upload/replace; a replacement overwrites the previous file.
- Leader, approved Members and Admins can download.
  - Headers: `Content-Disposition: attachment` (RFC 6266 `filename*`), `Content-Length`, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`.
  - "View" means the Idea's text fields; there is no online preview.
  - `Range` is unsupported; `HEAD` returns 405 so it never writes a download audit row. **[Proposed]**
  - If storage fails after streaming has started, the connection is aborted. **[Proposed]**
- The backend streams the file; no pre-signed MinIO URLs. **[Decided]**
- A SHA-256 of every stored file is recorded. Deleting an Idea deletes its slides.

**Storage consistency** **[Decided]** (mechanism: `docs/Architecture/Sequence.md` §2, `Backend.md` §4.4)
- Every upload writes a **new object key** (`slides/{ideaId}/{uuid}.{ext}`, no personal data **[Proposed]**) before the write transaction; the transaction then switches the database pointer.
- An Idea never points at a missing file. When a transaction's outcome is **unknown** (for example the connection is lost around `COMMIT`), the new object is **not** deleted, because the commit may have succeeded.
- A request known to have rolled back leaves no unreferenced object: the object is deleted, or its key is recorded in `pending_file_deletion` with reason `UPLOAD_ABORTED`.
- A replaced or deleted object's key is recorded in `pending_file_deletion` (`REPLACED` / `DELETED`) **in the same transaction** as the change. Deleting the object is attempted after commit, so deleting an Idea succeeds even if MinIO is down.
- Remaining rows are drained by the orphan clean-up job (§9 #7).
- Create: if MinIO fails, the whole create fails and no Idea row exists. MinIO unavailable → upload and download return 503 `STORAGE_UNAVAILABLE`.
- An object left by a crash between upload and commit is not tracked in v1 (§10).
### 6.9 Deadline
- One global deadline in a single-row table.
- `GET /deadline` is **public**: `{ "deadline": "…Z" | null, "state": "NOT_OPEN|OPEN|CLOSED", "serverTime": "…Z" }`. The front-end countdown is corrected with `serverTime` and shown in the browser's time zone.
- `PUT /admin/deadline` (Admin only, CSRF) with `{ "deadline": "…" }`: ISO 8601 with offset or `Z`; any value is allowed, including the past. Stored in UTC at second precision (fractions truncated). **[Proposed]**
  - Missing or `null` → 400 `MALFORMED_REQUEST` (it cannot be cleared).
  - No offset → 422 `INVALID_FORMAT`.
  - Year outside 2000–2099 → 422 `OUT_OF_RANGE`.
  - The response is the same as `GET /deadline`.
- Every change is audited with `before` and `after` (including `null → value`).
### 6.10 Administration
- Set the deadline (§6.9); query the Audit Log through the API (§6.11).
- List, filter, search, view, download, replace slides of, and delete every Idea. Admins cannot create Ideas or edit text fields.
### 6.11 Audit Log
**Recorded events**
| `action` | Trigger | `outcome` values | before / after |
|---|---|---|---|
| `AUTH_REGISTER` | Registration | SUCCESS, FAILED | after: `empNo`, `name` (no email) |
| `AUTH_LOGIN` | Login | SUCCESS, FAILED, THROTTLED | – |
| `AUTH_LOGOUT` | Logout | SUCCESS | – |
| `IDEA_CREATE` / `IDEA_UPDATE` / `IDEA_DELETE` | Idea writes | SUCCESS | changed fields only; create includes slide metadata |
| `IDEA_VIEW` | **Admin** views a single Idea | SUCCESS | – |
| `SLIDES_REPLACE` | Slide replacement | SUCCESS | filename, size, SHA-256 (never content) |
| `SLIDES_DOWNLOAD` | Download by anyone | SUCCESS | – |
| `JOIN_REQUEST_CREATE` / `_APPROVE` / `_REJECT` / `_WITHDRAW`, `MEMBER_REMOVE`, `MEMBER_LEAVE` | Team actions | SUCCESS | membership change (the only history, as the row is deleted) |
| `DEADLINE_UPDATE` | Deadline change | SUCCESS | `before` / `after` |
| `ADMIN_SYNC` | Vault sync | SUCCESS, FAILED | admins added/disabled/updated, sessions revoked, skipped entries; actor = SYSTEM; written only by the replica whose update changed a row |
| `AUDIT_QUERY` | Audit Log query | SUCCESS | filters used |
| any of the above | Rejected attempt | **DENIED** | real reason (CSRF, role, state, not-yours, not-found) |

**Fields:** `id` (sequence, Admin-visible), `occurredAt` (database time, UTC), `requestId`, `actorEmpNo`, `actorRole` (`USER`, `ADMIN`, `ANONYMOUS`, `SYSTEM`), `action`, `outcome`, `targetType` (`USER`, `IDEA`, `DEADLINE` or null; team actions target the `IDEA` and carry the member's employee ID in `after`), `targetId`, `method`, `route` (template), `status`, `errorCode`, `ip`, `userAgent` (≤ 256 chars), `before`, `after`, and for failed logins the sanitized submitted employee-ID string. **[Proposed: value sets]**

**What counts as a rejected attempt [Proposed]** (OI-16)
- DENIED covers CSRF, role, state, not-yours and not-found.
- 404 on a single Idea is audited as DENIED for **both** "does not exist" and "not yours"; auditing only one would make their timing differ and break the uniform 404 (§7 rule 3).
- 409 and 422 are input errors and are **not** audited, except `AUTH_REGISTER` FAILED (written for 400, 409, 422), because a 409 there is the enumeration channel for employee IDs and emails. `after` holds the employee ID only if well-formed.
- Requests matching no route are not audited (metric only).

**Rules**
- Never record passwords, tokens, session IDs, CSRF values or file contents.
- **Two write tracks.** A successful write and its audit row are written in the **same transaction**; if the audit row cannot be written, the operation fails (fail-closed). A download is served only after its audit row is written. DENIED / FAILED / THROTTLED rows are written in a **separate transaction**, best effort; failures increment a metric. **[Decided]**
- Audit writing is synchronous; no message queue. **[Decided]**
- Unauthenticated 401 responses are not audited (metrics only). A throttled key is audited once per window.
- Ordinary list/read calls are not audited.
- Retention: at least 365 days; no automatic purge in v1. The table is append-only: the application contains no `UPDATE` or `DELETE` for it and an automated test enforces this (§10). Columns for the optional hash chain (§9 #5) are reserved and nullable.
- **Query:** `GET /admin/audit-logs`.
  - Filters: `from`, `to` (half-open), `actorEmpNo`, `action` (multi), `outcome`, `targetType`, `targetId`, `requestId`, `ip`.
  - Keyset pagination: `cursor`, `limit` (default 50, max 100), ordered by `occurredAt`, `id` descending.
  - The cursor is opaque (`base64url` of the microsecond `occurredAt` and `id`); an invalid cursor → 422 `INVALID_FORMAT`.
  - Response: `{items, nextCursor}`. **[Proposed]** No front-end UI.
### 6.12 API errors, API set and response bodies
Envelope and code table: `docs/API/error-codes.md`. Every endpoint with caller, success status and main errors: `docs/API/api-endpoints.md`. Bodies: `docs/API/response-bodies.md` **[Proposed]** (OI-26). Requirements:
- Errors are RFC 9457 `application/problem+json`. The stable contract is `code`; `detail` is English and not for display (the UI localizes by `code`).
- Every error carries a server-generated `requestId`, also in the response header; an incoming `X-Request-Id` is reused only if it is a valid UUID.
- 500 responses carry only `INTERNAL_ERROR` and `requestId`: no stack traces, SQL or paths. A malformed ID in a path is a 404, not a 400. Which error wins is fixed by §6.1.
- Every listed endpoint is implemented and covered by the generated OpenAPI document (§8.2). Operations endpoints (`/actuator/health/liveness`, `/readiness`, `/prometheus`) are served only on the separate management port (§8.3).
- A write returns the current state of the resource it changed. `pendingRequests` in the Idea detail is `null` (not `[]`) for callers who may not see it. Email addresses of other people are never returned.

## 7. Permission Matrix
`*` = allowed only in state `OPEN`; otherwise 403 `SUBMISSION_NOT_OPEN` / `SUBMISSION_CLOSED`.

| Operation | Visitor | User (no relation) | Member | Leader | Admin |
|---|---|---|---|---|---|
| csrf, register, login, logout, deadline | 200/201/204 | same | same | same | same |
| `me` | 401 | 200 | 200 | 200 | 200 |
| set deadline | 401 | 403 | 403 | 403 | 200 |
| create Idea | 401 | 201* | 201* | 201* | 403 |
| list Ideas | 401 | own | own | own | all |
| view Idea | 401 | 404 | 200 | 200 | 200 |
| download slides | 401 | 404 | 200 | 200 | 200 |
| edit text fields | 401 | 404 | 403 | 200* | 403 |
| replace slides | 401 | 404 | 403 | 200* | 200 |
| delete / dissolve Idea | 401 | 404 | 403 | 204* | 204 |
| apply to join (`POST /join-requests`) | 401 | 201* (404 if no Idea matches) | 409 | 422 | 403 |
| approve, reject, remove member | 401 | 404 | 403 | 2xx* | 403 |
| withdraw request / leave team | 401 | 404 (204* with own pending request) | 204* | 403 | 403 |
| my join requests | 401 | 200 | 200 | 200 | 200 (empty) |
| query Audit Log | 401 | 403 | 403 | 403 | 200 |

Note: a User with a `PENDING` request has no access to the Idea and counts as "User (no relation)" in every row except withdrawing their own request.

Rules:
1. Every permission is enforced on the server; hiding a button is never enough.
2. *Cannot see it → 404*; *can see it but not allowed → 403*; *does not exist → 404*; *not yours → 404*.
3. The 404 body, headers and response time are identical for "does not exist" and "not yours", including the audit write (§6.11).
4. The matrix is stored as `testdata/permission-matrix.json`; every cell is an automated test case.

## 8. Non-Functional Requirements
### 8.1 Grading rules
- The **Basic Requirements** (all of §3.1, including CI/CD, deployment and **backend line coverage ≥ 80 %**) and **every Functional Requirement** MUST be met.
- Of the NFR items in §8.2, **at least one** must be met. The team commits to four (API Spec, Structured Logging, Availability, Fault Tolerance). **[Decided]**
- Bonus features (§9) start only after the above are complete and verified.
### 8.2 NFR items
| Item | Requirement | Status |
|---|---|---|
| API Spec | OpenAPI 3.x (spec-first: `backend/api/openapi.yaml` is the source; server code and front-end types are generated from it), covering all endpoints, error responses and the CSRF header; validated in CI | Committed **[Decided]** |
| Structured Logging | JSON logs to stdout, every line has `requestId`; no passwords, session IDs or tokens (a test proves it) | Committed **[Decided]** |
| Fault Tolerance | MinIO down: delete succeeds, upload/download → 503 `STORAGE_UNAVAILABLE` | Committed **[Decided]** |
| Availability | 2 backend replicas; startup / liveness / readiness probes; sessions and throttle counters in the DB so any pod serves any request. Readiness depends on the database (and the first Vault read), **not** on MinIO. | Committed **[Decided]** |
| Reliability | `GET /ideas`, `/ideas/{id}`, `/deadline`, `/auth/me` sustain ≥ 100 RPS for 60 s on 10,000 Ideas, p95 < 300 ms, errors < 1 % (k6, after warm-up) | Target **[Proposed]** |
| Performance | Admin list p95 < 500 ms at 10,000 Ideas (page size 20); 5 MiB upload < 3 s on the intranet | Target **[Proposed]** |
| Monitoring | Prometheus metrics (requests/latency by route template, login failures, lockouts, upload bytes, storage errors, audit-write failures, deadline state) and a Grafana dashboard JSON in the repo | Not committed; depends on platform **[Open]** (OI-05) |
### 8.3 Security
- No secrets in source code; configuration comes from environment variables and platform-provided secrets.
- Passwords only as bcrypt hashes (cost 10); session IDs only as SHA-256 hashes.
- Every write is CSRF-protected with a session-bound token (§6.5); session cookie `HttpOnly; Secure; SameSite=Lax`.
- Slides are accessible only to the Idea's Leader, Members and Admins; the bucket is private and the backend streams downloads.
- Every query is parameterized; every list query has a server-enforced maximum page size.
- Debug and profiling endpoints are off; probes and metrics run on a separate management port that the public ingress does not route.
- Uniform 404 behaviour and dummy bcrypt work prevent enumeration by response or timing.
- Security headers on all responses **[Proposed]**: `Content-Security-Policy: default-src 'self'; frame-ancestors 'none'` (SPA); `Referrer-Policy: strict-origin-when-cross-origin`; `X-Content-Type-Options: nosniff`; HSTS at the ingress.
### 8.4 Quality
- Unit tests plus integration tests against a **real MariaDB** (testcontainers-go) with the **real migrations**; a test report is produced.
- Backend **statement** coverage ≥ 80 % (Go's coverage unit; measured with `go test -coverpkg=./...`), enforced as a CI gate on every build and pull request (a per-package report published); unit and integration results merged; only the main class and generated code excluded. **[Proposed]**
- Every permission-matrix cell has an automated test.
- Shared front-end/back-end test data in `testdata/` (text clean-up, employee ID, person name, email, password policy, expected filename, error codes), each with a JSON Schema in `testdata/schema/` validated in CI. **[Proposed]**
- An import-graph test, the Go compiler's `internal/` rule and `depguard` enforce module dependency direction, no access to another module's `internal/`, no ORM, and no `UPDATE`/`DELETE` on `audit_log` (`docs/Architecture/Backend.md` §2.4).
- Storage-consistency tests (MinIO down, lost-commit simulation, concurrent replace) per `docs/Architecture/Sequence.md` §7.
- One end-to-end smoke test against DEV: login → create Idea with slides → list → download → delete (`docs/PR-plan.md`, slice S3).
### 8.5 Compatibility and usability
- Primary browsers: current and previous major versions of Chrome and Edge; others best effort. **[Proposed]**
- Responsive layout, desktop first. Accessibility: WCAG 2.1 AA basics (labels, keyboard operation, contrast, readable errors; charts have a table alternative).
- UI is bilingual (zh-TW and en) through i18n dictionaries; errors are localized by `code`. Time is stored in UTC; the UI displays the browser's time zone.
- Upload UX: progress bar and client-side pre-check of filename and size; the back end remains the final judge.
- Names support CJK and emoji, counted in code points.
### 8.6 Platform and tech stack
| Area | Decision |
|---|---|
| Language | JavaScript, Python, C# and Rust are **not allowed**. Front end: **TypeScript only**. (Not enforced by CI.) |
| Architecture | **Modular monolith** in one Go module, packages by feature: `account`, `auth`, `idea`, `team`, `storage`, `slides`, `deadline`, `admin`, `audit`, plus a `shared` kernel that depends on no business module. Boundaries are enforced by the Go `internal/` rule, an import-graph test and `depguard`. No microservices, no message queue. Details: `docs/Architecture/Backend.md` §2. **[Decided]** |
| Backend | **Go 1.26** **[Decided]** (`docs/Adr/0001-Backend-in-go.md`): chi router, spec-first OpenAPI with oapi-codegen (standard non-strict server), **sqlc with module-owned hand-written SQL, no ORM**, `go-sql-driver/mysql`, Flyway migrations, own session / CSRF / throttling middleware, `log/slog`, `prometheus/client_golang`, minio-go, golangci-lint, testcontainers-go. Unknown JSON fields are ignored; JSON binding behaviour is locked by tests (§6.1). The earlier Java 21 + Spring Boot 4 choice is superseded. |
| Front end | TypeScript SPA, **React + Vite** **[Decided]**, same origin as the API. The public `site/` stays on VitePress. See `docs/Architecture/Frontend.md`. |
| Database | **MariaDB, version [Open]** (OI-04; `utf8mb4`). Version 10.8 is end of life; proposed **11.4 LTS** (maintained until 2029-05); required ≥ 10.7 for native `UUID`. Native `UUID` for public IDs, `INET6` for IPs, `DATETIME` in UTC. Enumerations are `VARCHAR` plus `CHECK`, never `ENUM`. Idea-name and email columns use `utf8mb4_bin`; name search is `LOWER(name) LIKE LOWER(?)` with escaped wildcards. Constraints are pushed into the database wherever it can enforce them. Details: `Backend.md` §10. |
| Object storage | MinIO (platform-provided) |
| Secrets/Admins | HashiCorp Vault (admin directory) |
| Monitoring | Prometheus + Grafana; JSON logs (ECS format **[Proposed]**) to stdout |
| Containers | Docker; registry: company Harbor |
| Orchestration | Company Kubernetes platform; personal K3S for pre-testing |
| Delivery | GitHub Actions CI, Helm chart, Argo CD (GitOps) with a **separate deployment repository** |
### 8.7 Environments and delivery
Environments: **Local** (docker-compose with MariaDB, MinIO, Vault dev mode or the file-based admin directory, §4.2); **CI** (testcontainers-go, real migrations); **personal K3S** (pre-testing); **company K8S DEV** (Argo CD, E2E smoke test). MariaDB and MinIO are platform-provided.

**Repositories** **[Decided]**: an application monorepo (`backend/`, `frontend/`, `docs/`, `site/`, `testdata/` with `schema/`) and a separate deployment repository (`chart/` Helm chart; `values/values-k3s.yaml`, `values-dev.yaml` holding the image tags), which is the GitOps source for Argo CD.

**Branching:** `main` is protected; feature branches merge by pull request; `dev` is auto-deployed to DEV.

**Pipeline (GitHub Actions):** PR → build, unit + integration tests, coverage gate (≥ 80 %), lint, OpenAPI validation, secret scan, image scan. Merge to `dev` → image tagged with the git SHA → push to Harbor → commit the tag to the deployment repository (so the application pipeline is not re-triggered) → Argo CD syncs → E2E smoke test.

**Helm + Argo CD**
- Argo CD renders the chart with `helm template` (no `--atomic`). Rollback = revert the Git commit in the deployment repository.
- Migration is a **Job** annotated as an Argo CD `PreSync` hook (delete policy `BeforeHookCreation`); the application does not run Flyway. Migrations MUST use **expand-then-contract**, so an application rollback needs no schema rollback.
- Chart: backend and front-end Deployments, Service, Ingress (`/` and `/api`), ConfigMap, migration Job, PodDisruptionBudget, optional ServiceMonitor.
- Pods run with `runAsNonRoot`, all capabilities dropped, seccomp `RuntimeDefault`, `readOnlyRootFilesystem` and an `emptyDir` on `/tmp` for upload spooling. **[Proposed]**
- The platform allows **one** MariaDB account without the `TRIGGER` privilege (§10).
### 8.8 Documents
All repo-internal documents are in English: this PRD (`docs/PRD.md`), `docs/Open-items.md`, `README.md`, API contract (`docs/API/`: `api-endpoints.md`, `error-codes.md`, `response-bodies.md`, generated `openapi.json`), architecture (`docs/Architecture/`), ADRs (`docs/Adr/`), `docs/PR-plan.md` (delivery slices and Definition of Done; tracked in beads), `docs/runbook.md` (deploy, rollback, Vault sync, MinIO outage), `docs/test-report.md` (coverage, permission-matrix results; also a CI artifact) and `docs/limitations.md`.

The **public site** (`site/`, VitePress, bilingual zh-TW / en) holds the API reference, architecture overview, user manual and other public information only. Internal infrastructure details (Vault paths, internal hostnames, runbook, ADRs) MUST NOT appear there.

## 9. Bonus Features
Start only after §8.1 is satisfied. Priority order:
| # | Feature | Notes |
|---|---|---|
| 1 | Admin statistics dashboard | `GET /admin/stats`: totals and share by org, category, direction, plus a daily trend; charts with an accessible table alternative. |
| 2 | Admin Excel export | `GET /admin/ideas/export` (.xlsx); list filters plus `createdFrom` / `createdTo`; employee ID written as text; all cells strings (never formulas); max 10,000 rows (`EXPORT_TOO_LARGE`); streamed; `Cache-Control: no-store`; audited as `IDEAS_EXPORT`. |
| 3 | Optimistic locking | `version` on the Idea, `ETag` on GET. `If-Match` is **optional** (absent = last write wins); mismatch → 412 `VERSION_CONFLICT`. Slide replacement also bumps the version. The column exists from the first migration. |
| 4 | Rate limit on join requests | 30 per minute per user (configurable), counters in MariaDB, 429 `RATE_LIMITED` with `Retry-After`; one audit row per window. |
| 5 | Audit Log hash chain | `prevHash` + `hash` (SHA-256) per row, ordered by `audit_log.id`; `GET /admin/audit-logs/verify` returns `valid`, `checked`, `firstBrokenId`. Tamper-*evident*, not tamper-proof. Sealing under concurrent writes is decided in an ADR when this starts (OI-30). |
| 6 | Pending-request badge | Front end polls and shows the number of pending join requests on the Leader's Ideas. Endpoint to be defined. |
| 7 | Orphan-file clean-up job | Drains `pending_file_deletion` per the reason rules of `Sequence.md` §2 and removes objects unreferenced for more than 24 h. Several replicas may run it by claiming rows with `SELECT … FOR UPDATE SKIP LOCKED`. |
| – | OpenTelemetry tracing | Deferred: needs a collector the platform has not confirmed. |

## 10. Known Limitations
- **Single database account, no `TRIGGER` privilege.** The runtime account also holds DDL rights, so an append-only Audit Log cannot be enforced by grants or triggers; it is enforced in application code and by an automated test. The hash chain (§9 #5) makes tampering detectable.
- **Cross-table rules cannot be enforced by the database** for the same reason (an Admin is never a Leader/Member/Applicant, a Leader cannot join their own Idea, capacity ≤ 3). They are enforced by the application, a row lock on the Idea, and tests.
- No off-site copy of the Audit Log; the hash chain does not protect against someone who can rewrite the whole table.
- Objects can remain in MinIO after a failed post-commit delete or an unknown transaction outcome (keys stay in `pending_file_deletion` until the clean-up job exists), or after a crash between upload and commit (not tracked in v1).
- An attacker can lock an account (including an Admin's) for 15 minutes by failing five logins.
- `POST /join-requests` lets a logged-in user who knows a Leader's employee ID and an Idea name confirm the Idea exists. Pending requests are not capped, so a Leader can be flooded. Both accepted.
- A write racing with an Admin deadline change may land on either side (§5).
- An oversized request body may get 413 before authentication, CSRF or state checks (§6.1).
- No password reset or 2FA: a forgotten password cannot be recovered by the user.
- The `Secure` cookie requires HTTPS outside `localhost` (the personal K3S needs a TLS ingress). Probing Idea IDs writes one DENIED audit row per request (no de-duplication in v1).
- Idea names sort by code point, not zh-TW stroke or Zhuyin order; MariaDB has no such `utf8mb4` collation.
- Login and registration use an anonymous (not session-bound) CSRF token, because no session exists yet; protection there relies on the `__Host-` cookie and `SameSite`.

## 11. Assumptions and Open Issues
Assumptions are inline **[Proposed]** rules in the sections they belong to or in the architecture documents. Open questions, owner confirmations and platform answers (Vault authentication, Argo CD access, credential delivery, MariaDB/MinIO versions, Prometheus/Grafana, hosting and ingress limits, CI runners, front-end libraries, Spring Boot 4 compatibility) are tracked only in `docs/Open-items.md` (OI-01 … OI-41). Old references of the form "§12 #n" map through `Open-items.md` §6.

## 12. Revision History
| Version | Changes |
|---|---|
| 1.0–1.4 | Drafts and review passes: join requests instead of invitations, session-bound CSRF, storage consistency around `pending_file_deletion`, edge-case rules marked **[Proposed]**, modular monolith with React + Vite and a separate deployment repository. |
| 1.5 | De-duplication: open issues moved to `docs/Open-items.md`, mechanism text to the architecture documents, API contract to `docs/API/`. |
| 1.6 | Compaction pass (~705 → ~480 lines); no requirement changed. |
| 1.7 | Backend language changed to Go (ADR-0001 accepted): §6.5, §8.2–§8.4, §8.6 rewritten; file names updated; delivery tracking moved to beads. Coverage unit is statement coverage. |

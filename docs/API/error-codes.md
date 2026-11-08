# API Errors and Error Codes
 
| | |
|---|---|
| Version | 1.1 |
| Date | 2026-11-08 |
| Companion of | `docs/PRD.md` v1.5 (requirements: §6.1 conventions and check order, §6.2 per-field errors), `docs/API/README.md` |
| Moved from | PRD v1.4 §6.12 |
| Status markers | **[Decided]** · **[Proposed]** awaiting confirmation (see `docs/Open-items.md`) |
 
Changes in 1.1: `REQUEST_TOO_LARGE` (413) added for JSON request bodies over 16 KiB — decided by the owner on 2026-11-08 (`Architecture.md` D60). `testdata/error-codes.json` and the zh-TW / en front-end dictionaries get the same entry in the pull requests that implement the error mapping (`docs/PR-plan.md` `B0-06`, `B0-07`, `F0-03`).
 
Machine-readable counterpart: `testdata/error-codes.json` (`{code, status}`). The front end generates a TypeScript union from it and requires zh-TW and en texts for every code (`Frontend.md` §4).
 
---
 
## 1. Envelope
 
Format: RFC 9457 `application/problem+json`.
 
```json
{
  "type": "about:blank",
  "title": "Unprocessable Content",
  "status": 422,
  "code": "FILENAME_MISMATCH",
  "detail": "Uploaded filename does not match the expected filename.",
  "requestId": "6f1c2c1e-…",
  "errors": [
    { "field": "slides", "code": "FILENAME_MISMATCH",
      "expected": "I_BSID_王小明_089290_成立供應商永續學院.pdf" }
  ]
}
```
 
## 2. Rules
 
- `code` is the stable contract; `detail` is English and not for display (the UI localizes by `code`).
- With several field errors, `errors[]` lists all of them in form-field order; the top-level `code` equals `errors[0].code` when that is a registered specific code, otherwise `VALIDATION_FAILED`.
- Generic field codes: `REQUIRED`, `INVALID_FORMAT`, `OUT_OF_RANGE`, `INVALID_ENUM`. Per-field rules (which error a field reports first) are in PRD §6.2. The password is the exception: `errors[]` lists every violated rule (PRD §6.4).
- `requestId` is a server-generated UUID. An incoming `X-Request-Id` is reused only if it is a valid UUID. It is returned in the response header and the error body.
- 500 responses carry only `INTERNAL_ERROR` and `requestId`: no stack traces, SQL or paths.
- A malformed ID in a path is a 404, not a 400.
- Which error wins when several apply is fixed by the check order in PRD §6.1: 401 → CSRF 403 → 404 → role 403 → state 403 → validation. A transport-level 413 may come first and may not be `problem+json` (PRD §6.1).
- A JSON request body over 16 KiB is answered with 413 `REQUEST_TOO_LARGE` by the body-limit middleware, before any other check and as `problem+json`. **[Decided]** (owner, 2026-11-08)
- 405 and 406 are also returned as problem+json. **[Proposed]** (OI-15)
## 3. Error codes
 
| HTTP | `code` | Situation |
|---|---|---|
| 400 | `MALFORMED_REQUEST` | Broken JSON, duplicate keys, wrong JSON type, missing or `null` required field, missing or repeated file part, not multipart (PRD §6.1) |
| 401 | `UNAUTHENTICATED` | No or expired session |
| 401 | `INVALID_CREDENTIALS` | Any login failure |
| 403 | `CSRF_INVALID` | CSRF token missing, wrong, or bound to another session |
| 403 | `FORBIDDEN` | Role not allowed |
| 403 | `SUBMISSION_NOT_OPEN`, `SUBMISSION_CLOSED` | State forbids the write |
| 404 | `NOT_FOUND` | Missing, not yours, malformed ID, unknown path (with a session), or no Idea matches the leader employee ID + name of a join request |
| 405 | `METHOD_NOT_ALLOWED` | Wrong method, including `HEAD` on slide downloads **[Proposed]** |
| 406 | `NOT_ACCEPTABLE` | Unsupported `Accept` **[Proposed]** |
| 409 | `EMP_NO_TAKEN`, `EMAIL_TAKEN` | Registration duplicate |
| 409 | `IDEA_DUPLICATE` | Same leader, same name |
| 409 | `ALREADY_MEMBER`, `JOIN_REQUEST_EXISTS`, `TEAM_FULL`, `JOIN_REQUEST_NOT_PENDING` | Team state conflict |
| 412 | `VERSION_CONFLICT` | *(bonus)* `If-Match` mismatch |
| 413 | `FILE_TOO_LARGE` | Over 5 MiB (may be answered by the transport layer before any other check, PRD §6.1) |
| 413 | `REQUEST_TOO_LARGE` | JSON request body over 16 KiB; always `problem+json` **[Decided]** |
| 415 | `UNSUPPORTED_FILE_TYPE`, `UNSUPPORTED_MEDIA_TYPE` | Slide content type / request content type |
| 422 | `VALIDATION_FAILED` | Generic field validation |
| 422 | `PASSWORD_POLICY`, `INVALID_NAME_CHARS`, `FILENAME_TOO_LONG`, `FILENAME_MISMATCH`, `EMPTY_FILE` | Specific validation |
| 422 | `CANNOT_JOIN_OWN_IDEA` | Join request validation |
| 422 | `EXPORT_TOO_LARGE` | *(bonus)* more than 10,000 rows |
| 429 | `LOGIN_THROTTLED` | With `Retry-After` |
| 429 | `RATE_LIMITED` | *(bonus)* join-request rate limit, with `Retry-After` |
| 500 | `INTERNAL_ERROR` | Unexpected |
| 503 | `STORAGE_UNAVAILABLE`, `SERVICE_UNAVAILABLE` | MinIO / database unavailable, or a write whose outcome is unknown |
 
Where a code is raised: the validation rules are in PRD §6.2–§6.4, §6.6–§6.9; the state codes in PRD §5; the throttling codes in PRD §6.5; the storage and unknown-outcome cases in PRD §6.8 and `Sequence.md` §2.
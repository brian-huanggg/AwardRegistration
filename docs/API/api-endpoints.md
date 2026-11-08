# API Set
 
| | |
|---|---|
| Version | 1.0 |
| Date | 2026-11-08 |
| Companion of | `docs/PRD.md` v1.5 (requirements: §6, §7 permission matrix), `docs/API/README.md` |
| Moved from | PRD v1.4 §6.13 |
| Status markers | **[Decided]** · **[Proposed]** awaiting confirmation (see `docs/Open-items.md`) |
 
Base path `/api/v1`. The front end is served from the same origin, so no CORS is needed (PRD §6.1). Schemas are in the OpenAPI snapshot; error codes in `error-codes.md`; response bodies in `response-bodies.md`. Who may do what in every state is the permission matrix in PRD §7 (`testdata/permission-matrix.json`).
 
Every write (`POST`, `PUT`, `DELETE`, including login, register and logout) MUST send `X-CSRF-Token` (PRD §6.5).
 
| Endpoint | Callable by | Success | Main errors |
|---|---|---|---|
| `GET /auth/csrf` | everyone | 200 | – |
| `POST /auth/register` | everyone | 201 | 409 `EMP_NO_TAKEN`/`EMAIL_TAKEN`; 422 `PASSWORD_POLICY`/`VALIDATION_FAILED` |
| `POST /auth/login` | everyone | 200 | 401 `INVALID_CREDENTIALS`; 429 `LOGIN_THROTTLED` |
| `POST /auth/logout` | everyone | 204 | 403 `CSRF_INVALID` |
| `GET /auth/me` | User, Admin | 200 | 401 |
| `GET /deadline` | everyone | 200 | – |
| `POST /ideas` (multipart) | User | 201 | 403 `SUBMISSION_*`; 409 `IDEA_DUPLICATE`; 413/415/422; 503 |
| `GET /ideas` | User, Admin | 200 | 422 (`size` > 100, invalid filter) |
| `GET /ideas/{id}` | Leader, Member, Admin | 200 | 404 |
| `PUT /ideas/{id}` | Leader | 200 | 403 `SUBMISSION_*`; 409; 422; *(bonus)* 412 |
| `DELETE /ideas/{id}` | Leader, Admin | 204 | 403 (Leader is state-limited) |
| `PUT /ideas/{id}/slides` | Leader, Admin | 200 | 413/415/422 `FILENAME_MISMATCH`; 503 |
| `GET /ideas/{id}/slides` | Leader, Member, Admin | 200 | 404; 405 (`HEAD`); 503 |
| `POST /join-requests` | User | 201 | 403 `SUBMISSION_*`; 404 `NOT_FOUND`; 409 `ALREADY_MEMBER`/`JOIN_REQUEST_EXISTS`; 422 `CANNOT_JOIN_OWN_IDEA`/`VALIDATION_FAILED`; *(bonus)* 429 `RATE_LIMITED` |
| `GET /join-requests` | User, Admin (empty) | 200 | – |
| `POST /ideas/{id}/members/{empNo}/approve` | Leader | 200 | 403 `SUBMISSION_*`; 404; 409 `TEAM_FULL`/`JOIN_REQUEST_NOT_PENDING` |
| `DELETE /ideas/{id}/members/{empNo}` | Leader | 204 | 403 `SUBMISSION_*`; 404 |
| `DELETE /ideas/{id}/members/me` | Member, Applicant | 204 | 403 `SUBMISSION_*`; 404 |
| `PUT /admin/deadline` | Admin | 200 | 400; 403; 422 |
| `GET /admin/audit-logs` | Admin | 200 | 422 |
| *(bonus)* `GET /admin/stats`, `GET /admin/ideas/export`, `GET /admin/audit-logs/verify` | Admin | 200 | 403 |
| Operations: `/actuator/health/liveness`, `/readiness`, `/prometheus` | cluster only (separate management port) | 200 | – |
 
Notes:
 
- `DELETE /ideas/{id}/members/{empNo}` works on both `PENDING` and `ACCEPTED` rows (reject or remove); `DELETE …/members/me` works on the caller's own row (withdraw or leave). `me` is never a 6-digit employee ID, so the two paths cannot collide (PRD §6.7).
- List endpoints enforce a maximum page size of 100 (`GET /ideas`: `page`, `size`, filters, `sort`, `order`; PRD §6.6). `GET /admin/audit-logs` uses keyset pagination with `cursor` and `limit` (PRD §6.11).
- Unknown paths: 401 without a session, 404 with one; wrong method 405; unsupported `Accept` 406 (PRD §6.1, OI-15).
- Operations endpoints are on the management port and are not routed by the public ingress (PRD §8.3).
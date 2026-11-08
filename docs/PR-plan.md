# PR Split Plan — Go Backend

| | |
|---|---|
| Version | 0.4 (Go DoD recorded in §1 per D-02; owner-confirmed; frontend routing changed: no `/admin/ideas`, single login entry; §7 "72-hour budget" still unconfirmed) |
| Date | 2026-11-08 |
| Requirements | `docs/PRD.md` v1.5, `docs/API/`, `docs/Open-items.md`, `docs/Architecture/Frontend.md` v0.5 |
| Tech baseline | `docs/Architecture/Backend.md` v0.1, `docs/Adr/0001-Backend-in-go.md` (**accepted**; A1–A3 decided or tracked as OI-50–OI-52; O1–O3 tracked as OI-53–OI-55) |
| Tracking | **beads (`bd`)** is the only task tracker. Linear is no longer used; the earlier Linear projects are archived and nothing is read from them. This file is the plan, not a task list: work items live in `bd` |

## 0. How to read this
- One row = **one PR = one beads issue** (`bd create`). Slices (S0–S7) become beads **epics**; each PR issue is a child of its epic.
- "Verify" is how a reviewer judges the PR done; if you can't write one, the PR is too big.
- "Deps" become beads dependencies (`bd dep add <issue> <blocker>`); rows without deps can run in parallel.
- ID prefix: `D` docs/decisions, `B` backend, `F` frontend, `O` devops, `Q` test/acceptance; number = slice (`B0-` = S0 backend).
- **Execution order follows §7 priorities (P0 → P3), not table order.**

## 1. PR rules
**Size/scope:** one thing per PR (one endpoint, middleware, table+queries, or page); hand-written code ≤ 400 lines (excluding sqlc/oapi-codegen output, `testdata/`), exceptions justified in the description; doable in 1–2 days; `main` always deployable (unfinished routes unregistered or `x-internal`).

**PR DoD:**
- CI green: `golangci-lint` (incl. `depguard`), `go test -race` (unit + `integration` tag), statement coverage ≥ 80 % (from the first code PR). Coverage is measured with `go test -coverpkg=./...`, unit and integration results merged; generated code (sqlc, oapi-codegen) is excluded from the denominator (OI-54).
- Integration tests run on **testcontainers-go against a real MariaDB with the real Flyway migrations** (no H2/SQLite/mocks for SQL); a new migration comes with a test that exercises it.
- Import-graph test (`test/arch`) green: module dependency direction, no access to another module's `internal/`, `shared` imports no module, no ORM, no `UPDATE`/`DELETE` on `audit_log`.
- `sqlc generate`, `go generate` (oapi-codegen) and frontend type generation produce no diff (CI runs them and fails on `git diff`).
- New behavior tested; touched permission-matrix cells added to `testdata/permission-matrix.json` with automated tests.
- New error codes synced: catalogue, `testdata/error-codes.json`, `docs/API/error-codes.md`, zh-TW/en dictionaries.
- New audit events tested (track A SUCCESS; track B DENIED/FAILED/THROTTLED).
- Matching `docs/modules/*.md` updated in the same PR ([Planned] → real paths); see §7.7 for 72-hour mode.
- PR description includes evidence from "Verify" (test names, screenshots, curl output).

**Slice DoD:** at milestone end every PR of the slice meets the PR DoD above, the slice's "Verify" rows pass, and the open items that gate the slice (`docs/Open-items.md`) are answered.

## 2. Beads structure
- Epics: M0 Decisions, M1 S0 Skeleton, M2 S1a Identity, M3 S1b Password & throttling, M4 S2 Admin & deadline, M5 S3 Idea core, M6 S4 Edit & delete, M7 S5 Team, M8 S6 Admin ops & audit, M9 S7 Hardening & acceptance, M10 Bonus.
- Priority: P0 = 0 (critical), P1 = 1 (high), P2 = 2 (medium), P3 = 3. Labels: `backend`/`frontend`/`devops`/`docs`/`test`; size `S` (≤ half day) / `M` (1–2 days), no L (split it); `blocked-by-OI`. No estimates.
- Create issues for P0, P1, P2, prep and wrap-up when work starts. P3 and Bonus stay in this file until needed. Put the PR ID (e.g. `B0-02`) at the start of the issue title so this file and `bd` stay cross-referenced.
- Day-to-day commands: `bd ready`, `bd update <id> --claim`, `bd close <id>`; see `AGENTS.md`.
- Branch names contain the PR ID; PR titles are prefixed with it.

## 3. PR list
### M0 — Decisions & docs
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| D-01 | Adopt ADR-0001 (Go). ADR is Accepted (2026-11-08). Left: PRD §8.1/8.2/8.4/8.6 to Go terms; `Backend.md` status and `Open-items.md` updated; A1–A3 into ADR after owner confirms | No "Go is not adopted" left in docs | – | S |
| D-02 | Go DoD recorded in §1 (Flyway on testcontainers-go, oapi-codegen diff, import-graph test, statement coverage); keep §1 and PRD §8.4 in sync | Review | D-01 | S |
| D-03 | open-items: Go OIs — A3(a) sqlc/MariaDB, A3(b) JSON body limit, A3(c) minio-go retry; O1–O3; `shared.md` §12 gaps 1–4 | Each has OI number, default, owner | D-01 | S |

### M1 — S0 Walking skeleton
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| O0-01 | Monorepo (`backend/ frontend/ docs/ site/ testdata/schema/`), `go.mod` toolchain pin, `Makefile`, `.golangci.yml` (depguard bans ORM, gosec, errcheck) | `make lint test` passes on empty `main.go` | D-01 | S |
| O0-02 | docker-compose: MariaDB 11.4, MinIO, Flyway; `make dev-up`/`migrate` | Fresh clone up in 3 commands | O0-01 | S |
| B0-01 | V001: `contest_setting` (single row + `CHECK`), `audit_log` skeleton | Migrate OK; rerun no-op | O0-02 | S |
| B0-02 | Spike: sqlc vs MariaDB (A3a) — `UUID`/`INET6` overrides; test `CHECK`, `ON DUPLICATE KEY UPDATE`, `SKIP LOCKED` | ADR records what works/needs hand-written SQL | B0-01 | S |
| B0-03 | `shared/config` + `cmd/server` (public + management servers, graceful SIGTERM) | Unit test; readiness false before shutdown | O0-01 | M |
| B0-04/O0-06 | Probes: liveness/readiness (DB yes, MinIO no); path per O3; Backend image: `CGO_ENABLED=0`, distroless nonroot, `GOMEMLIMIT` | DB down → readiness 503; Runs with read-only rootfs | B0-03, B0-04 | M |
| B0-05 | Request ID (valid UUID only), recover, ECS JSON access log, `test/logging` | Non-UUID replaced; panic → 500; every log line has `requestId` | B0-03 | M |
| B0-06 | problem+json, `ErrorCode` catalogue, single problem writer; 500 body = `INTERNAL_ERROR` + `requestId`; `testdata/error-codes.json` + schema | Catalogue == JSON; 500 leaks nothing | B0-05 | M |
| B0-07 | chi router transport errors: 404/405/406/415/413 as problem+json (A2) | One handler test each | B0-06 | M |
| B0-08 | Strict JSON decoder (O1): duplicate keys, trailing data, exact-case names, missing/null/type → 400, unknown fields ignored; built with first JSON endpoint | `test/binding`: one test per `Backend.md` §3 row | B0-06 | M |
| B0-09 | testcontainers-go base + `DbClock`; DSN (`parseTime`, `loc=UTC`, `clientFoundRows`) | Session TZ is UTC; `UPDATE` returns matched rows | B0-01 | M |
| B0-10 | `shared/db.InTx` + Outcome: READ COMMITTED, `WithoutCancel`+timeout (G8), Committed/RolledBack/Unknown (G7), 1205/1213 → 503, `IP` type | Unit per failure point; lock-wait rolls back whole tx | B0-09 | M |
| B0-11 | `test/arch`: import-graph rules; `shared` can't import modules; scan `audit_log` UPDATE/DELETE | Deliberate violation turns CI red | O0-01 | S |
| B0-12 | OpenAPI skeleton + oapi-codegen (non-strict chi, A1); binding errors (bad UUID → 404, `page=abc` → 422); Spectral (writes declare `X-CSRF-Token`, problem+json) | `go generate` no diff; Spectral passes | B0-07 | M |
| B0-13 | `test/apispec`: validate every integration response with `kin-openapi` | Wrong field fails test | B0-12 | S |
| B0-14 | `GET /deadline` → `{deadline:null,state:NOT_OPEN,serverTime}`; ms output helper (truncate vs round per D-03) | Integration test; matches spec | B0-10, B0-12 | S |
| O0-03 | `make ci` (lint, unit, integration, `-race`, merged coverage, 80 % gate) + GitHub Actions workflow (`.github/workflows/ci.yml`) | Coverage-dropping PR blocked | O0-01 | M |
| O0-04/05 | Pipeline: sqlc/oapi/frontend-type diff, Spectral, `testdata/` schema checks; Pipeline: gitleaks, govulncheck, image scan | Each has a failing demo; Fake secret blocked | O0-03, B0-12, O0-03 | M |
| O0-07 | Helm backend: 2 replicas, probes, hardened securityContext, PDB, ConfigMap, `values-k3s/dev` | `helm lint` + K3S deploy | O0-06 | M |
| O0-08 | Helm: Flyway PreSync Job; Ingress `/`, `/api`, 6 MiB, TLS | `curl …/api/v1/deadline` → 200 | O0-07 (OI-06) | S |
| O0-09 | CD to DEV: merge `dev` → Harbor (git SHA) → deploy repo → Argo CD → smoke | No manual step | O0-08 (OI-02) | M |
| F0-01 | Frontend skeleton: Vite+React+strict TS, ESLint (boundaries, jsx-a11y), no `.js`, Vitest | `npm run lint test build` pass | O0-01 (OI-24) | S |
| F0-02 | `shared/api`: types from openapi, problem+json parsing, MSW | Unit per problem type | F0-01, B0-12 | M |
| F0-03 | i18n zh-TW/en; error-code completeness check vs `error-codes.json` | Missing translation fails | F0-01, B0-06 | S |
| F0-04 | Layout, router, deadline banner; **single route table with access (`public`/`USER`/`ADMIN`/`ANY`)**; nav from table+role; shared 404 (wrong role = unknown route) | MSW 3 states; route-access test; axe clean | F0-02, F0-03 | S |
| F0-05 | Frontend image (nginx, SPA fallback, cache, CSP) + Helm | Banner visible on DEV (**S0 exit**) | F0-04, O0-09 | M |

### M2 — S1a Register, login, session, CSRF
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B1-01 | `shared/text`: NFC, 25 `White_Space` code points, code-point count, name/summary cleanup, per-field error order; `testdata/text-cleanup.json` | Contract test (NFD, emoji, U+3000) | B0-06 | M |
| B1-02 | `shared/security`: `Principal`, `Role`, `ActorRole`, ctx accessors | Unit | B0-03 | S |
| B1-03 | Audit write: full `audit_log` migration; `Record(ctx,tx,e)` / `RecordRejected(ctx,e)` (G9); sanitize/truncate | Track A rolls back with tx; track B persists | B0-10, B1-02 | M |
| B1-04 | Central error adapter → track B (`errors.As` on `AuditReason`) | With reason → DENIED row; without → none | B1-03 | S |
| B1-05 | `app_user` + field rules (email `CHECK`, `utf8mb4_bin`; emp no/name/email per OI-14) | Contract test | B1-01, B0-02 | M |
| B1-06 | `POST /auth/register`: format → password stub → uniqueness; 409 `EMP_NO_TAKEN`/`EMAIL_TAKEN`; no cookie; `AUTH_REGISTER` audit | Integration + audit tests | B1-04, B1-05, B0-08 | M |
| B1-07 | Session store: 32-byte ID, SHA-256 only; atomic validate+touch (60 s rule, OI-11); revoke by user | Idle 30 min, absolute 8 h, disabled user, matched rows | B1-05 | M |
| B1-08 | Session middleware: public list; else 401; principal in ctx; unknown path: no session 401, session 404 | All combinations | B1-07, B1-02 | S |
| B1-09 | CSRF: `GET /auth/csrf`; session-bound HMAC (D40); `__Host-csrf`; after session, no body read; key rotation | Missing/mismatch/other-session/anonymous token → 403 `CSRF_INVALID` | B1-08 | M |
| B1-10 | `POST /auth/login`: dummy bcrypt (cost 10), >72 bytes rule (G3), new ID per login, `AUTH_LOGIN`; throttle hook | Unknown vs wrong password timing similar; old CSRF invalid | B1-09 | M |
| Q1-01 | Permission-matrix harness (`test/matrix`); auth rows; order 401 → CSRF → … | All rows green | B1-11 | M |
| B1-11/12 | Logging safety: no password/session ID/CSRF/Cookie in logs; `POST /auth/logout` (idempotent 204) + `GET /auth/me`; `AUTH_LOGOUT` | `test/logging` for login/register; Integration | B1-10 | M |
| F1-01 | Frontend `shared/text` using same `text-cleanup.json` | Contract test | F0-01, B1-01 | S |
| F1-02 | CSRF in memory; refetch on login/logout/401; retry once on `CSRF_INVALID`; global 401 → `/login?next=` | MSW unit | F0-02, B1-09 | M |
| F1-03 | Register page: labels, `autocomplete`, error summary, email precheck; success → login with navigation-state notice | MSW + axe + keyboard | F1-01, F1-02 | M |
| F1-04 | Login/logout, **shared by Admin and User** → `/ideas`; logged-in visits to `/login`,`/register` redirect; `next` only same-site relative and must resolve to a role-allowed route else `/ideas`; Admin badge | Local register→login→me→logout (**S1a exit**); `next=//evil.example`, `next=/admin/deadline` (User) → `/ideas` | F1-02 | M |
| F1-05 | Idle timeout warning at 28 min; "continue" calls `GET /auth/me` (OI-22) | Fake timers | F1-04 | S |

### M3 — S1b Password policy & login throttling
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B1b-01 | Password policy framework (PRD §6.4), simple rules, list all violations in table order; wire into register | `testdata/password.json` contract | B1-06 | M |
| B1b-02/03 | Keyboard-sequence rule (QWERTY rows incl. shift, no wrap, overlaps); Top-10,000 common passwords (`go:embed` SecLists; NOTICE for source/license) | Table-driven; Contract | B1b-01 | M |
| B1b-04 | `login_lock`: 5 failures/15 min → fixed 15 min lock; correct password during lock → 429; 5th failure → 401 (OI-12) | Integration (DB time) | B1-10 | M |
| B1b-05 | `login_attempt` IP sliding window: 30/15 min; configurable `X-Forwarded-For` trust depth | Integration | B1b-04 | M |
| B1b-06 | 429 + THROTTLED audit: `Retry-After` = larger of two, ≥ 1 s; no bcrypt/attempt when throttled; one THROTTLED row per window | Tests + timing | B1b-05 | S |
| B1b-07 | Cleanup job: idempotent batch delete of sessions/throttle rows; ticker+jitter; stops on shutdown | Two runners don't conflict | B1b-05 | S |
| F1b-01 | Live password checklist; lazy-loaded Top-10k chunk | Unit; bundle screenshot | F1-03, B1b-03 | M |
| F1b-02 | 429 countdown from `Retry-After` | Fake timers | F1-04, B1b-06 | S |

### M4 — S2 Admin directory, deadline, SubmissionGate
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B2-01 | `AdminDirectory` interface + file impl (local/test); reject bcrypt cost ≠ 10 (OI-13) | Unit | B1-05 | S |
| B2-02 | AdminSync: first read at startup (readiness `atomic.Bool`), every 60 s + jitter, conditional update, only the changing replica writes `ADMIN_SYNC`, skip User conflicts, re-enable on rejoin, keep Admins on sync failure (OI-40) | Two replicas concurrently; Admin emp no register → 409 | B2-01, B1-04 | M |
| B2-03 | `SessionRevoker` (declared in `account`, impl in `auth`, wired in `main.go`); revoke on hash change/disable | Removed Admin's next request → 401 | B2-02, B1-07 | S |
| B2-04 | `SubmissionGate`: `Precheck`, `AssertOpenInTx`; Admin exempt (OI-18); **seed an `OPEN` deadline via migration/config locally/DEV** until `PUT` exists | NOT_OPEN/OPEN/CLOSED tests | B0-14, B0-10 | S |
| B2-05 | `PUT /admin/deadline`: ISO 8601 with offset, second precision; null/missing → 400; no offset → 422; years 2000–2099; `DEADLINE_UPDATE` before/after | Integration + matrix row | B2-04, B2-02 | M |
| B2-06 | Vault impl (`hashicorp/vault/api`, auth per OI-01) | Integration vs Vault dev container | B2-01 | M |
| F2-01 | `/admin/deadline` page (Admin only; User sees 404 page) | MSW + axe | F1-04, B2-05 | M |
| F2-02 | Countdown using `serverTime` + `performance.now()`; coarse `aria-live` | Fake timers; Admin opens/closes submissions (**S2 exit**) | F0-04 | S |

### M5 — S3 Create, view, list, download Idea (core path)
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B3-01 | `idea` migration/queries: NOT NULL slide cols, unique `(leader_id,name)`, `version`, indexes; **single visibility query** | Visibility per role | B1-05 | M |
| B3-02 | MinIO port (`minio-go`, 2 s dial, 30 s call, **retry off** A3c; put/get/delete) | Integration; MinIO down → `STORAGE_UNAVAILABLE` | B0-09 | M |
| B3-03 | SlideValidator order: parts → size → empty → magic bytes (PPTX needs `ppt/presentation.xml`) + extension → filename | Table-driven | B0-06 | M |
| B3-04 | ExpectedFilename generator (`.pptx` length rule); `testdata/expected-filename.json` (NFD) | Contract | B1-01 | S |
| B3-05 | Multipart spooling: `MultipartReader()` streaming, temp file + `MultiWriter(file,sha256)`, deferred delete; missing/duplicate part → 400 | Temp file removed on success and failure | B0-07 | M |
| B3-06/07 | `pending_file_deletion` (`REPLACED`/`DELETED`/`UPLOAD_ABORTED`); Lost-commit tooling: wrap `driver.Connector` ("fail after commit" / "fail without commit"); one Toxiproxy test | Integration; `InTx` yields `Unknown` / `RolledBack` | B3-02, B0-10 | M |
| B3-08 | `POST /ideas`: validate text → duplicate precheck → validate file → upload → tx (`AssertOpenInTx` + insert + `IDEA_CREATE`); 201 + `Location` + IdeaDetail | Integration + matrix row | B3-01, B3-03, B3-04, B3-05, B2-04 | M |
| B3-09 | Create compensation: known rollback → delete object now (else `UPLOAD_ABORTED`); `Unknown` → keep, record `UPLOAD_ABORTED` (OI-19) | Lost-commit tests green | B3-08, B3-06, B3-07 | M |
| B3-10 | `GET /ideas/{id}` via visibility query; `expectedFilenameBase`; `pendingRequests` for Leader/Admin only; missing vs forbidden → identical 404 + DENIED audit | Both 404s identical in body/headers | B3-01, B1-04 | S |
| B3-11 | `GET /ideas` paging/filters: page/size (>100 → 422), `INVALID_ENUM`, time range `[from,to)` | Integration | B3-01 | M |
| B3-12 | List search/sort: escaped `LOWER() LIKE`, `sort`+`order` with `id` tie-break, `myRelation` | `%` and `_` not wildcards | B3-11 | S |
| B3-13 | `GET /ideas/{id}/slides`: commit audit before streaming; `Content-Length`, `filename*`, `no-store`, `nosniff`; `HEAD` 405; MinIO down 503; abort on mid-stream failure | `Sequence.md` §7 download tests | B3-10, B3-02 | M |
| F3-01 | Create form: code-point counts, expected filename + copy, precheck | MSW + axe (OI-17, OI-21) | F1-04 | M |
| F3-02 | Upload progress via XHR; handle 413/415/422/503 | MSW | F3-01, B3-08 | S |
| F3-03 | List page **shared by User (own) and Admin (all)**; filters/paging in URL; sort/search after B3-12 | MSW; state survives refresh | F1-04, B3-11 | M |
| F3-04 | Detail page + download (**shared**); fetch → blob with size check | MSW | F3-03, B3-13 | S |
| Q3-01 | Playwright smoke: login → create → list → download (delete added after B4-03); plain User, secret password, Idea name with git SHA, pre-clean, fail clearly if deadline not `OPEN`; compose first, DEV after O0-09 | One-command local pass (**S3 exit**) | F3-04 | M |

### M6 — S4 Edit, replace slides, delete
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B4-01 | `PUT /ideas/{id}`: Leader only, full replace; `FILENAME_TOO_LONG`, `IDEA_DUPLICATE`; `IDEA_UPDATE` logs changed fields | Integration + matrix | B3-10 | M |
| B4-02 | `PUT /ideas/{id}/slides`: validate vs current idea; lock row → swap pointer → `REPLACED` → `version`+1 → `SLIDES_REPLACE`; delete old key post-commit | Concurrent replace leaves no orphan | B3-09 | M |
| B4-03 | `DELETE /ideas/{id}`: lock → delete (members cascade) → `DELETED` → audit → delete file post-commit; 204 | Integration + matrix | B3-06, B3-10 | S |
| Q4-01 | Fault tolerance: MinIO down → delete 204, upload/download 503; lost-commit cases; one Toxiproxy | Green + MinIO-stop demo (**S4 exit**) | B4-02, B4-03 | M |
| F4-01 | Edit page (reuse F3-01 form) | MSW + axe | F3-04, B4-01 | S |
| F4-02 | Replace-slides control + delete confirm dialog | MSW + keyboard; smoke adds delete | F3-04, B4-02, B4-03 | M |

### M7 — S5 Team & join requests
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B5-01 | `idea_member` migration/queries: PK `(idea_id,user_id)`, cascade, index `(user_id,status)`, `accepted_at` `CHECK` | Integration | B3-01 | S |
| B5-02 | `POST /join-requests`: lookup by `(leaderEmpNo, ideaName)` (name cleaned), body validated first; 404/409/422/403 (Admin); full team may still apply | Integration + matrix | B5-01, B2-04 | M |
| B5-03 | `GET /join-requests`: slim Idea info (with `ideaId`); Admin gets `[]` | Integration | B5-01 | S |
| B5-04 | Approve: lock Idea row → member rows; `TEAM_FULL`, `JOIN_REQUEST_NOT_PENDING`; set `accepted_at` | Concurrent approvals never exceed 3 | B5-02 | M |
| B5-05/06 | Reject/remove: `DELETE …/members/{empNo}`; Leader's own emp no → 404; `MEMBER_REMOVE`/`JOIN_REQUEST_REJECT`; Withdraw/leave: `DELETE …/members/me` (applicants too); `MEMBER_LEAVE`/`JOIN_REQUEST_WITHDRAW` | Integration + matrix; Integration + matrix (applicant case) | B5-02 | M |
| F5-01 | `/join-requests` page: apply, my requests, withdraw; User only (Admin sees 404 page) | MSW + axe | F1-04, B5-03 | M |
| F5-02 | Member management on detail page: approve, reject, remove, leave | MSW + axe | F3-04, B5-04 | M |

### M8 — S6 Admin Idea ops & audit query
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| B6-01/02 | Admin sees all Ideas (list/detail); detail view writes `IDEA_VIEW`; Admin download/replace/delete regardless of deadline; Admin create/edit text → 403 | Matrix; Matrix | B3-12, B4-02, B4-03 | M |
| B6-03 | `GET /admin/audit-logs`: all filters, `[from,to)`, opaque keyset cursor (µs), bad cursor 422, limit ≤ 100, `AUDIT_QUERY` | Paging has no dupes/gaps | B1-03 | M |
| F6-01 | Admin actions on the **shared** list/detail pages via `can()`; **no `/admin/ideas` route** (`Frontend.md` §6.2, F17); Admin detail query: no prefetch, no refetch on focus (each `GET /ideas/{id}` writes `IDEA_VIEW`, §6.7) | MSW + axe; one detail open = one GET | F3-03, F3-04, F4-02, B6-01, B6-02 | S |

### M9 — S7 Hardening & acceptance
| ID | PR / scope | Verify | Deps | Size |
|---|---|---|---|---|
| Q7-01 | Full permission-matrix check; report in `docs/test-report.md` | Report | M4–M8 | M |
| Q7-02/O7-01 | 404 timing consistency (including audit write); Security headers, CSP; probes on management port only; dependency/image scans clean | Tests + report; Scan report | B3-10, O0-05 | M |
| Q7-03 | Manual a11y: keyboard, NVDA+Chrome (OI-23), 200 % zoom, 320 px reflow, forced colors, reduced motion | Checklist | all F PRs | M |
| D7-01 | README, runbook (deploy, rollback, Vault sync, MinIO outage, CSRF key rotation), `docs/limitations.md`, `docs/API/` matches spec; **in 72-hour mode update all module guides at once (§7.7)** | Review | – | M |
| F7-01 | Public `site/` (VitePress zh-TW/en, no infra details) | Local preview | F0-01 (OI-06) | M |
| Q7-04 | Acceptance review vs PRD §3.1, §6, §7, §8.1–8.2; sign-off before Bonus; 72-hour mode lists unfinished items and reasons | Sign-off | all above | S |

### M10 — Bonus (after S7 sign-off, PRD §9 order; no issues yet; all depend on Q7-04)
B-B1 stats API/dashboard · B-B2 Excel export (streaming, ≤ 10,000 rows) · B-B3 optimistic lock (`ETag`/`If-Match`, 412) · B-B4 join-request rate limit · B-B5 hash-chain archive (OI-30) · F-B6 pending badge · B-B7 orphan-file cleanup job (`SKIP LOCKED`).

## 4. Parallel tracks
Solo 72 h: serial by §7 priority. With 4–5 people: DevOps `O*`; BE-1 shared/audit/storage/idea; BE-2 account/auth/deadline/team; FE-1/FE-2 `F*`. Backend identity (B1-*) ∥ frontend identity (F1-*, on MSW). After M5, S4 ∥ S5, coupled only via idea's public interface.

## 5. Counts
Prep 3 (3S) · P0 13 (5S/8M) · P1 33 (11/22) · P2 40 (20/20) · P3 18 (11/7) · Wrap-up 2 (1/1) = **109 core (51 S / 58 M)**; Bonus ~10. (v0.3: F6-01 M → S; P2 S/M off by one each.)
## 6. Confirmed (2026-11-08)
1. Epic names confirmed. 2. Linear dropped in favour of beads; finished Golang work not reused (PRD v1.5 differs). 3. ADR-0001 **approved**; A1–A3, O1–O3 use `Backend.md` suggestions, recorded in D-03. 4. Hand-written code ≤ 400 lines. 5. Bonus issues: default **create after S7 sign-off**. 6. Routing (v0.3): single login entry; **no `/admin/ideas`**; wrong-role routes show the same 404 page as unknown routes (`Frontend.md` §6, F16–F18).

## 7. 72-hour budget (unconfirmed)
**Assumptions:** 72 h = one person's hours; Claude Code writes code, owner prompts/reviews/verifies/waits on CI; costs include DoD overhead. Per-PR cost (S / M): fast 0.5 / 1 h, **baseline 0.75 / 1.5 h**, slow 1 / 2 h; calibrate after the 8th PR.

**The full plan does not fit:** core total (109 PRs) = 83.5 h fast, 125 h baseline, 167 h slow (1.2× / 1.7× / 2.3× of 72 h). Bonus is out of scope.

**Allocation:** prep 3 h (rest of D-01, D-02, D-03, decide OI-24, beads setup) · build 54 h · buffer 9 h (~17 %) · wrap-up 6 h (D7-01 light, Q7-04, final verification, demo script).

### 7.1 Priorities (risk and dependents first, then fastest endpoint growth, platform-dependent last)
| Pri | Content | PRs | Endpoints (of 20) |
|---|---|---|---|
| P0 | Local skeleton, `make ci` | O0-01, O0-02, B0-01, B0-02, B0-03, B0-05, B0-06, B0-07, B0-09, B0-10, B0-12, B0-14, O0-03 | 1 |
| P1 | Auth, Idea create/view/list/download, minimal frontend, smoke | B0-08, B1-01–11, B2-04, B3-01–05, B3-08, B3-10, B3-11, B3-13, F0-01–04, F1-01–04, F3-01, F3-03, F3-04, Q3-01 | **10** |
| P2a | Guardrails | B0-11, B0-13, B1-12, Q1-01 | – |
| P2b | Storage consistency | B3-06, B3-07, B3-09 | – |
| P2c | Edit/replace/delete | B4-03, B4-01, B4-02, Q4-01, F4-01, F4-02 | 13 |
| P2d | Password policy | B1b-01–03, F1b-01 | – |
| P2e | Admin & deadline | B2-01–03, B2-05, F2-01 | 14 |
| P2f | Login throttling | B1b-04–07 | – |
| P2g | Team & join requests | B5-01–06, F5-01, F5-02 | 19 |
| P2h | Admin ops, audit query, search/sort | B6-01–03, F6-01, B3-12 | 20 |
| P3 | Out of 72 h | B0-04, O0-04–09, F0-05, F1-05, F1b-02, B2-06, F2-02, F3-02, Q7-01–03, O7-01, F7-01 | – |
| Wrap-up | Always | D7-01, Q7-04 | – |

F0-04 (route table + 404 page) is needed by the P1 login page, so it moved into P1 (v0.3); timetable not recomputed.

### 7.2 Outcomes (54 h build) and checkpoints
- **Fast** (P0+P1 = 38 h): P0, P1, P2a–d done, P2e half → **13/20**, with frontend.
- **Baseline** (57 h): 3 h over; cut Q3-01 and F1-01 → ~54.75 h → **10/20**, no P2.
- **Slow** (76 h): P0 + auth + create/view Idea → **8/20**, API only.
- P1 cut order when behind (baseline hours saved): Q3-01 (1.5) → F1-01 (0.75) → F3-04 (0.75) → F3-03 (1.5) → B3-13 + B3-11 (3).

| Checkpoint | Build h | Pass | If missed |
|---|---|---|---|
| Calibration | 8–10 | Real unit cost → pick scenario | ≥ 20 % over baseline: slow cuts |
| P0 done | ~16 | `make ci` green, gate on, `GET /deadline` on compose | Fix gate first |
| Backend login | ~31 | register→login→me→logout; CSRF green | Fix before Idea |
| Core path | ~43.5 | create→view→list→download; 404 consistency | Cut F3-03/F3-04 |
| Cutoff | 54 | No new PRs | – |
| Buffer end | project 66 | Wrap-up; abandon unfinished, list in limitations | – |

### 7.3 Costs, savings, DoD adjustments
- Per-PR DoD overhead 0.25–0.5 h × 46 PRs = 12–23 h → merge adjacent S PRs (≤ 400 lines), saves ~4 h.
- Self-built auth: P1 ~6 h, throttling +6 h (P2) — unavoidable, defer throttling.
- Platform dependencies ~10 h plus OI waits → all P3; docker-compose as acceptance env.
- Storage consistency ~5 h → P2b; if unfinished, document orphan files as known limit.
- Coverage 80 %: **decide O2 = generated code excluded from denominator.**

**Adjustments needing your approval:** (1) module guides updated once in D7-01 (deviates from `docs/modules/README.md` rule 4); (2) matrix harness Q1-01 deferred to P2a, but every PR still adds its cells; (3) no guaranteed CD, acceptance = docker-compose + `make ci`; (4) zh-TW first, en dictionary and Q7-03 out of scope; (5) **unchanged:** coverage ≥ 80 %, lint, `-race`, no generated diff, check order 401 → CSRF → 404.

### 7.4 Risks
| Risk | Mitigation |
|---|---|
| sqlc can't parse `UUID`/`INET6`/`SKIP LOCKED` (+3 h) | B0-02 first; fall back to `database/sql` in same `store`, same interface |
| OI-07 unanswered | `make ci` enforces gate locally; GitHub-hosted runners run the same target |
| OI-24 / OI-04 undecided | Decide OI-24 in prep (30 min); pin MariaDB 11.4 LTS |
| A1/A2 overturned (+2 h) | Accepted risk |
| Tests skipped under rush | Gate on from first code PR, never relaxed |

### 7.5 Hard gate
PRD §8.1 and §1 above: grading requires **all functional requirements, CI/CD, backend coverage ≥ 80 %** (PRD §8.1). In 72 h coverage and CI are achievable; CD is not in any scenario; only 8–13 of 20 endpoints. The real decision: **may 72 hours deliver partially?** If not, extend time or reduce scope (e.g. ask graders whether team features may be excluded).

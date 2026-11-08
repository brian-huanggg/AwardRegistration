# Open Items — Single Tracker
 
| | |
|---|---|
| Version | 1.5 |
| Date | 2026-11-08 |
| Companion of | `docs/PRD.md` v1.5, `docs/API/`, `docs/Architecture/Architecture.md` v0.8, `docs/Architecture/Backend.md` v0.4, `docs/Architecture/Frontend.md` v0.6, `docs/Architecture/Sequence.md` v0.3, `docs/PR-plan.md` v0.4, `docs/modules/` |
 
This is the **only** place where unresolved questions, owner confirmations and platform answers are tracked. It replaces PRD §11–§12 (v1.4), overview §14 (open points), frontend §14 and delivery-plan §9.
 
How to use it:
 
- One row per question, identified by **OI-nn**. Other documents cite the ID and never restate the question.
- A **[Proposed]** marker in the owning document points here by ID. When a row is answered, move it to "Resolved" below and update the marker in the owning document.
- "Gates" says which delivery slice (`docs/PR-plan.md`) cannot be accepted before the answer; the PR list in `PR-plan.md` shows the same information per slice.
- The backend is implemented in Go (ADR-0001, accepted 2026-11-08). The Go-specific points of ADR-0001 are tracked here: A3a = OI-51, A3b = OI-50, A3c = OI-52, O1 = OI-53, O2 = OI-54, O3 = OI-55.
- Gaps found while writing a module guide (`docs/modules/`) are listed in that guide's last section first and move here once someone agrees they are real questions.
Changes in 1.5 (D-03): ADR-0001 open points O1–O3 given OI-53–OI-55 with default and owner; A3a/A3b/A3c cross-referenced; `modules/shared.md` gaps 1–4 were already decided (§5, D53–D56).
 
Changes in 1.4: OI-24 (front-end libraries) answered by the owner and moved to §5.
 
Changes in 1.3: OI-50 (JSON body limit and error code) answered by the owner and moved to §5.
 
Changes in 1.2: Go items OI-50 to OI-52 added (§3); owner decisions on the Go design (A1, A2, Go version, status 413) recorded in §5.
 
Changes in 1.1: four gaps found while writing `docs/modules/shared.md` were decided by the owner on the same day and are recorded directly in §5 (D53–D56).
 
---
 
## 1. Platform and external answers
 
| ID | Item | From | Gates | Where it applies | Status |
|---|---|---|---|---|---|
| OI-01 | Vault authentication method; path and JSON schema of the Admin directory | Platform / security | S2 Vault implementation; before S6 (the file-based directory is the fallback) | PRD §4.2; backend §9 | Open |
| OI-02 | Argo CD application and project names; credentials for the pipeline to commit to the deployment repository (layout and tag-update loop are resolved by the separate deployment repository) | Platform | S0 deploy | PRD §8.7; overview §12 | Partly resolved |
| OI-03 | How database and MinIO credentials are delivered to the pods (Kubernetes Secrets provisioned by the platform?) | Platform | S3 deploy | PRD §8.7 | Open |
| OI-04 | **MariaDB version reopened:** 10.8 is a short-term release and end of life; **11.4 LTS** proposed (maintained until 2029-05; minimum 10.7 for native `UUID`). MinIO version also needed; the Testcontainers images must match | Platform | S0 Testcontainers (before S0) | PRD §8.6; overview D27, D45; backend §10.1, §11 | Open |
| OI-05 | Prometheus / Grafana availability and the dashboard import mechanism (Monitoring is not committed) | Platform | none | PRD §8.2 | Open |
| OI-06 | Hosting location and domain of the public `site/` and of the application (same-origin routing); ingress body-size limit 6 MiB | Platform | S3 exit | PRD §6.8, §8.7; frontend §11 | Open |
| OI-07 | GitHub Actions runners: GitHub-hosted (Docker and Docker Hub available, fine for `make ci`) or self-hosted? The image-push and deployment-repo stages need Harbor and Argo CD reachable from the runner; must images come through a Harbor proxy cache? | Platform | S0 pipeline (before S0) | PRD §8.7; backend §11; delivery-plan S0 | Open |
 
## 2. Owner decisions and confirmations
 
Each row changes a **[Decided]** text or is a product decision; the engineering default is given.
 
| ID | Item and proposed default | From | Gates | Where it applies | Status |
|---|---|---|---|---|---|
| OI-10 | Replace Spring Session with an own session filter plus `SecurityContextRepository` (**Yes**) — overview D41. Until answered, the custom Spring Session repository stays the working assumption | Project owner | S1a (ask before it starts) | PRD §6.5, §8.6; backend §1, §6; overview D6, D41 | Proposed |
| OI-11 | Refresh the session at most once per 60 s (**Yes**); effective idle timeout becomes 29–30 minutes — overview D42 | Project owner | S1a | PRD §6.5, §10; backend §6 | Proposed |
| OI-12 | The 5th failed login returns 401 and sets the lock; 429 starts with the 6th attempt (**Yes**) | Project owner | S1b | PRD §6.5; backend §6 | Proposed |
| OI-13 | Reject Vault hashes with bcrypt cost ≠ 10 (**Yes**). Reason: another cost changes login timing and would let an attacker tell Admin employee IDs apart. Constrains how the platform team generates hashes — overview D51 | Project owner | S2 | PRD §4.2; backend §9 | Proposed |
| OI-14 | Email limited to printable ASCII (**Yes**); lower-casing is then the ASCII mapping and front end and back end cannot disagree | Project owner | S1a | PRD §6.3; frontend §5.1 | Proposed |
| OI-15 | New error codes `METHOD_NOT_ALLOWED` (405) and `NOT_ACCEPTABLE` (406); new list parameter `order` (**Yes**) | Project owner | S0 (error mapping) | PRD §6.1, §6.6; `API/error-codes.md` | Proposed |
| OI-16 | 409 / 422 are not audited, except registration (**Yes**) — overview D48 | Project owner | S1a | PRD §6.11; backend §7 | Proposed |
| OI-17 | Official zh-TW labels of categories and ESG directions; full names of the Org codes (**owner supplies**). Working drafts are in `Frontend.md` §10 | Project owner | S3 UI (start of S3) | PRD §6.6; frontend §10 | Open |
| OI-18 | Confirm `SubmissionGate` in `deadline` (overview D34) | Project owner | S2, S3 (end of S1a) | backend §2.1; overview D34 | Proposed |
| OI-19 | Confirm the 10-minute grace period before an `UPLOAD_ABORTED` key may be cleaned up | Project owner | S3 exit | sequences §2 | Proposed |
| OI-20 | Product defaults to confirm: millisecond timestamps in the API; second-precision deadline; immediate re-apply after reject or withdraw; Chrome and Edge only | Project owner | S1a – S3 | PRD §6.1, §6.7, §6.9, §8.5 | Proposed |
| OI-21 | Front end: no automatic renaming of the slide file before upload (**proposed no**) | Project owner | S3 UI (start of S3) | frontend §5.5 | Proposed |
| OI-22 | Front end: idle-timeout warning at 28 minutes, "Continue" calls `GET /auth/me` (**proposed yes**) | Project owner | S1a UI (end of S1a) | frontend §8.1; delivery-plan S1a | Proposed |
| OI-23 | Manual screen-reader check with NVDA + Chrome only (**proposed**) | Project owner | S7 | frontend §8.2 | Proposed |
| OI-25 | Team size and contest deadline, needed to set calendar dates | Project owner | milestone dates (before S0) | delivery-plan §2, §7 | Open |
| OI-26 | Response bodies, including `expectedFilenameBase` in the Idea detail and `testdata/expected-filename.json` | Project owner | S3 | `api/response-bodies.md`; PRD §6.14; frontend §5.5, F14 | Proposed |
 
## 3. Engineering items
 
| ID | Item | From | Gates | Where it applies | Status |
|---|---|---|---|---|---|
| OI-30 | Hash-chain sealing strategy under concurrent audit writes (inline with a chain-head lock, or a background sealer): decided in an ADR when Bonus #5 starts | Architect | Bonus B5 | PRD §9 #5; backend §7 | Deferred |
| OI-31 | Third-party library compatibility with Spring Boot 4 (springdoc-openapi, Testcontainers, Jackson 3, …): handled as problems appear, no dedicated spike; risk accepted (overview D28). S0 exercises every library | Engineering | S0 | PRD §8.6; delivery-plan S0, §8 | Deferred |
| OI-51 | **sqlc on the real schema.** The spike (`Adr/0001-Annex-sqlc-spike.md`) proved that sqlc works on a converted schema with rules R1–R5, but ran only on test tables and without a database. Repeat on the real `V001__init.sql`; run every statement once against MariaDB (UUID text round-trip, `INET6` scanning, matched-row counts); decide how a later `ALTER TABLE … ADD COLUMN x UUID` is handled by the conversion script; pin the sqlc version | Engineering | S0 (PR `B0-02`, before the first module) | backend-go §2.3; ADR-0001 G1 | Resolved (B0-02, 2026-11-08; Annex §5) |
| OI-52 | **minio-go single attempt.** `Options.MaxRetries` must be `1`; `0` keeps the default of 10 (verified in the source of v7.4.1). Add a test that asserts one connection attempt against a refusing server | Engineering | S3 (PR `B3-02`, MinIO port) | backend-go §4.4; overview D50 | Proposed |
 | OI-53 | **JSON decoder design (ADR-0001 O1).** `encoding/json` v1 matches field names case-insensitively and lets the last duplicate key win; `encoding/json/v2` is still behind `GOEXPERIMENT=jsonv2`. Proposed: `Token()` pre-pass (duplicate keys, trailing data) plus exact-case field picking in `shared/web`; replace by v2 when stable, `test/binding` stays. **Default: as proposed (implemented in B0-08)** | Project owner | S0 (confirm before S1a) | ADR-0001 O1; backend §3 | Proposed |
| OI-54 | **Coverage denominator (ADR-0001 O2).** Exclude generated code (sqlc, oapi-codegen) and the `cmd/` entrypoint from the 80 % statement-coverage gate? **Default: yes** | Project owner | S0 (`make ci`) | PRD §8.4; PR-plan §1 | Proposed |
| OI-55 | **Probe paths (ADR-0001 O3).** Keep `/actuator/health/{liveness,readiness}` (current code, `backend/internal/server/server.go`) or switch to `/healthz`-style paths (the frontend chart already uses `/healthz`) and update the backend chart. **Default: keep `/actuator/...` until O0-07 (backend Helm), then decide with Platform** | Engineering / Platform | S0 deploy (O0-07) | PRD §8.7; PR-plan O0-07 | Open |

## 4. Assumptions awaiting review
 
Assumptions that are already visible as an inline **[Proposed]** rule in the owning document are tracked by that rule and are not repeated here. Only assumptions with a dependency on someone else's answer remain:
 
| ID | Assumption | Depends on | Where it applies |
|---|---|---|---|
| OI-40 | If a Vault sync fails after startup, existing Admins are kept (never emptied) and an alert metric is raised; if the first read fails, the service is not ready | OI-01 | PRD §4.2; backend §9 |
| OI-41 | The Admin directory sits behind an interface with a Vault implementation and a file-based implementation for local and test profiles only | OI-01 | PRD §4.2; backend §9 |
 
---
 
## 5. Resolved
 
| Was | Resolution | Date | Recorded in |
|---|---|---|---|
| PRD §12 #7 — front-end framework | React + Vite (VitePress stays Vue) | 2026-11-08 | overview D11 |
| PRD §12 #8 — NFR commitments | API Spec, Structured Logging, Availability, Fault Tolerance; numeric targets stay proposals | 2026-11-08 | overview D12; PRD §8.1–8.2 |
| PRD §12 #11 — error precedence for oversized bodies | **413 first**, bodies are not drained | 2026-11-08 | overview D39; PRD §6.1 |
| PRD §12 #14 — CSRF binding; email whitespace | CSRF token bound to the session; no whitespace in email | 2026-11-08 (owner) | overview D40; PRD §6.3, §6.5 |
| Review 1.4 — delete status; table name | Delete returns 204 everywhere; table is `pending_file_deletion` | 2026-11-08 | PRD §7; backend §10.2 |
| Review 1.2 — package list in the PRD | Aligned with architecture | 2026-11-08 | PRD §8.6 |
| `modules/shared.md` gap 1 — direction of the track-B audit call | `shared/web` defines the port `RejectedRequestListener`; `audit` implements it; the central exception handler and the filters call the port, so `shared` never depends on `audit` | 2026-11-08 (owner) | overview D53; backend §2.1, §7; `modules/shared.md` |
| `modules/shared.md` gap 2 — `ErrorCode` catalogue vs D32 | `shared` defines an `ErrorCode` interface and the generic codes; each module defines its own enum; a contract test checks that the union equals `testdata/error-codes.json` | 2026-11-08 (owner) | overview D54; backend §2.5; `modules/shared.md` |
| `modules/shared.md` gap 3 — millisecond output | API timestamps are **truncated** (not rounded) to milliseconds; locked by a test | 2026-11-08 (owner) | overview D55; backend §10.1 |
| `modules/shared.md` gap 4 — malformed Idea ID | A malformed ID on an Idea path returns the same 404 and **is audited as DENIED** (reason `NOT_FOUND`, `targetId` null, route template recorded), like every other 404 on an Idea path | 2026-11-08 (owner) | overview D56; backend §7 |
| ADR-0001 A1 — generated server style | oapi-codegen generates the **standard (non-strict)** chi server interface; the strict server decodes JSON before the handler and would break the check order | 2026-11-08 (owner) | overview D58; ADR-0001 G4; backend-go §3 |
| ADR-0001 A2 — router | **chi** instead of `net/http.ServeMux` (problem+json 404 / 405 hooks; `HEAD` on a `GET`-only route answers 405; checked on Go 1.26.10) | 2026-11-08 (owner) | overview D59; ADR-0001 G12; backend-go §3 |
| Go language version | **Go 1.26** (`go 1.26` + `toolchain` line, latest patch at the start of S0) | 2026-11-08 (owner) | overview D57; ADR-0001 G13; backend-go §1 |
| OI-24 — front-end library choices | React Router, TanStack Query, React Hook Form, react-i18next, **shadcn/ui (Radix) + Tailwind CSS**, raw `XMLHttpRequest` for upload, `eslint-plugin-boundaries`; chart library deferred to the bonus. Runtime-injected `<style>` elements are checked under the real CSP | 2026-11-08 (owner) | Frontend §2, F4, F6, F8 |
| OI-50 / ADR-0001 A3b — oversized JSON request body | Answers **413** with the new code **`REQUEST_TOO_LARGE`**; limit **16 KiB** (the largest JSON body, the Idea edit, stays under 5 KiB even with every character escaped). `FILE_TOO_LARGE` stays for files. The code is added to `API/error-codes.md` now; `testdata/error-codes.json` and the zh-TW / en dictionaries follow in the implementing pull requests | 2026-11-08 (owner) | overview D60; `API/error-codes.md` v1.1; backend-go §3 |
 
## 6. Mapping from the old numbers
 
Older documents and commit messages may still say "PRD §12 #n". Use this table to translate.
 
| Old reference | Now |
|---|---|
| PRD §12 #1 | OI-01 |
| PRD §12 #2 | OI-02 |
| PRD §12 #3 | OI-03 |
| PRD §12 #4 | OI-04 |
| PRD §12 #5 | OI-05 |
| PRD §12 #6 | OI-06 |
| PRD §12 #7, #8, #11, #14 | Resolved (§5) |
| PRD §12 #9 | OI-31 |
| PRD §12 #10, frontend §14 #1 | OI-24 (Resolved, §5) |
| PRD §12 #12 (table rows, in order) | OI-10, OI-11, OI-12, OI-13, OI-14, OI-15, OI-16, OI-17, then OI-18 / OI-19 / OI-20 (defaults row), then OI-21 / OI-22 / OI-23 (front-end row) |
| PRD §12 #13 | OI-07 |
| PRD §11 #1, #2 | OI-40, OI-41 (other §11 items are inline rules) |
| overview §14 #1 – #10 | #1 OI-01 · #2 OI-04 / OI-03 / OI-05 / OI-06 · #3 OI-24 · #4 OI-30 · #5 OI-31 · #6 OI-18 · #7 OI-19 · #8 OI-10 / OI-11 · #9 OI-10 … OI-23 · #10 OI-07 |
| frontend §14 #1, #3, #4, #5, #6, #7 | OI-24, OI-26, OI-21, OI-22, OI-23, OI-17 |
| delivery-plan §9 (old decision table) | OI-25, OI-24, OI-04, OI-07, OI-10, OI-11, OI-12, OI-14, OI-18, OI-13, OI-21, OI-22, OI-17, OI-01, OI-03, OI-06, OI-05; the per-slice view is the PR list in `PR-plan.md` |
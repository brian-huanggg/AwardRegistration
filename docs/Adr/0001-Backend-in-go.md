# ADR-0001 — Implementing the backend in Go
 
| | |
|---|---|
| Status | **Accepted** by the project owner on 2026-11-08. The backend is implemented in **Go**. The previous Java 21 + Spring Boot 4 decision (PRD §8.6) is superseded once the documents listed under "What adoption requires" are updated (PR `D-01` in `docs/PR-plan.md`). |
| Date | 2026-11-08 (drafted for PRD 1.4; accepted against PRD 1.5; amendments A1, A2, the Go version and the A3 findings recorded the same day) |
| Affects | `docs/PRD.md` §8.1, §8.2, §8.4, §8.6; `docs/Architecture/Architecture.md` D1, D6, D28, D31, D33, D41, D43, D46; `docs/Architecture/Backend.md` §1, §2, §3, §4, §6, §11 (Go equivalents are in `docs/Architecture/Backend.md`) |
| Unaffected | Data model, storage-consistency rules (D36–D39, D44 rule itself), audit tracks and scope (D2, D3, D48), uniform 404 (D9), database-time authority (D7), session-bound CSRF design (D40), API contract (PRD §6.1, §6.14), front end, delivery (D10) |
| Annex | `docs/Adr/0001-Annex-sqlc-spike.md` — sqlc spike (A3a), 2026-11-08 |
| Decided after acceptance | **A1, A2** (owner, 2026-11-08), **Go 1.26** (G13), **A3b** status 413 for oversized JSON bodies. **A3a** spike done, verdict "usable with rules R1–R5", to be repeated on the real schema in PR `B0-02`. **A3c** checked in the minio-go source. Open: **A3b** limit value and error code (OI-50), **O1–O3** below. |
 
## Context
 
This document checks what a Go implementation looks like if the current architecture is kept. Go is not on the PRD's list of forbidden languages (JavaScript, Python, C#, Rust). Most architecture decisions don't depend on the language. What changes is the work Spring did for us (session, CSRF, transactions, OpenAPI generation) and how the module rules get enforced.
 
## Choices
 
### Agreed by the project owner
 
| # | Choice | Notes |
|---|---|---|
| G1 | Persistence: **sqlc**. Each module owns its own `queries.sql` and the generated code under `<module>/internal/store/`. No ORM. | Keeps the intent of D31: hand-written SQL owned per module. Driver `go-sql-driver/mysql`; DSN `parseTime=true&loc=UTC&time_zone='+00:00'`. ORMs are banned via `depguard`. The session statement relies on matched-row counts: the DSN needs `clientFoundRows=true` (the driver's default reports changed rows). **Spike result (2026-11-08, sqlc v1.31.1, Go 1.26.10; parsing and code generation only):** sqlc's MySQL parser **rejects the native `UUID` and `INET6` column types**, so sqlc reads a schema converted from the Flyway migrations by a script, with per-column Go type overrides (`google/uuid`, `shared/db.IP`). 17 of the 18 test statements, including `FOR UPDATE SKIP LOCKED`, `ON DUPLICATE KEY UPDATE` and the `UPDATE … JOIN` session statement, generate correct code (two of them with a loosely typed parameter). The 18th, a keyset row comparison, fails **silently**, and so does one further probe, `sqlc.arg(…) IS TRUE`: sqlc reports no error but leaves the call in the SQL text. CI must catch this. Rules R1–R5 and details: Annex. |
| G2 | JSON request DTOs use **pointer fields** for every client-supplied field. ~~Decoding uses `DisallowUnknownFields`.~~ **Unknown fields are ignored** (amended, see below). | Pointers tell "missing" apart from "zero value", which is the line between 400 `MALFORMED_REQUEST` and 422 `REQUIRED` (PRD §6.1). Remaining gaps are listed in O1. |
| G3 | Passwords use **bcrypt, cost 10** (`golang.org/x/crypto/bcrypt`). Argon2id is not used. | Same as PRD §6.4. Go's bcrypt rejects inputs over 72 bytes. At login, an input over 72 bytes returns `INVALID_CREDENTIALS` **after** the dummy-hash comparison still runs, so timing stays uniform. Vault admin hashes (`$2a$` / `$2b$`) need a contract test; hashes with cost ≠ 10 are rejected by the sync (D51). |
| G4 | OpenAPI is **spec-first**: `backend/api/openapi.yaml` is the single source. Server interfaces are generated with **oapi-codegen, standard (non-strict) chi server interface**. ~~strict server~~ | **Amended by owner decision A1 (2026-11-08).** Each handler receives `http.ResponseWriter` and `*http.Request` and decodes its own body. The strict server was dropped: it decodes JSON before the handler runs (a malformed body would answer 400 before 404 / role 403 / state 403) and it uses plain `encoding/json` (duplicate keys accepted). Path and query parameters are still bound by the generated wrapper. Multipart endpoints get a streaming reader (D38). CI: lint the spec (Spectral, same rules as `Backend.md` §11), regenerate the server code and fail on a diff, and generate front-end types from the same file. |
| G5 | The coverage gate is **statement coverage ≥ 80 %**. | `go test -coverpkg=./... -coverprofile`, gated on the total from `go tool cover -func`, and published as a GitHub Actions job summary and artifact. |
| G13 | **Go version 1.26.** `go.mod` carries `go 1.26` and a `toolchain` line with the latest 1.26 patch release at the start of S0 (1.26.10 on 2026-11-08). CI and the container build use the same version. | Owner instruction, 2026-11-08. The spike, the chi check and the `json/v2` check were run on Go 1.26.10. |
 
**G2 amendment (needs owner confirmation).**
- `DisallowUnknownFields` would answer 400 to a request that carries a `leader` field. PRD §6.6 requires such a field to be **ignored**, and PRD 1.4 §6.1 / D46 make "unknown fields are ignored" the general rule.
- The Go implementation therefore decodes without `DisallowUnknownFields`.
- It still rejects duplicate keys and wrong types with 400 (O1).
### Design points (accepted with this ADR)
 
| # | Point |
|---|---|
| G6 | **Module boundaries:** every module is a package under `internal/<module>/`, with its implementation in `internal/<module>/internal/`. The compiler enforces both cross-module `internal` access and acyclic imports. The dependency direction of `Backend.md` §2.1 is checked by a Go test over the import graph, plus `depguard`. ArchUnit is not needed. |
| G7 | **Transaction helper** `shared/db.InTx` returns `Committed`, `RolledBack` or `Unknown`, using the rule of D44 (`Sequence.md` §2). An error before `COMMIT` counts as a known rollback. A `COMMIT` failing with SQLState class `23` or `40` also counts as a known rollback. **Any other `COMMIT` error, including other `*mysql.MySQLError` values, counts as unknown.** (Revised: the earlier draft treated every `*mysql.MySQLError` from `COMMIT` as a known rollback; aligned with the conservative Java rule so that both implementations behave the same.) |
| G8 | **Context policy:** write transactions run on `context.WithoutCancel(r.Context())` with their own timeout. `database/sql` rolls a transaction back automatically when its context is cancelled. |
| G9 | **Audit by signature:** `Record(ctx, tx, event)` needs the caller's transaction (track A). `RecordRejected(ctx, event)` opens its own transaction (track B), and is called only from the central handler-error adapter. `SubmissionGate.AssertOpenInTx` likewise requires a `tx`. |
| G10 | **Pipeline:** RequestID → Recover → Logging → BodyLimit (413) → Session (401) → CSRF (403, session-bound tokens per D40) → handler. 404 / role / state are resolved before the multipart body is read, then the body is streamed into a temp file through `io.MultiWriter(file, sha256)`. Router: chi (G12). |
| G11 | **Own implementations replace Spring Session and Spring Security:** session middleware, HMAC-signed, session-bound double-submit CSRF, and throttling with the existing SQL. Flyway stays as the migration tool. Logging uses `log/slog` (ECS field names); metrics use `prometheus/client_golang`; probes run on a second `http.Server`. |
| G12 | **Router: chi** (`github.com/go-chi/chi/v5`), replacing `net/http.ServeMux`. **Decided by owner (A2, 2026-11-08).** Checked on Go 1.26.10 with chi v5.5.0: the `NotFound` and `MethodNotAllowed` hooks produce problem+json 404 / 405, and `HEAD` on a `GET`-only route answers **405** by default. `ServeMux` answers `HEAD` with the `GET` handler (status 200, which would run the download and write an audit row) and its 404 / 405 are plain text with no hook. The download route still registers an explicit `HEAD` → 405 and a test locks it (D47). |
 
### Amendments and findings
 
| # | Item | Status |
|---|---|---|
| A1 | G4: oapi-codegen generates the **standard (non-strict)** chi server interface, not the strict server. | **Decided** (owner, 2026-11-08) |
| A2 | G10 / G12: router **chi** instead of `net/http.ServeMux`. | **Decided** (owner, 2026-11-08) |
| A3a | sqlc against the MariaDB schema. | **Spike done** (Annex): usable with rules R1–R5. Repeat on the real `V001__init.sql` and against a running MariaDB in PR `B0-02` (OI-51) |
| A3b | JSON body limit and its 413. | **Status 413 decided** (owner, 2026-11-08). Limit value and error code open (OI-50); worst-case body is under 5 KiB, 16 KiB proposed |
| A3c | minio-go retry switch-off. | **Checked in the source** (v7.4.1, the version the module proxy returned): `Options.MaxRetries` set to **1** gives a single attempt; **0 means the default of 10** and does not disable retries. A test locks it (OI-52) |
 
### Open
 
| # | Item |
|---|---|
| O1 | **Facts checked on Go 1.26.10.** `encoding/json` (v1) matches field names case-insensitively (`"EMPNO"` fills `empNo`) and, for a duplicate key, the last one wins. `encoding/json/v2` is **still behind `GOEXPERIMENT=jsonv2`** and cannot be imported without it, so it is not used. With the experiment on, v2 rejects duplicate keys and matches names case-sensitively, which is what the PRD wants. `Backend.md` §3 proposes the `Token()` pre-pass plus exact-name field picking. When v2 becomes stable, only the decoder in `shared/web` is replaced; the `test/binding` tests stay. Waiting for owner confirmation of the decoder design. |
| O2 | Exclude generated code (sqlc, oapi-codegen) from the coverage denominator? Recommended: yes. |
| O3 | Probe paths: keep the `/actuator/...` paths or switch to `/healthz`-style paths and update the chart. |
 
## What adoption requires
 
Tracked as delivery task **D-01** in `docs/PR-plan.md`; this ADR does not change those documents by itself.
 
- PRD §8.6 Backend row rewritten for the Go stack.
- PRD §8.2 API Spec: "generated from code" becomes "spec-first, checked against the implementation in CI".
- PRD §8.1, §8.4: line coverage becomes statement coverage; ArchUnit becomes compiler rules plus an import-graph test.
- `Backend.md` §1, §2.2–§2.5, §3, §4, §6, §11 superseded by `Backend.md`; new `Architecture.md` decision-log entries superseding D1, D6, D28, D31, D33 (entries D57–D61 for the choices of 2026-11-08 are already added).
- `Backend.md` status row and the sentence about Go in `Open-items.md` — updated on 2026-11-08.
- Java-specific items from the implementation-readiness review that are redone in Go:
  - own CSRF filter placement (D43) and session filter (D41);
  - JSON binding (D46, see G2 and O1);
  - multipart limit layers and temp-file location (`Backend.md` §3);
  - multipart filename decoding;
  - commit classification (G7);
  - `HEAD` handling (G12);
  - JSON logging format;
  - OpenAPI snapshot test (replaced by G4);
  - JaCoCo merge (replaced by G5).
## Comparison summary
 
| Aspect | Java + Spring (previous) | Go (accepted) |
|---|---|---|
| Module isolation | ArchUnit rules in CI | Mostly enforced by the compiler; direction still needs a test |
| Session, CSRF, transactions | Provided by the framework (custom session repository and own CSRF filter already required) | Written by the team (moderate size) |
| JSON binding | Jackson 3, defaults locked by tests | `encoding/json` gaps (O1); `encoding/json/v2` not stable in Go 1.26 |
| OpenAPI | Code-first (springdoc), as the PRD stated | Spec-first; PRD wording to be changed (D-01) |
| SQL access | `JdbcClient`, plain SQL | sqlc on a converted schema with CI guards (Annex); dynamic `ORDER BY` and a few statements hand-written |
| Coverage metric | Line (JaCoCo), as the PRD stated | Statement; PRD wording to be changed (D-01) |
| Runtime | JVM; warm-up before load tests | Static binary; small image, fast start |
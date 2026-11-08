# Backend Architecture
 
| | |
|---|---|
| Version | 0.2 |
| Date | 2026-11-08 |
| Companion of | `docs/Adr/0001-Backend-in-go.md` (+ `0001-Annex-sqlc-spike.md`), `docs/PRD.md` v1.5, `docs/Architecture/Sequence.md` v0.3 |
| Status | **Accepted with ADR-0001 (2026-11-08).** This is the backend design for the Go implementation. The former Java design document is no longer kept; sections that say "Same as `Backend.md` §n" refer to the section of this file. |
| Status markers | **[Decided]** agreed by the owner · **[Proposed]** drafted by the architect, awaiting confirmation · **[Open]** unresolved (tracked in `docs/Open-items.md`) · **[ADR Gn]** the choice of that number in ADR-0001 |
 
Changes in 0.2: owner decisions of 2026-11-08 applied — standard (non-strict) generated server (A1), chi router (A2), Go 1.26 (G13), status 413 for oversized JSON bodies (A3b); sqlc spike result and rules R1–R5 (§2.3); minio-go retry setting verified (§4.4); `encoding/json/v2` status on Go 1.26 (§3); §13 rewritten.
 
This document describes how the backend of `Backend.md` is built in Go. **Section numbers mirror `Backend.md`**, so references such as "§4.4" or "§6" from `Sequence.md` and `Open-items.md` resolve in both documents.
 
Sections that do not depend on the language are **not repeated**; they say "Same as `Backend.md` §x". If the two documents ever disagree on such a section, `Backend.md` wins. Only the implementation mechanism changes here; requirements, check order, data model, audit scope and storage-consistency rules stay as they are.
 
---
 
## 1. Shape of the backend
 
A **modular monolith**: one deployable static binary, one Go module (`go.mod`), packages by feature, a small shared kernel, no microservices, no message queue. Same principle as D1.
 
| Concern | Choice | Status |
|---|---|---|
| Language | **Go 1.26**: `go 1.26` and a `toolchain` line with the latest 1.26 patch at the start of S0 (1.26.10 on 2026-11-08) | [Decided] (ADR G13) |
| HTTP | `net/http` server; router **chi** (`github.com/go-chi/chi/v5`); server interface generated from `api/openapi.yaml` by **oapi-codegen**, **standard (non-strict)** chi interface | [Decided] (ADR G4 / G12, A1, A2) |
| Persistence | **sqlc** on a **converted schema** (§2.3) + `database/sql` + `go-sql-driver/mysql`; SQL per module; no ORM | [ADR G1]; conversion rules [Proposed] |
| Migrations | **Flyway**, run by the Argo CD PreSync Job (unchanged) | [Decided] (language-neutral) |
| Object storage | `minio-go` v7, `MaxRetries: 1` (§4.4) | [Proposed] |
| Passwords | `golang.org/x/crypto/bcrypt`, cost 10 | [ADR G3] |
| Text clean-up | `golang.org/x/text/unicode/norm` (NFC) + own White_Space table | [Proposed] |
| IDs / IPs | `github.com/google/uuid`; `net/netip` | [ADR G1] |
| Logging | `log/slog`, JSON handler with ECS field names | [ADR G11] |
| Metrics | `prometheus/client_golang` on the management server | [ADR G11] |
| Vault | `github.com/hashicorp/vault/api` behind the `AdminDirectory` interface | [Proposed] (auth method still OI-01) |
| Lint | `golangci-lint` incl. `depguard`, `gosec`, `errcheck` | [Proposed] |
| Tests | `go test` (with `-race`), `testcontainers-go` (MariaDB, MinIO, Flyway, Toxiproxy) | [Proposed] |
| Dependency injection | None: constructors wired by hand in `cmd/server/main.go` | [Proposed] |
 
---
 
## 2. Modules
 
Same modules, responsibilities and table ownership as `Backend.md` §2.
 
### 2.1 Dependency rules
 
Same graph and rules as `Backend.md` §2.1. Go mechanics:
 
- **No cycles** is enforced by the compiler (Go forbids import cycles).
- **`SessionRevoker`:** `account` declares `type SessionRevoker interface { RevokeAll(ctx context.Context, userID uuid.UUID) error }`. `auth` implements it. `main.go` passes the `auth` implementation into the `account` constructor, so `account` never imports `auth` (D30).
- **`SubmissionGate`** (D34, OI-18):
  - `Precheck(ctx, actor) error` — read-only;
  - `AssertOpenInTx(ctx, tx *sql.Tx, actor) error` — the `tx` parameter makes it impossible to call outside a write transaction [ADR G9].
- Allowed edges are listed once in a table inside the import-graph test (§2.4) and compared with the real graph.
### 2.2 Package layout inside a module **[ADR G6]**
 
```
internal/<module>/
    service.go      exported Service interface, commands, DTOs
    handler.go      HTTP adapter for this module's operations (where it has endpoints)
    errors.go       exported error values / codes the module can return
    internal/       implementation, repositories, mappers
        store/      sqlc input (queries.sql), generated code, hand-written queries
```
 
- Go's `internal` rule means `internal/<module>/internal/...` can be imported **only** from inside `internal/<module>/`. This replaces the ArchUnit "no `..internal..` from another module" rule with a compiler error (D33 no longer needs a CI rule).
- Transactions are opened in service implementations. Handlers contain no business logic and no SQL.
### 2.3 Persistence **[ADR G1]**
 
Evidence for this section: `docs/Adr/0001-Annex-sqlc-spike.md` (sqlc v1.31.1, Go 1.26.10; parsing and code generation only, no running MariaDB).
 
- One `sqlc.yaml` with **one package entry per module**; each module's queries live in `internal/<module>/internal/store/queries.sql`.
- No module queries another module's tables. The only cross-table reads stay the explicit joins of `Backend.md` §6 (session) and §8 (visibility).
- Generated code is committed; CI runs `sqlc generate` and fails on a diff.
- **DSN** (driver `go-sql-driver/mysql`):
  ```
  parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27&clientFoundRows=true
  ```
 
  `clientFoundRows=true` makes `RowsAffected()` report **matched** rows. The session statement (§6) depends on it, the same way the Java version depends on `useAffectedRows=false`; a test locks it.
**What the spike found.** sqlc's MySQL parser rejects the native `UUID` and `INET6` column types and therefore cannot read the Flyway migrations as they are. Everything else the design needs parsed and generated correctly: `CHECK`, `JSON`, `DATETIME(6)`, `utf8mb4_bin`, `BINARY(32)`, foreign keys, `FOR UPDATE`, `FOR UPDATE SKIP LOCKED`, `INSERT … ON DUPLICATE KEY UPDATE`, `UPDATE … JOIN … SET … = IF(…)`, `NOW(6) - INTERVAL …`, `LIKE … ESCAPE`, `DELETE … LIMIT`, `sqlc.slice`.
 
**Rules R1–R5 [Proposed]** (full text and evidence in the Annex):
 
| # | Rule |
|---|---|
| R1 | sqlc reads a **converted schema**, never the migrations. `make sqlc-schema` runs `scripts/gen_sqlc_schema.py`: `UUID` → `CHAR(36)`, `INET6` → `VARCHAR(45)`, and writes one Go type override per converted column. The two outputs are generated files; CI regenerates them and fails on a diff. |
| R2 | CI fails if any generated `*.sql.go` contains `sqlc.arg`, `sqlc.narg` or `sqlc.slice` inside a SQL string. sqlc leaves such text unreplaced **without an error** when it does not recognise the position (row comparison, `sqlc.arg(x) IS TRUE`), and the statement fails only when it runs. |
| R3 | No row comparison `(a, b) < (…)`. Keyset pagination (Audit Log) is written as `occurred_at < ? OR (occurred_at = ? AND id < ?)`. |
| R4 | A flag passed to SQL is written `CAST(sqlc.arg(x) AS SIGNED)` (typed `int64`); without the cast sqlc types it `interface{}`. |
| R5 | Dynamic `ORDER BY` (the `sort` / `order` of the Idea list) is hand-written in the module's `store` package from a closed `switch` over the API enum; sqlc types a sort-key parameter `interface{}` whatever the cast. Other statements sqlc cannot express are hand-written in the same package with `database/sql`. |
 
- **Type mapping [Proposed]:**
  | MariaDB | Go | Note |
  |---|---|---|
  | `UUID` | `uuid.UUID`; nullable `uuid.NullUUID` | `Scan` accepts the 36-char text form; `Value` writes text, which MariaDB accepts |
  | `INET6` | `shared/db.IP`; nullable `shared/db.NullIP` | wraps `netip.Addr`, implements `Scanner` / `Valuer` (`netip.Addr` has no `Scanner`) |
  | `DATETIME(6)`, `DATETIME(0)` | `time.Time` (UTC) | `parseTime=true`, `loc=UTC` |
  | `JSON` (`audit_log.before/after`) | `json.RawMessage` | |
  | `BINARY(32)` (`session.id_hash`) | `[]byte` | |
  | nullable columns | pointers (`emit_pointers_for_null_types: true`) | |
- **ORM ban:** `depguard` denies `gorm.io/...`, `entgo.io/...`, `github.com/uptrace/bun/...`.
- **[Open] OI-51:** the spike must be repeated on the real `V001__init.sql` and against a running MariaDB (UUID text round-trip, `INET6` scanning, matched-row counts, each statement executed once). Owner: PR `B0-02`. Also open there: a column added later by `ALTER TABLE … ADD COLUMN x UUID` is not seen by the conversion script; the script or a test must cover it.
- **[Proposed] Migration file names:** Flyway versions are zero-padded (`V001__init.sql`), so that tools reading the directory in lexical order see the same order as Flyway.
### 2.4 Rules replacing ArchUnit (CI gate) **[ADR G6]**
 
| `Backend.md` §2.4 rule | Go mechanism |
|---|---|
| 1. `shared` depends on no business module | Import-graph test (`golang.org/x/tools/go/packages`) |
| 2. No access to another module's `internal` | **Compiler** |
| 3. Graph acyclic and equal to §2.1 | Acyclic: **compiler**. Allowed edges: import-graph test with an explicit edge table |
| 4. No ORM | `depguard` |
| 5. No `UPDATE` / `DELETE` on `audit_log` | Test scanning all `queries.sql` and `.go` files |
| – (new) No unreplaced `sqlc.*` call in generated SQL | CI grep, rule R2 |
 
Additional Go rule **[Proposed]**: the generated API package (`internal/api`) may be imported by module handlers and `internal/server`, never by `shared` or by any `internal/<module>/internal/` package. This keeps generated transport types out of service code.
 
### 2.5 Codebase layout **[Proposed]**
 
```
backend/
├─ go.mod, go.sum                   go 1.26 + toolchain
├─ sqlc.yaml, sqlc.overrides.yaml   overrides file is generated (R1)
├─ .golangci.yml
├─ scripts/gen_sqlc_schema.py       Flyway schema -> sqlc schema + overrides (R1)
├─ api/openapi.yaml                 single source of the API contract (G4)
├─ cmd/server/main.go               config, wiring, public + management http.Server, background loops
├─ db/
│  ├─ migration/V001__init.sql …    Flyway (expand-then-contract)
│  └─ sqlc/schema.sql               generated from the migrations (R1); do not edit
├─ internal/
│  ├─ api/                          generated by oapi-codegen (models + chi ServerInterface); do not edit
│  ├─ server/                       router, middleware chain, composes module handlers into api.ServerInterface
│  ├─ shared/
│  │  ├─ web/        requestid, recover, access log, body limits, problem+json, ErrorCode, strict JSON decoder, multipart spooling
│  │  ├─ db/         InTx + Outcome classifier, IP / NullIP types, MySQL error helpers
│  │  ├─ clock/      DbClock (SELECT NOW(6))
│  │  ├─ text/       TextCleanup (NFC, White_Space 25 code points, code-point count)
│  │  ├─ security/   Principal, context accessors
│  │  └─ config/     environment-variable loader (profiles local, k3s, dev)
│  ├─ audit/         Record, RecordRejected, query
│  ├─ deadline/      SubmissionGate, DeadlineService
│  ├─ account/       registration, PasswordPolicy, AdminDirectory (Vault, file), AdminSync, SessionRevoker
│  │  └─ internal/passwords/common-passwords.txt   embedded with //go:embed
│  ├─ auth/          login, session middleware, CSRF middleware, throttling
│  ├─ storage/       SlideStorage (MinIO port), SlideValidator, ExpectedFilename, PendingDeletion
│  ├─ idea/          IdeaService, visibility query
│  ├─ team/          join requests and members
│  ├─ slides/        /ideas/{id}/slides upload and download
│  └─ admin/         HTTP entry points only
└─ test/
   ├─ arch/          import-graph, audit_log scan and sqlc-leftover scan (§2.4)
   ├─ matrix/        permission matrix over testdata/permission-matrix.json
   ├─ contract/      testdata/*.json (text clean-up, password, email, expected filename, error codes)
   ├─ binding/       locks JSON decoding rules (§3)
   ├─ logging/       proves passwords, session IDs and tokens never reach the logs
   └─ apispec/       response validation against api/openapi.yaml (§11)
```
 
Unit tests live next to their package (`*_test.go`). Integration tests use the build tag `integration` and run in a separate CI step.
 
---
 
## 3. Request Pipeline and Check Order
 
The order **401 → CSRF 403 → 404 → role 403 → state 403 → validation** is unchanged. Placement in Go **[ADR G10]**:
 
| Step | Where |
|---|---|
| Request ID, recover, access log | Middleware (outermost) |
| 413 | Body-limit middleware (`http.MaxBytesReader`, limit per route) |
| 401 | Session middleware: every route needs a session except an explicit public list (login, register, `GET /auth/csrf`, plus any other route the PRD marks public) |
| CSRF 403 | CSRF middleware after the session middleware; reads only `X-CSRF-Token` and `__Host-csrf`; **never touches the body** |
| 404, role 403, state 403 (pre-check) | Handler → service, **before the body is decoded** |
| Validation (400 / 422) | Handler decodes the body with the strict decoder, service validates |
| State 403 (authoritative) | `SubmissionGate.AssertOpenInTx` inside the write transaction |
 
`POST /join-requests` keeps its documented order (body before lookup), as in `Backend.md` §3.
 
**Generated server style [Decided] (A1).** oapi-codegen generates the **standard (non-strict) chi `ServerInterface`**: each handler receives `http.ResponseWriter` and `*http.Request` and decodes the body itself. The strict server is **not** used, for two reasons:
 
1. The strict wrapper decodes JSON before the handler runs, so a malformed body would answer 400 before 404 / role 403 / state 403.
2. It decodes with plain `encoding/json`, which accepts duplicate keys (O1).
Path and query parameters are still bound by the generated wrapper. Its error handler maps binding errors to the PRD's codes (for example a malformed Idea UUID in the path), and a test locks each mapping.
 
**Router [Decided] (A2).** **chi** instead of `net/http.ServeMux`. Checked on Go 1.26.10 with chi v5.5.0:
 
| Request | chi | `ServeMux` (`GET /ideas/{id}/slides`) |
|---|---|---|
| `HEAD` on a `GET`-only route | **405**, body written by our `MethodNotAllowed` hook (problem+json) | 200 — runs the `GET` handler |
| other wrong method | 405 problem+json | 405 `text/plain` |
| unknown path | 404 problem+json | 404 `text/plain` |
 
- `NotFound` and `MethodNotAllowed` hooks produce problem+json 404 / 405 (OI-15).
- The download's 405 on `HEAD` (D47) is therefore the default. An explicit `HEAD` route is still registered and tested, so the rule does not depend on a router default.
- 406 `NOT_ACCEPTABLE` is produced by a small `Accept` check middleware.
- An unknown path without a session → 401 (session middleware runs first); with a session → 404 problem+json.
**JSON binding [Proposed]** (D46, ADR G2, O1). One decoder in `shared/web` used by every JSON endpoint. Facts checked on Go 1.26.10: `encoding/json` matches names case-insensitively and lets the last duplicate key win; `encoding/json/v2` is **still behind `GOEXPERIMENT=jsonv2`** and is not used.
 
1. Check `Content-Type: application/json`, else 415 (mapped like Java).
2. Walk the body once with `json.Decoder.Token()`; a **duplicate key** at any level or trailing data after the top-level value → 400 `MALFORMED_REQUEST`.
3. Decode the top level into `map[string]json.RawMessage`, then pick the DTO's fields by **exact, case-sensitive name**. This removes `encoding/json`'s case-insensitive matching; keys that are not exact field names are ignored (PRD §6.6).
4. Decode each picked value into its typed field. Absent, `null` or wrong JSON type (`"empNo": 89290`) → 400 `MALFORMED_REQUEST`. Go performs no scalar coercion.
5. Text clean-up; empty after clean-up → 422 `REQUIRED`.
Every row of `Backend.md` §3's binding table has a test in `test/binding/`. When `encoding/json/v2` leaves the experiment, steps 2–4 can be replaced by its options (with it, duplicate keys are rejected and names are case-sensitive); the tests stay.
 
**Multipart and size limits** (D37, D38, D39):
 
| Layer | Limit | 413 body |
|---|---|---|
| Ingress | 6 MiB | may be HTML |
| `http.MaxBytesReader` on slide routes | 5 MiB + 64 KiB | problem+json `FILE_TOO_LARGE` |
| `http.MaxBytesReader` on JSON routes | **413 [Decided]** (owner, 2026-11-08); **value [Open], 16 KiB proposed** (OI-50) | problem+json; code `REQUEST_TOO_LARGE` **[Proposed]** (OI-50) |
| `storage` validator | exactly 5,242,880 bytes | problem+json |
 
- **Why 16 KiB.** The largest JSON body is the Idea edit (`PUT /ideas/{id}`): name ≤ 75 and summary ≤ 300 code points (PRD §6.6). Even if every code point is written as a 12-byte `\uXXXX\uXXXX` escape, name and summary add up to about 4.5 KiB; with the enum fields and JSON syntax the body stays under 5 KiB. 16 KiB leaves a threefold margin; 64 KiB would also work but admits needlessly large bodies. The password maximum (PRD §6.4) was not checked for this estimate.
- `FILE_TOO_LARGE` says "file"; a JSON body is not a file, so a separate code is proposed. The code is part of the shared error catalogue (`testdata/error-codes.json`, `API/error-codes.md`, both front-end dictionaries); it is added there only when OI-50 is answered.
- The handler uses `r.MultipartReader()` and streams parts itself. `r.ParseMultipartForm` is **not** used: it buffers in memory and creates temp files outside our control.
- The file part is copied with `io.Copy` into `os.CreateTemp("/tmp", …)` through `io.MultiWriter(file, sha256)` with a byte counter. The file is removed in a `defer`. `/tmp` is the pod's `emptyDir`.
- The file name from `Part.FileName()` is taken as UTF-8; NFD file names are covered by the contract tests.
- All text and file validation completes before the upload to MinIO (D37).
---
 
## 4. Transactions and Consistency
 
### 4.1 Database settings
 
Same rules as `Backend.md` §4.1. Go mechanics **[Proposed]**:
 
- Every write transaction starts with `BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})` inside `InTx`, so the isolation level does not depend on server defaults or variable names that differ between MariaDB versions (OI-04).
- Session time zone is fixed by the DSN (§2.3).
- Pool: `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime` below the server's `wait_timeout`. Values are set with the platform (OI-04).
### 4.2 Locking
 
Same as `Backend.md` §4.2. Lock-wait timeout (MySQL error 1205) and deadlock (1213) are detected with `errors.As(err, &*mysql.MySQLError)` and map to 503 with a metric. `InTx` always calls `Rollback` explicitly after a failed statement (MariaDB rolls back only the statement on 1205).
 
### 4.3 Rules the database cannot enforce
 
Same as `Backend.md` §4.3.
 
### 4.4 Storage consistency
 
Rules unchanged (D36, `Sequence.md` §2). Implementation **[ADR G7, G8]**:
 
```go
type Outcome int
const (Committed Outcome = iota; RolledBack; Unknown)
 
func InTx(ctx context.Context, db *sql.DB,
          fn func(ctx context.Context, tx *sql.Tx) error) (Outcome, error)
```
 
| Where it failed | Outcome |
|---|---|
| `BeginTx` or `fn` returned an error (before `Commit`) | `RolledBack` |
| `Commit` failed with SQLState class `23` or `40` | `RolledBack` |
| `Commit` failed in any other way (including `driver.ErrBadConn`, `mysql.ErrInvalidConn` and other `*mysql.MySQLError`) | **`Unknown`** |
 
- **Context [ADR G8]:** `InTx` runs on `context.WithoutCancel(ctx)` with its own timeout (10 s proposed). `database/sql` rolls back a transaction when its context is cancelled; a client disconnect must not turn into a rollback after the object was uploaded.
- **MinIO client [Proposed]** (D50): `http.Transport` with a 2 s dial timeout; per-call context with a 30 s timeout. **`minio.Options{MaxRetries: 1}`** gives a single attempt. Checked in the minio-go source (v7.4.1, the version the module proxy returned): the client uses `MaxRetries` only when it is greater than 0, otherwise it keeps the default of 10, so **`0` does not switch retries off**. A test points the client at a refusing server and asserts exactly one connection attempt (OI-52). Failures map to 503 `STORAGE_UNAVAILABLE`.
---
 
## 5. Team (join request) flow
 
Same as `Backend.md` §5.
 
---
 
## 6. Authentication and Session
 
Requirements and SQL are the same as `Backend.md` §6. Spring Session and Spring Security are replaced by own middleware **[ADR G11]**; this is the Go equivalent of D41.
 
**Session middleware**
 
- Session ID: 32 bytes from `crypto/rand`, base64url in the cookie (attributes per PRD §6.5). The table stores `SHA-256(id)` only.
- Per request:
  1. The atomic `UPDATE session s JOIN app_user u … ` statement of `Backend.md` §6 (generated by sqlc as `:execresult`, spike case c05). `RowsAffected() == 1` (matched rows, `clientFoundRows=true`) means valid.
  2. A `SELECT` of `id`, `emp_no`, `role` from `app_user` joined by the session hash, building the `Principal`.
  The principal is stored in the request context under an unexported key; `shared/security` offers `PrincipalFrom(ctx)`.
- Login issues a new ID; logout deletes the row and is idempotent.
**CSRF middleware** (D40, unchanged design)
 
- Token `base64url(nonce) "." base64url(HMAC-SHA256(key, binding ‖ nonce))` built with `crypto/hmac`; comparison with `hmac.Equal`.
- The cookie value is compared with the header using `subtle.ConstantTimeCompare`.
- HMAC key(s) come from a Kubernetes Secret via environment variables; the previous key is accepted for 8 h after rotation.
**Throttling:** same tables and statements; called from the login handler.
 
**Passwords [ADR G3]**
 
- A dummy bcrypt hash (cost 10) is computed once at startup and compared for unknown or disabled accounts.
- An input over 72 bytes still runs the dummy comparison, then returns `INVALID_CREDENTIALS`. At registration the password policy rejects it first (PRD §6.4).
- The common-password list is embedded with `//go:embed`.
**Admin directory:** `AdminDirectory` interface with a Vault implementation and a file implementation for the local and test profiles (OI-01, OI-41).
 
---
 
## 7. Audit
 
Tracks, scope and table are the same as `Backend.md` §7. Go shape **[ADR G9]**:
 
```go
// Track A: joins the caller's transaction; cannot be called without one.
func (s *Service) Record(ctx context.Context, tx *sql.Tx, e Event) error
// Track B: opens its own transaction; best effort, increments a metric on failure.
func (s *Service) RecordRejected(ctx context.Context, e Event)
```
 
- Handlers return errors of one type, `*web.Error{Status, Code, AuditReason, …}`. The central error adapter in `internal/server` uses `errors.As`, writes problem+json, and calls `RecordRejected` when `AuditReason` is set. No other code calls `RecordRejected`.
- `RecordRejected` uses `context.WithoutCancel` so a client disconnect does not drop the row.
- The Audit Log query pages with the expanded keyset condition of rule R3.
---
 
## 8. Uniform 404 and Timing
 
Same as `Backend.md` §8. The visibility query is one sqlc query in `idea/internal/store`; `idea.Service` exposes it as the only way to read an Idea. The admin flag is passed as `CAST(sqlc.arg(is_admin) AS SIGNED)` (rule R4).
 
---
 
## 9. Multi-Replica Behaviour
 
Same rules as `Backend.md` §9, including the Vault sync table. Go mechanics **[Proposed]**:
 
- Background work (Vault sync, session and throttle purge) runs as goroutines started in `main.go` with a `time.Ticker` plus random jitter; each stops when the root context is cancelled at shutdown.
- Readiness waits for the first successful Vault read (`atomic.Bool` read by the readiness handler).
- The orphan-file claim (`SELECT … FOR UPDATE SKIP LOCKED`) is generated by sqlc as written (spike case c03).
---
 
## 10. Data Model
 
Same as `Backend.md` §10. Go type mapping is in §2.3. The API writes times as UTC truncated to milliseconds (D55; `2006-01-02T15:04:05.000Z`) through one formatting helper in `shared/web`.
 
---
 
## 11. Testing structure
 
| `Backend.md` §11 item | Go equivalent **[Proposed]** |
|---|---|
| Testcontainers, real MariaDB + MinIO, real migrations | `testcontainers-go`; migrations applied by the **Flyway container**, the same tool as production. Images via Harbor proxy cache (OI-07) |
| `testdata/` contract and permission matrix | Table-driven tests reading the same JSON files (`test/contract`, `test/matrix`) |
| ArchUnit | `test/arch` (§2.4) + compiler + `depguard` |
| Logging test | slog handler writing to a buffer; assertions on passwords, session IDs, tokens |
| JSON binding tests | `test/binding` over the strict decoder (§3) |
| Storage-consistency, lost commit | A wrapping `driver.Connector` whose `Tx.Commit` can **commit then fail** or **fail without committing**; one Toxiproxy test (`testcontainers-go` module) cuts a real connection |
| OpenAPI snapshot test | Spec-first (ADR G4): Spectral lint with the same custom rules; `go generate` diff check; **response validation** — integration tests validate every response against `api/openapi.yaml` with `kin-openapi`'s `openapi3filter`, so the implementation cannot drift from the spec |
| Coverage ≥ 80 % (line, JaCoCo) | Statement coverage ≥ 80 % (ADR G5), unit and integration profiles merged. Exclusion of generated code is ADR O2 |
| – (new) sqlc | CI: regenerate the converted schema and the sqlc output and fail on a diff (R1); grep for unreplaced `sqlc.*` calls (R2) |
| – (new) router | Handler tests for 404, 405, 406 and `HEAD` on the download route (§3) |
| – (new) MinIO | One test asserting a single connection attempt (§4.4) |
| — | `go test -race` on every CI run |
 
---
 
## 12. Operations differences (summary; owner document is `Architecture.md` §12)
 
- Image: static binary (`CGO_ENABLED=0`) built with the Go 1.26 toolchain on a distroless base; `readOnlyRootFilesystem` works unchanged; `/tmp` `emptyDir` is still needed for upload spooling.
- Memory: `GOMEMLIMIT` at about 75 % of the container limit replaces `MaxRAMPercentage=75`.
- Probes and metrics run on a second `http.Server` on the management port. Paths (`/actuator/...` or `/healthz`-style) are ADR O3.
- Shutdown: on `SIGTERM`, readiness turns false, then `http.Server.Shutdown` with a timeout below the pod's `terminationGracePeriodSeconds`, then background loops stop.
- No JVM warm-up before load tests.
---
 
## 13. Decisions and open points
 
**Owner decisions of 2026-11-08** (recorded in ADR-0001 and `Architecture.md` D57–D61):
 
| # | Decision | Status |
|---|---|---|
| G13 | Go 1.26 | [Decided] |
| A1 | oapi-codegen generates the standard (non-strict) server interface | [Decided] |
| A2 | Router is chi | [Decided] |
| A3a | sqlc spike first; result: usable with rules R1–R5 | Spike done; repeat in `B0-02` (OI-51) |
| A3b | Oversized JSON bodies answer 413 | Status [Decided]; limit and error code [Open] (OI-50) |
| A3c | minio-go `MaxRetries: 1` | Verified in source; test pending (OI-52) |
 
**Still open from ADR-0001:** G2 amendment (unknown fields ignored — this document assumes it is accepted), O1 (duplicate keys / case — the decoder in §3 is proposed, confirmation needed), O2 (generated code in coverage), O3 (probe paths).
 
OI-50, OI-51 and OI-52 are tracked in `docs/Open-items.md`.
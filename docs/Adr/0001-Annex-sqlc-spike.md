# ADR-0001 Annex — sqlc spike (A3a)
 
| | |
|---|---|
| Date | 2026-11-08 |
| Belongs to | `docs/Adr/0001-Backend-in-go.md` (G1), `docs/Architecture/Backend.md` §2.3 |
| Tooling | Go 1.26.10, sqlc v1.31.1 (built with `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`), engine `mysql` |
| Scope | **Parsing and code generation only.** No MariaDB server was started; no generated query was executed. The schema is the table definitions of `Backend.md` §10 written as MariaDB DDL. |
| Verdict | **sqlc is usable, with five rules (R1–R5).** Two failure modes are silent and must be caught by CI (R2). |
| Follow-up | PR `B0-02` repeats this on the real `V001__init.sql` and adds a runtime check against MariaDB (see §5). |
 
---
 
## 1. Schema constructs
 
| Construct | Result |
|---|---|
| native `UUID` column | **Rejected** — `syntax error near "UUID NOT NULL"` |
| `INET6` (and `INET4`) column | **Rejected** — same error |
| `CHAR(36)`, `VARCHAR(45)` | OK |
| `JSON` | OK (maps to `json.RawMessage`) |
| `CHECK` constraints, including `(status = 'ACCEPTED') = (accepted_at IS NOT NULL)` | OK |
| `DATETIME(0)`, `DATETIME(6)` | OK (`time.Time`) |
| `CHARACTER SET ascii`, `CHARACTER SET utf8mb4 COLLATE utf8mb4_bin` | OK |
| `BINARY(32)` | OK (`[]byte`) |
| `BOOLEAN NOT NULL DEFAULT 1` | OK |
| Foreign keys, `ON DELETE CASCADE` | OK |
| Reserved words as column names in backticks (`` `before` ``, `` `after` ``) | OK |
 
Only `UUID` and `INET6` fail. They are exactly the two types the architecture chose for MariaDB 10.7+ (D14, D16, D45).
 
## 2. Query cases
 
Each case was generated as its own sqlc package, so one failure cannot hide another.
 
| Case | Statement | Result |
|---|---|---|
| c01 | `SELECT * … WHERE id = ?` | OK |
| c02 | `SELECT … FOR UPDATE` | OK |
| c03 | `SELECT … LIMIT 50 FOR UPDATE SKIP LOCKED` | OK, kept verbatim |
| c04 | `INSERT … ON DUPLICATE KEY UPDATE` with `IF(…, 1, fail_count + 1)` | OK |
| c05 | Session statement: `UPDATE session s JOIN app_user u … SET s.last_access_at = IF(…)`, `NOW(6) - INTERVAL 60 SECOND`; `:execresult` | OK (returns `sql.Result`) |
| c06 | Visibility query with `OR EXISTS (…)` and a repeated `sqlc.arg(user_id)` | OK. The repeated argument is passed twice. `sqlc.arg(is_admin) = 1` is typed **`interface{}`** (see R4) |
| c07 | `LOWER(name) LIKE LOWER(?) ESCAPE '\\'` | OK |
| c08 | Keyset page with a row comparison `(occurred_at, id) < (sqlc.arg(a), sqlc.arg(b))` | **Silent failure** (see R2, R3) |
| c09 | `SELECT NOW(6)` | OK (`time.Time`) |
| c10 | `DELETE … LIMIT 1000` with `:execrows` | OK (`int64`) |
| c11 | `INSERT` into `audit_log` with `JSON`, `UUID`, `INET6` columns | OK (`json.RawMessage`, `uuid.NullUUID`, `db.NullIP`) |
| c12 | `COUNT(*)` | OK (`int64`) |
| c13 | Conditional `UPDATE … WHERE enabled = 1` with `:execrows` | OK |
| c14 | `WHERE ip = ? AND attempted_at > NOW(6) - INTERVAL 15 MINUTE` | OK (`db.IP`) |
| c15 | Sort with `CASE WHEN sqlc.arg(sort_key) = 'name' THEN name END` and `sqlc.narg(org)` | OK, but `sort_key` is typed **`interface{}`** (see R5) |
| c16 | `IN (sqlc.slice(categories))` | OK (expanded at run time) |
| c17 | `INSERT … NOW(6) + INTERVAL 10 MINUTE` | OK |
| c18 | Three-table join with a column alias | OK |
 
17 of the 18 generated packages compile and pass `go vet` on Go 1.26.10 (c08 was excluded because its output is known to be wrong).
 
### The two silent failures
 
sqlc reports **no error** and writes Go code that compiles, but leaves the `sqlc.arg(…)` text inside the SQL string and drops the parameter from the function signature. The statement would fail only when it runs.
 
| Written as | Generated SQL | Generated Go |
|---|---|---|
| `WHERE (occurred_at, id) < (sqlc.arg(cursor_time), sqlc.arg(cursor_id))` | the same text, unchanged | `ListAudit(ctx, limit)` — no cursor parameters |
| `WHERE sqlc.arg(is_admin) IS TRUE OR …` | the same text, unchanged | parameter missing |
 
Verified fixes:
 
| Fix | Result |
|---|---|
| `WHERE occurred_at < sqlc.arg(cursor_time) OR (occurred_at = sqlc.arg(cursor_time) AND id < sqlc.arg(cursor_id))` | `CursorTime time.Time`, `CursorID int64`, each passed twice where needed |
| `CAST(sqlc.arg(is_admin) AS SIGNED) = 1` | `IsAdmin int64` |
| `CAST(sqlc.arg(sort_key) AS CHAR)` / `AS CHAR(20)` / `CONCAT('', sqlc.arg(sort_key))` | still `interface{}` — no fix by casting |
 
## 3. Rules for the team
 
| # | Rule |
|---|---|
| **R1** | sqlc never reads the Flyway migrations directly. `make sqlc-schema` converts them with the script in §4 (`UUID` → `CHAR(36)`, `INET6` → `VARCHAR(45)`) into `db/sqlc/schema.sql` and writes the per-column Go type overrides into `sqlc.overrides.yaml`. Both files are generated; CI regenerates them and fails on a diff. |
| **R2** | CI fails when any generated `*.sql.go` contains the text `sqlc.arg`, `sqlc.narg` or `sqlc.slice` inside a SQL string (`grep -rn 'sqlc\.\(arg\|narg\|slice\)' internal --include=*.sql.go`). This catches both silent failures above. |
| **R3** | No row comparison `(a, b) < (…)`. Keyset pagination is written in the expanded form of §2. |
| **R4** | A flag passed to SQL is written `CAST(sqlc.arg(x) AS SIGNED)`; `sqlc.arg(…)` is never placed directly before `IS`. |
| **R5** | Dynamic `ORDER BY` (the list endpoint's `sort` and `order`) is **not** written in sqlc. It is hand-written in the module's `store` package: the column and direction come from a closed `switch` over the API enum, never from input text. Statements sqlc cannot express are written by hand in the same package with `database/sql`. |
 
Types that the overrides must provide in `shared/db` (the spike used a 30-line stub): `db.IP` and `db.NullIP`, both implementing `Scanner` and `Valuer`. `google/uuid` already provides `uuid.UUID` and `uuid.NullUUID`.
 
## 4. Conversion script (used in the spike)
 
```python
#!/usr/bin/env python3
"""Flyway schema -> sqlc-readable schema + column overrides.
UUID -> CHAR(36), INET6 -> VARCHAR(45); each converted column gets a Go type override.
Usage: gen_sqlc_schema.py <in.sql> <out_schema.sql> <out_overrides.yaml>"""
import re, sys
src, out_schema, out_overrides = sys.argv[1:4]
table = None; ov = []; lines = []
for line in open(src, encoding='utf-8'):
    m = re.match(r'\s*CREATE TABLE\s+`?(\w+)`?', line, re.I)
    if m: table = m.group(1)
    c = re.match(r'(\s*)`?(\w+)`?\s+(UUID|INET6)\b(.*)', line, re.I)
    if c and table:
        ind, col, typ, rest = c.groups()
        ov.append((table, col, typ.upper(), 'NOT NULL' not in rest.upper()))
        line = f"{ind}`{col}` {'CHAR(36)' if typ.upper()=='UUID' else 'VARCHAR(45)'}{rest}\n"
    lines.append(line)
open(out_schema, 'w', encoding='utf-8').writelines(lines)
y = []
for t, c, typ, nullable in ov:
    if typ == 'UUID':
        gt = '{import: "github.com/google/uuid", type: "NullUUID"}' if nullable else '"github.com/google/uuid.UUID"'
    else:
        gt = '{import: "<module>/internal/shared/db", type: "NullIP"}' if nullable else '"<module>/internal/shared/db.IP"'
    y.append(f'          - column: "{t}.{c}"\n            go_type: {gt}')
open(out_overrides, 'w').write('\n'.join(y) + '\n')
```
 
Limits of the script: it reads one column per line and one `CREATE TABLE` at a time. A column added by `ALTER TABLE … ADD COLUMN x UUID` in a later migration is **not** seen; the migration style (expand-then-contract) therefore needs either a rule that the script also handles `ADD COLUMN`, or a test that fails when a converted type is missing an override. This is decided in `B0-02`.
 
## 5. Not tested — to repeat in PR `B0-02`
 
1. The real `V001__init.sql` (Flyway comments, placeholders, `ALTER TABLE` statements).
2. Runtime behaviour against MariaDB 11.4 (OI-04): `uuid.UUID` text round-trip through the native `UUID` type; scanning an `INET6` value as `[]byte` or `string`; `RowsAffected()` with `clientFoundRows=true`; every statement of §2 executed once.
3. Pinning the sqlc version in CI (the spike used the version the module proxy returned as latest).
4. Whether the `sqlc.slice` expansion and `sqlc.narg` patterns are needed at all; the first modules may not use them.
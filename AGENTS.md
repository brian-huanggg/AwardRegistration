# Agent Instructions

Project: **ESG Idea Contest** registration system. Employees register, form teams of 1–4 and submit one Idea (one slide file per team); admins manage everything; every write is audited. The repo currently holds **documents only** — no `backend/` or `frontend/` code exists yet (first code PR is `B0-01` in `docs/PR-plan.md`).

## Where things are (read in this order)

| Need | File |
|---|---|
| What to build (requirements, permission matrix, NFRs) | `docs/PRD.md` — **wins over every other document on behaviour** |
| System design, principles, decision log (D1–D61) | `docs/Architecture/Architecture.md` |
| Backend design (Go): modules, pipeline, transactions, session/CSRF, audit, data model, tests | `docs/Architecture/Backend.md` |
| Frontend design | `docs/Architecture/Frontend.md` |
| Sequence diagrams (create / replace / delete / download, storage consistency) | `docs/Architecture/Sequence.md` |
| API contract (endpoints, error codes, response bodies) | `docs/API/` (start at `README.md`) |
| Why Go, and the (historical) sqlc spike | `docs/Adr/0001-Backend-in-go.md`, `docs/Adr/0001-Annex-sqlc-spike.md` |
| UI refactor to the Pencil design (`design/ui-design.pen`), one document per slice UI-01–UI-07 | `docs/UI-refactor/` (start at `README.md`) — **[Proposed]** |
| Unresolved questions (OI-nn) and owner confirmations | `docs/Open-items.md` — the only tracker for them |
| PR breakdown, slices S0–S7, PR rules and Definition of Done | `docs/PR-plan.md` (plan only; work items live in `bd`) |

Not written yet (do not cite as existing): `docs/modules/`, `docs/runbook.md`, `docs/test-report.md`, `docs/limitations.md`, root `README.md`, `backend/api/openapi.yaml`.

If documents disagree: PRD > ADR > Backend/Frontend > Architecture decision log; report the conflict, do not silently pick one. Status markers: **[Decided]** / **[Proposed]** / **[Open]** (open items in `docs/Open-items.md`). Do not treat **[Proposed]** as agreed.

## Stack (decided)

Go 1.26 · chi · spec-first OpenAPI (oapi-codegen, standard non-strict server) · native `database/sql` + `go-sql-driver/mysql` against MariaDB (hand-written SQL, no sqlc, no ORM) · golang-migrate migrations (embedded, `cmd/migrate`) · own session/CSRF/throttling middleware · `log/slog` · minio-go (`MaxRetries: 1`) · React + Vite + TypeScript frontend. Java/Spring text in older decision-log rows is superseded.

## Build & Test (target commands; they start to work when the code PRs land)

```bash
golangci-lint run
go test -race ./...                       # unit
go test -race -tags integration ./...     # needs Docker (testcontainers-go, real MariaDB + migrations)
go test -coverpkg=./... -coverprofile=cover.out ./... && go tool cover -func=cover.out   # gate: statement coverage >= 80 %
go generate ./...                         # must produce no diff
```

## Conventions that are easy to get wrong

- Packages by feature: `internal/<module>/` with implementation in `internal/<module>/internal/`; dependency direction is checked by an import-graph test and `depguard`. No ORM.
- Uniform 404: a resource the caller may not see answers the **same uniform 404** as a missing one (and is audited as DENIED). Do not add distinguishing responses.
- Database time is the only clock. A write succeeds only together with its audit row (track A inside the tx; track B `RecordRejected` in its own tx).
- Check order: 413 → 401 → CSRF 403 → 404 / role / state, all **before** the multipart body is read. JSON bodies: pointer fields, unknown fields ignored, duplicate keys and wrong types → 400.
- Unknown transaction outcome (`COMMIT` error not in SQLState class 23/40): keep storage objects; cleanup goes through `pending_file_deletion`.
- Every PR that adds an error code updates the catalogue, `testdata/error-codes.json`, `docs/API/error-codes.md` and the zh-TW/en dictionaries; every touched permission-matrix cell gets a test.
- Repo documents are English. No secrets, Vault paths or internal hostnames in `site/`.

This project uses **bd** (beads) for issue tracking. Run `bd prime` for full workflow context.

> **Architecture in one line:** Issues live in a local Dolt database
> (`.beads/dolt/`); cross-machine sync uses `bd dolt push/pull` (a
> git-compatible protocol), stored under `refs/dolt/data` on your git
> remote — separate from `refs/heads/*` where your code lives.
> `.beads/issues.jsonl` is a passive export, not the wire protocol.
>
> See [sync-concepts](https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md)
> for the one-screen overview and anti-patterns (don't treat JSONL as the
> source of truth; don't `bd import` during normal operation; don't
> reach for third-party Dolt hosting before trying the default).

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work atomically
bd close <id>         # Complete work
bd dolt push          # Push beads data to remote
```

## Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i` (interactive) mode on some systems, causing the agent to hang indefinitely waiting for y/n input.

**Use these forms instead:**
```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**Other commands that may prompt:**
- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:1105d646 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->

<!-- BEGIN BEADS CODEX SETUP: generated by bd setup codex -->
## Beads Issue Tracker

Use Beads (`bd`) for durable task tracking in repositories that include it. Use the `beads` skill at `.agents/skills/beads/SKILL.md` (project install) or `~/.agents/skills/beads/SKILL.md` (global install) for Beads workflow guidance, then use the `bd` CLI for issue operations.

### Quick Reference

```bash
bd ready                # Find available work
bd show <id>            # View issue details
bd update <id> --claim  # Claim work
bd close <id>           # Complete work
bd prime                # Refresh Beads context
```

### Rules

- Use `bd` for all task tracking; do not create markdown TODO lists.
- Run `bd prime` when Beads context is missing or stale. Codex 0.129.0+ can load Beads context automatically through native hooks; use `/hooks` to inspect or toggle them.
- Keep persistent project memory in Beads via `bd remember`; do not create ad hoc memory files.

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.
<!-- END BEADS CODEX SETUP -->

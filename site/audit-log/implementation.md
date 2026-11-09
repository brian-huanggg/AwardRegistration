# Audit Log: Implementation

Code lives in `backend/internal/audit/`.

| File | Content |
|---|---|
| `event.go` | `Action`, `Outcome`, `TargetType` value sets and the `Event` struct |
| `service.go` | `Service`, `Record`, `RecordRejected`, the `INSERT` |
| `sanitize.go` | Length limits, text cleaning, secret redaction |

## The Event

```go
type Event struct {
    Action  Action
    Outcome Outcome

    ActorRole  security.ActorRole // optional override
    ActorEmpNo string

    TargetType TargetType
    TargetID   uuid.UUID // uuid.Nil = none

    Method, Route string
    Status        int
    ErrorCode     string
    IP            netip.Addr
    UserAgent     string

    SubmittedEmpNo string

    Before json.RawMessage
    After  json.RawMessage
}
```

The request ID and the actor come from the request context. Set `ActorRole` and `ActorEmpNo` only where the context has no principal yet (for example a login that has just succeeded) or the actor is not the caller (`ADMIN_SYNC`, where the actor is `SYSTEM`).

## Track A: recording a successful write

`Record` needs the caller's transaction and refuses to run without one.

```go
_, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
    // 1. business write
    // 2. audit row in the same transaction
    return s.Audit.Record(ctx, tx, audit.Event{
        Action:     audit.IdeaUpdate,
        Outcome:    audit.Success,
        TargetType: audit.TargetIdea,
        TargetID:   id,
        Before:     beforeJSON,
        After:      afterJSON,
    })
})
```

If `Record` returns an error, return it from the function so `InTx` rolls back. Never ignore it.

## Track B: recording a rejection

Handlers do not call `RecordRejected`. They return a `*web.Error` that carries an audit reason. The central error adapter in `internal/server/errors.go` builds the event and calls the `RecordRejected` port. `RecordRejected` rejects an event with outcome `SUCCESS`, because success belongs to track A.

## What `insert` guarantees

Every row passes through one `insert` function, so these hold for all callers:

- Unknown action, outcome, target type or actor role is an error, never a stored value.
- A target ID without a target type is an error.
- Text from the client is **cleaned and cut by code point** to the column limit: invalid UTF-8 and control characters (including NUL and line breaks, which could forge log lines) are removed. A hostile header therefore cannot make the insert fail.

| Field | Limit |
|---|---|
| Employee ID | 64 |
| Method | 8 |
| Route | 200 |
| Error code | 64 |
| User agent | 256 |

- `before` and `after` are validated as JSON and compacted. **Secret keys are redacted by name** (compared after lower-casing and removing `_` and `-`): `password`, `token`, `csrf`, `session`, `sessionid`, `cookie`, `authorization`, `secret`, `content`, `filecontent` and similar. The value becomes `[REDACTED]`.
- A JSON document larger than 16 KiB is replaced by a marker rather than cut in the middle, because a cut document would be invalid JSON.
- `occurred_at` is the column default, so it is database time.

## Adding a new audited action

1. Add the constant to `event.go` and to the `actions` map.
2. Add the row to the event table in PRD §6.11 and decide its `before` / `after` content.
3. Call `Record` inside the transaction of the business write, or return a `*web.Error` with an audit reason for a rejection.
4. Add a test: the success path writes exactly one row, and a failing audit write rolls the operation back.
5. If the action is reachable by a permission-matrix cell, add or update the matrix test.

## Tests

| Test | Purpose |
|---|---|
| `sanitize_test.go` | Cutting, control characters, redaction, oversize JSON |
| `integration_test.go` | Both tracks against a real MariaDB |
| Architecture scan | No `UPDATE` or `DELETE` on `audit_log` anywhere under `internal/` |
| Matrix tests | Every touched permission-matrix cell, including `DENIED` rows |

Run the integration tests with `go test -race -tags integration ./...`. They need Docker.

## Current status

- Implemented: both tracks, the sanitizer, the schema, and audited actions for auth, Idea writes, downloads and the deadline.
- Not implemented yet: `GET /admin/audit-logs`, the hash chain, off-site copy.

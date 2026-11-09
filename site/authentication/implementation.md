# Authentication: Implementation

Paths are relative to `backend/internal/auth/`.

## Where the code lives

| Concern | File |
|---|---|
| Session create, validate, revoke; ID format and hashing | `session.go` |
| Session SQL (`Touch`, `Principal`, `Delete`, `DeleteExpired`), account reads | `internal/store/session.go` |
| Login flow, dummy hash, bcrypt comparison, HTTP handler | `login.go` |
| Throttle policy (key lock + IP limit) | `throttle.go` |
| Throttle SQL (`RecordFailure`, `LockRemaining`, …) | `internal/store/login_lock.go`, `login_attempt.go`, `login_throttle_audit.go` |
| Session middleware | `middleware.go` |
| CSRF signer, middleware, `GET /auth/csrf` | `csrf.go` |
| Logout | `logout.go` |
| `GET /auth/me` | `me.go` |
| Background purge | `purge.go` |

Only the `auth` module may import `auth/internal/store`, which is Go's `internal` rule. No other module touches the `session` table. Revoking all sessions of a user from the `account` module goes through the `SessionRevoker` interface, wired in `cmd/server/main.go`, so `account` never imports `auth`.

## Session ID

```go
raw := make([]byte, 32)
rand.Read(raw)                                   // crypto/rand
store.Insert(ctx, x, sha256(raw), userID)        // only the hash is stored
return base64.RawURLEncoding.EncodeToString(raw) // only this goes in the cookie
```

`parse` rejects anything that is not exactly 32 bytes of base64url **before any database call**, so garbage cookies cost nothing. `Sessions.CreateIn` takes a `*sql.Tx`, which is how login commits the session and its audit row together.

## The `Throttle` interface

```go
type Throttle interface {
    Check(ctx, key, ip) (retryAfter time.Duration, err error)
    AuditOnce(ctx, key, ip) (bool, error)
    Fail(ctx, key, ip) error
    Succeed(ctx, key) error
}
```

`Logins` depends on the interface, not on the table. `NoThrottle` is the stub used in unit tests; `DBThrottle` is the real one. A database error from any method fails the login **closed** with 500, so a broken counter never turns the limit off.

`RecordFailure` is one `INSERT … ON DUPLICATE KEY UPDATE` statement on database time. MariaDB evaluates the assignments left to right, so later columns see the new `fail_count`. An active lock is never extended by a concurrent attempt that passed `Check` just before the lock was set.

## The dummy hash

`NewLogins` computes a bcrypt hash of a fixed string once at startup. For unknown, badly formatted and disabled accounts, the code compares against that hash. Two details matter:

- bcrypt only reads the first 72 bytes. Newer `x/crypto` refuses longer input *immediately*, which would be faster and so distinguishable. `comparePassword` cuts the input to 72 bytes for the dummy comparison and returns false.
- A known account with a password over 72 bytes also never matches, and is reported as `password_too_long` in the audit row only. The client sees the usual 401. The registration policy blocks such passwords at sign-up.

## How refusals reach the audit log

`Login` returns a `*web.Error` that carries `AuditAction`, `AuditOutcome`, `AuditReason` and the submitted employee ID. The central error adapter in `internal/server` calls `RecordRejected` (track B, own transaction). The login code never calls it. Only success writes its row directly, inside the login transaction.

Audit reasons seen in the log: `bad_format`, `unknown_account`, `disabled`, `wrong_password`, `password_too_long`, `throttled`, `malformed_request`. The client gets none of them.

## Middleware wiring

```go
auth.Middleware(sessions, public, log)   // wraps the router
csrf.Middleware(fail, actionFor)         // after it, also before routing
```

`public` decides on method and path whether a request may run without a session (login, register, `GET /auth/csrf`, health). On a public route a valid session is still resolved, so the audit actor and the session-bound CSRF token are correct. A database error in validation answers 500, never a guess.

The principal is stored under an unexported context key. Handlers read it with `security.PrincipalFrom(ctx)`.

## Tests

| Test file | What it pins |
|---|---|
| `session_test.go`, `session_integration_test.go` | ID format, hash-only storage, both timeouts, refresh interval, `RowsAffected` counts matched rows |
| `login_*_test.go` | Same 401 for every refusal, one bcrypt per attempt, lock after 5, IP limit, audit once per window |
| `csrf_test.go` | Token round trip, binding mismatch, key rotation |
| `middleware_test.go` | 401 before 404, public routes |
| `logout_integration_test.go` | Idempotent 204, row removed with audit |
| `purge_*_test.go` | Batch delete, no effect on live rows |
| `logging_integration_test.go` | Passwords, session IDs and tokens never reach the logs |

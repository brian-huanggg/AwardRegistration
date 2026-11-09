# Authentication: Architecture

## The pieces

```text
Client
  │  Cookie: __Host-session=<opaque ID>     Cookie: __Host-csrf=<token>
  │  X-CSRF-Token: <token>   (writes only)
  ▼
Middleware chain (outside the router)
  1. Session middleware   → 401 UNAUTHENTICATED, or puts the Principal in the context
  2. CSRF middleware      → 403 CSRF_INVALID (non-GET/HEAD/OPTIONS only)
  ▼
Router → role / state checks → handler
```

The order is fixed: **413 → 401 → CSRF 403 → 404 / role / state**. Both middlewares run *before* routing and never read the body. That has two effects:

- A caller without a session learns nothing about which paths exist. An unknown path answers 401, not 404.
- A forged write is stopped before a multipart body is read.

## Two cookies, two jobs

| Cookie | Holds | Purpose |
|---|---|---|
| `__Host-session` | 32 random bytes, base64url | Identifies the session |
| `__Host-csrf` | `nonce.signature` | Second half of the double-submit check |

Both are `HttpOnly; Secure; SameSite=Lax; Path=/`. The `__Host-` prefix makes the browser refuse any cookie that lacks `Secure` or `Path=/`, or that sets a `Domain`. A sibling subdomain therefore cannot overwrite them.

The front end cannot read either cookie. It gets the CSRF token from the body of `GET /auth/csrf` and sends it back in `X-CSRF-Token`.

## Login

```text
POST /auth/login {empNo, password}
 │
 ├─ 1. Throttle.Check(empNo, ip)         locked? → 429 + Retry-After, no bcrypt
 ├─ 2. Format check ^[0-9]{6}$, load account by empNo
 ├─ 3. ONE bcrypt comparison
 │       found + enabled   → the stored hash
 │       unknown/disabled/bad format → the dummy hash
 ├─ 4. Not ok?  Throttle.Fail(); 401 INVALID_CREDENTIALS; audit FAILED (track B)
 └─ 5. ok:      Throttle.Succeed()
                BEGIN
                  INSERT session (id_hash, user_id)
                  INSERT audit AUTH_LOGIN SUCCESS
                COMMIT
                Set-Cookie: __Host-session=<new ID>      → 200 {empNo, name, email, role}
```

Points that are easy to miss:

- **Throttle first.** A locked key never reaches bcrypt, so an attacker cannot make the server burn CPU through a lock.
- **Same cost for every refusal.** Unknown, malformed, disabled and wrong-password cases all run one cost-10 comparison and return the same 401.
- **Session and audit row commit together.** If the audit insert fails, no session exists, so there is no login without evidence.
- **A new ID every login.** The client never chooses it, so fixation is not possible.

After login the client calls `GET /auth/csrf` again, because the session-bound token changed.

## Validating a request

```sql
UPDATE session s JOIN app_user u ON u.id = s.user_id
   SET s.last_access_at = IF(s.last_access_at < NOW(6) - INTERVAL 60 SECOND,
                             NOW(6), s.last_access_at)
 WHERE s.id_hash = ?
   AND u.enabled
   AND s.last_access_at > NOW(6) - INTERVAL 1800 SECOND     -- idle: 30 min
   AND s.created_at     > NOW(6) - INTERVAL 28800 SECOND    -- absolute: 8 h
```

One statement checks existence, both timeouts and the account state, and refreshes the idle timer. `RowsAffected() == 1` means valid. Then a `SELECT` reads `emp_no` and `role` from `app_user` to build the `Principal`.

- **Atomic.** Two replicas cannot disagree, and there is no read-then-write gap.
- **Database time.** `NOW(6)` is the only clock, so replicas with skewed clocks agree.
- **Refresh at most once per 60 s.** This cuts write load. The effective idle timeout is 29–30 minutes.
- **The role is never stored in the session.** Demote or disable an account and the next request sees it.

## CSRF: signed double-submit, bound to the session

Token format:

```text
base64url(nonce) "." base64url( HMAC-SHA256(key, binding ‖ nonce) )
binding = 0x01 ‖ SHA-256(session ID)     with a session
        = 0x00 "anon"                    without one
```

A write passes only if all of these hold:

1. The `X-CSRF-Token` header and the `__Host-csrf` cookie are both present.
2. They are equal (`subtle.ConstantTimeCompare`).
3. The HMAC verifies with the current key or the previous key.
4. The binding in the signature matches this request's session (or "anon").

Condition 4 is what a plain double-submit scheme lacks. An attacker who can plant a cookie in the victim's browser also knows the matching header value, because they chose both. But a token minted for the attacker's own session, or anonymously, fails against the victim's session binding.

Login and registration use the anonymous binding because no session exists yet. Their protection rests on the `__Host-` prefix and `SameSite`.

Keys come from the environment (a Kubernetes Secret). On rotation the old key stays as `previous` for 8 hours, which is one absolute session lifetime. Tokens carry no expiry, so removing the old key is what ends them.

## Throttling

Two independent limits, both stored in MariaDB so every replica shares them.

| Limit | Rule | Result |
|---|---|---|
| Per submitted employee-ID string | 5 consecutive failures, each within 15 min of the last → lock 15 min | 6th attempt gets 429 |
| Per source IP | 30 failures in a sliding 15 min | 429 until the oldest leaves the window |

- The key is the string as typed (control characters removed, cut to 32 characters). Counting works even if the account does not exist, so the lock does not reveal which IDs are real.
- During a lock, a correct password still gets 429 and does not extend the lock.
- A throttled attempt is not counted again.
- `Retry-After` is the larger of the two waits, at least 1 s.
- A throttled key or IP is audited **once per window** (`THROTTLED`), not once per request.

## Logout

`POST /auth/logout` deletes the session row and writes `AUTH_LOGOUT` in one transaction, clears the cookie, and returns 204. Without a session it still returns 204 (idempotent), and the CSRF check still applies.

## Background purge

Expired rows already fail every check, so the purge only limits table size. Every replica runs `Purger` every 5 minutes plus up to 1 minute of random jitter. It deletes in batches of 1000 from four tables: `session`, `login_lock`, `login_attempt`, `login_throttle_audit`. No leader is needed because each `DELETE` matches only rows that every reader already ignores.

## Data model

| Table | Key | Purpose |
|---|---|---|
| `session` | `id_hash` BINARY(32) | One row per live session: `user_id`, `created_at`, `last_access_at` |
| `login_lock` | `lock_key` | Failure count and `locked_until` per submitted ID |
| `login_attempt` | by IP | One row per failed attempt, for the sliding IP window |
| `login_throttle_audit` | key or IP | Claim that the throttled event was already audited |

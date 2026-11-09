# Authentication: Technical FAQ

## Concepts

### Is a session cookie a token?
Yes. It is an opaque token: a random value with no meaning of its own. The meaning is in the database row it points to. A JWT is a self-contained token: the meaning is in the value.

### Is OAuth2 an authentication protocol?
No. It delegates *authorization* and returns an access token. OIDC adds identity on top of it with an ID token. "Log in with X" is OIDC.

### Does this system use OAuth2, OIDC or JWT?
No. It uses employee ID + password and a server-side session. See [Decision Thinking](./decisions) A1 and A3.

### Can we add company SSO later?
Yes, and the change is local. The OIDC callback takes the place of `Logins.Login` and then creates the same `session` row. The middleware, CSRF, audit and permission checks do not change. The throttle and password code would not apply to SSO accounts.

## Login

### Why does a locked account still get 429 with the right password?
Because the lock is checked before the password is. Otherwise the answer would reveal whether the password was correct while locked.

### Why does the 5th failure return 401 and not 429?
The 5th failure sets the lock. The lock shows from the 6th attempt.

### A user says "my account is not locked but I get 429". Why?
The IP limit may apply: 30 failures from one address in 15 minutes. It affects everyone behind the same address, such as an office NAT. `Retry-After` shows how long to wait.

### Why can a typo in the employee ID lock something?
The counter keys on the string typed, and counts even if no such account exists. This hides which IDs are real. A mistyped ID locks that string, not a real person.

### Why not say "account disabled"?
The response must be the same for every failure, or it leaks which accounts exist. The audit row records `disabled` for administrators.

### Where does the client IP come from?
From `X-Forwarded-For`, trusting a configurable number of proxy hops (default 1). If the proxy count is wrong, every user may share one IP, or an attacker may forge theirs.

## Sessions

### How long does a session last?
Until 30 minutes without a request, or 8 hours after login, whichever comes first.

### Why is the idle timeout 29–30 minutes and not exactly 30?
The idle timer is refreshed at most once every 60 seconds to reduce writes.

### What if a user's role changes while they are signed in?
It applies on the next request. The role is read from `app_user` each time and is never copied into the session.

### How do I sign a user out everywhere?
`Sessions.RevokeAll(userID)` deletes all their rows. It runs through the `SessionRevoker` interface when an account is disabled. Disabling also blocks validation by itself, because the validation query requires `u.enabled`.

### Why is logout 204 even without a session?
It is idempotent. A client that lost its session still ends in the same state. The CSRF check still applies.

### Can one account have several sessions?
Yes. Each login creates its own row.

### Can the session ID leak into logs?
A test (`logging_integration_test.go`) checks that passwords, session IDs and tokens never reach the logs.

## CSRF

### Why not rely on `SameSite=Lax`?
It is a good second layer, but browser behaviour has gaps (for example, same-site attackers on a sibling subdomain). The token check does not depend on the browser's cookie policy.

### Why does the front end call `GET /auth/csrf` after login?
The token is bound to the session. A new session means a new binding, so the old token fails. The client refetches, and retries a write once on `CSRF_INVALID`.

### Why do login and logout need a CSRF token if there is no session yet?
Login-CSRF is real: an attacker could sign a victim into the attacker's account. Login uses an anonymous token, protected by the `__Host-` cookie and `SameSite`.

### How is the HMAC key rotated?
Make the new key current and the old key `previous`, which only verifies. Remove the old key after 8 hours, the length of an absolute session. Tokens have no expiry, so removing the key is what ends them.

## Passwords

### Why bcrypt and not Argon2id?
ADR G3 chose bcrypt for cost 10 and library maturity. Argon2id is the usual modern choice; switching would need a rehash on next login.

### Why is a password over 72 bytes refused?
bcrypt reads only the first 72 bytes. The policy blocks longer passwords at registration. At login, longer input never matches and is reported to the client as the usual 401.

### Why is there a dummy hash?
So an unknown account costs one bcrypt comparison like a known one. Without it, response time reveals which IDs exist.

## Operations

### What cleans up old sessions?
`Purger` runs on every replica every 5 minutes (+ up to 1 minute jitter), deleting in batches of 1000. It is safe on several replicas because it only deletes rows that every reader already ignores.

### Where are the timeouts configured?
They are constants in `session.go` and `throttle.go`, taken from the PRD. They are not environment settings.

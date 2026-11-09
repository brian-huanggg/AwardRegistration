# Authentication: Decision Thinking

This page records **why** authentication looks the way it does: the question, the options, the choice and what it costs. Entries marked *(documented)* follow reasons written in the code or design documents. Entries marked *(inferred)* are the likely reason read from the code; confirm them with the owner before quoting them as fact.

## A framework for authentication decisions

1. **Where must the truth live?** If a login must be revocable at once, the server has to own the state.
2. **What does an attacker gain from each stolen thing?** A password hash, a session row, a cookie, a CSRF key.
3. **Can the system tell a failure apart from the outside?** It must not, for logins.
4. **What happens when a dependency is down?** Security checks fail closed.
5. **Which clock?** One clock for every replica.
6. **Can a test pin it?**

## Decisions

### A1. Server-side session, not JWT *(documented, D41 / G11)*

| | |
|---|---|
| Question | How does the server remember a logged-in user? |
| Options | Opaque ID + database row; signed JWT; JWT + refresh token |
| Choice | Opaque ID + database row |
| Why | Logout and "disable account" must work immediately. One backend and one database already exist, so the lookup is cheap. Role changes apply on the next request. |
| Cost | One `UPDATE` per request (reduced by the 60 s refresh rule). Sessions are shared state in MariaDB. |

JWT's advantage is verifying without a shared store. That helps when many services must trust a token. It does not apply to a single application.

### A2. Store only SHA-256 of the session ID *(documented)*

| | |
|---|---|
| Question | What if the `session` table leaks? |
| Choice | Store `SHA-256(id)`; the raw ID exists only in the cookie |
| Why | The ID is 32 random bytes, so a fast hash is enough (no brute force of a low-entropy secret). A leaked table cannot be replayed as cookies. |
| Cost | The ID cannot be shown again to the user. Nothing needs that. |

A slow hash such as bcrypt would be wrong here. It adds cost to every request and gives nothing, since the input already has 256 bits of entropy.

### A3. No SSO, no OIDC, no OAuth2 in v1 *(documented, PRD §3.2)*

| | |
|---|---|
| Question | Should login use the company identity provider? |
| Options | Local accounts; OIDC against a corporate IdP |
| Choice | Local accounts with employee ID + password |
| Why | The PRD puts SSO, MFA and password reset out of scope. The project runs on a personal K3s cluster with no corporate IdP to integrate. |
| Cost | Users hold one more password. Password reset and MFA are not available. |

**If SSO is added:** use the authorization code flow with PKCE, verify `iss`, `aud`, `exp` and `nonce` on the ID token, then create the same session row as today. Throttling and the password code would be bypassed or removed for SSO accounts. Everything after the session is created stays the same.

### A4. Same 401, same cost for every login failure *(documented, ADR G3)*

| | |
|---|---|
| Question | What may a failed login reveal? |
| Choice | Nothing: unknown, malformed, disabled and wrong-password cases all return `INVALID_CREDENTIALS` after one bcrypt comparison |
| Why | Different messages or different timing let an attacker list valid employee IDs. |
| Cost | A user cannot be told "this account is disabled". Support must check the account. |

The audit log still records the real reason, because the audit log is for administrators and the response is for the caller.

### A5. Throttle by submitted string, plus by IP *(documented, OI-12 proposed)*

| | |
|---|---|
| Question | What do we count failures against? |
| Options | Account; source IP; both |
| Choice | Both, and the account counter keys on the *string typed*, not on a found account |
| Why | A key-only limit stops guessing one account. An IP-only limit is bypassed by many IPs. Counting unknown IDs keeps the lock from revealing which IDs exist. |
| Cost | An attacker can lock out a known employee ID for 15 minutes by failing five times. The limit trades that denial of service for guessing protection. The fixed 15-minute lock, with no extension, bounds the harm. |

*(inferred)* No CAPTCHA is used because the PRD excludes it.

### A6. Throttle counters in MariaDB *(documented)*

| | |
|---|---|
| Question | Where do counters live with several replicas? |
| Options | In-memory per pod; Redis; the database |
| Choice | The database |
| Why | No new component. Every replica sees the same count. The same clock (`NOW(6)`) applies. |
| Cost | A write per failed login. Acceptable, because failed logins are rare. |

### A7. A lock check before bcrypt *(documented)*

A throttled request skips password work entirely. Otherwise an attacker could use a locked key to spend server CPU.

### A8. Fail closed on throttle errors *(documented)*

If the counter cannot be read or written, login answers 500. A limit that silently turns off when the database is slow is not a limit.

### A9. Session-bound CSRF token, not plain double-submit *(documented, D40)*

| | |
|---|---|
| Question | How do we stop forged writes? |
| Options | `SameSite` only; synchronizer token stored per session; plain double-submit; signed double-submit bound to the session |
| Choice | Signed double-submit, bound to `SHA-256(session ID)` |
| Why | No per-token storage. Plain double-submit can be beaten by a cookie-planting attacker; the binding stops that. `SameSite=Lax` stays as a second layer. |
| Cost | The client must refetch the token after login and logout. Key rotation needs an 8-hour overlap. |

### A10. `__Host-` cookie prefix *(documented, PRD §6.5, Proposed)*

The browser enforces `Secure`, `Path=/` and no `Domain`. A sibling subdomain cannot plant or overwrite the cookie. This needs HTTPS outside `localhost`, which the personal K3s cluster provides through a TLS ingress.

### A11. Role is read per request, not stored in the session *(documented)*

A promotion or demotion applies at once. The cost is the second `SELECT` on each request. The PRD accepts it.

### A12. New session ID at every login, many sessions per account *(documented)*

A new ID prevents fixation. Concurrent sessions are allowed because an employee may use a laptop and a phone. "Log out everywhere" exists as `RevokeAll`, used when an account is disabled.

### A13. Idle 30 min, absolute 8 h, refresh at most once per 60 s *(documented, OI-11 proposed)*

The idle timer limits an unattended browser. The absolute timer limits a stolen cookie that is kept alive. The 60-second refresh rule turns a write on every request into a write at most once a minute, at the cost of a 29–30 minute effective idle window.

### A14. bcrypt cost 10 *(documented, PRD §6.4, OI-13)*

One fixed cost, shared by the real and the dummy hash. A higher cost slows the server under a login burst; a lower one weakens stored hashes. The dummy hash uses the same cost so timing matches.

### A15. Session and audit row in one transaction *(documented)*

Login without evidence is not allowed. If the audit insert fails, the session insert rolls back and the login fails.

### A16. Public routes decided before routing *(documented, PRD §6.1)*

An unknown path without a session answers 401, not 404. Only a signed-in caller learns that a path does not exist. This matches the uniform 404 rule used for resources.

## Rejected or deferred

| Option | Status | Reason |
|---|---|---|
| JWT access + refresh tokens | Rejected | No revocation, no benefit with one backend |
| Redis session store | Not needed | The database already exists |
| MFA / TOTP | Out of scope | PRD |
| Password reset by email | Out of scope | PRD |
| CAPTCHA | Out of scope | PRD |
| Argon2id | Not chosen | ADR G3 picks bcrypt; revisit with a hash-upgrade-on-login path if needed |

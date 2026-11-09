# Authentication: Concepts

Authentication answers one question: **who is making this request?** Authorization answers a different one: **what may they do?** This project keeps the two apart. The `auth` module proves identity and holds the session. The permission matrix and the uniform 404 decide what that identity may do.

This page explains the vocabulary first, because tokens, sessions, OAuth2 and OIDC are often mixed up. It then shows which of them this project uses. **It uses none of the token-based ones.** It uses employee ID + password and a server-side session behind an opaque cookie.

## Four words that are not the same thing

| Word | What it is | Answers |
|---|---|---|
| Authentication (AuthN) | Proving who you are (password, key, one-time code) | Who? |
| Authorization (AuthZ) | Deciding what an identity may do | May they? |
| Session | State the server keeps between requests after AuthN | Is this the same person as a minute ago? |
| Token | A value the client presents on each request | What do I hold as proof? |

A session needs a token of some kind, because HTTP is stateless. The cookie that carries the session ID **is** a token. So "session or token?" is the wrong question. The right question is: **where does the truth live — on the server, or inside the value the client holds?**

## Two ways to remember a login

| | Opaque token + server-side session | Self-contained token (JWT) |
|---|---|---|
| What the client holds | A random ID with no meaning | A signed document with claims (`sub`, `role`, `exp`) |
| Where the truth lives | A database row | In the token itself |
| Each request | Look up the row | Verify the signature |
| Revoke now (logout, disable account) | Delete the row. Instant. | Hard. The token stays valid until `exp` unless a deny list is kept. |
| Change a role | Takes effect on the next request | Takes effect when the token is re-issued |
| Needs a shared store | Yes | No |
| Fits best | One application with its own database | Many services that must verify without calling a central store |

JWT is **not** more secure than a session. It trades instant revocation for not needing a lookup. This system has one backend, one database and a requirement that disabling an account takes effect at once. The trade does not pay off here. See [Decision Thinking](./decisions).

## OAuth 2.0 and OIDC in one page

These two are about *delegating* login to someone else. They are the usual answer to "sign in with the company account".

**OAuth 2.0** is a protocol for **delegated authorization**. An app gets permission to act on a user's behalf at another service, without seeing the user's password. The result is an **access token**. The token says what the app may do. It does not say who the user is.

**OpenID Connect (OIDC)** is a thin identity layer **on top of OAuth 2.0**. It adds an **ID token** (a JWT) that says who the user is, and a standard `userinfo` endpoint. "Log in with X" is OIDC, not plain OAuth2.

| Term | Meaning |
|---|---|
| Resource owner | The user |
| Client | The application that wants access (here: this system, if it used SSO) |
| Authorization server / IdP | The service that logs the user in and issues tokens |
| Authorization code flow (+ PKCE) | The browser is redirected to the IdP. It returns a one-time `code`. The backend exchanges the code for tokens over a back channel. |
| Access token | Proof of permission to call an API |
| ID token | Signed JWT with the user's identity (OIDC only) |
| Refresh token | Long-lived value used to get new access tokens |
| Scope | The permission set requested, such as `openid profile email` |

```text
Browser            This system (client)          IdP (OIDC provider)
   │  GET /login            │                           │
   │───────────────────────▶│                           │
   │  302 → IdP /authorize?code_challenge=…&state=…     │
   │◀───────────────────────│                           │
   │  user signs in at the IdP ───────────────────────▶ │
   │  302 → /callback?code=…&state=…                    │
   │───────────────────────▶│  POST /token (code + verifier)
   │                        │──────────────────────────▶│
   │                        │◀─ ID token + access token ─│
   │                        │  verify signature, iss, aud, exp, nonce
   │  Set-Cookie: session   │  create OWN session
   │◀───────────────────────│                           │
```

Note the last step. Even with OIDC, most web applications **still create their own session** after the callback. OIDC replaces *how the password is checked*. It does not replace the session.

### What this project uses

None of it. The PRD lists company SSO, 2FA/MFA, password reset and email verification as out of scope (`docs/PRD.md` §3.2). An employee registers an account in this system, with a 6-digit employee ID and a password. Admin accounts come from a directory (Vault or a file).

If SSO is added later, the change is local. The OIDC callback would take the place of `Logins.Login`, and everything after it (the session row, the cookie, CSRF, the audit rows, the middleware) would stay as it is.

## Threats and where each is handled

| Threat | Defence in this project |
|---|---|
| Stolen password database | bcrypt cost 10; password policy with a common-password list |
| Guessing passwords | Per-account lock, per-IP limit, one bcrypt comparison per attempt |
| Finding which employee IDs exist | Same 401 and same timing for unknown, disabled and wrong-password cases |
| Stolen session cookie from script (XSS) | `HttpOnly` cookie; the server stores only a SHA-256 of the ID |
| Stolen session from the database | The table holds hashes, so a leaked row cannot be replayed |
| Session fixation | A new ID at every login |
| CSRF | Double-submit token signed with HMAC and bound to the session; `SameSite=Lax` |
| Replay after logout or disable | The row is deleted or the account is disabled; the next request fails |
| Long-lived sessions | 30 minutes idle, 8 hours absolute |

## Enterprise practice compared with this project

| Topic | Common enterprise practice | This project |
|---|---|---|
| Identity source | SSO through OIDC or SAML to a corporate IdP | Local employee ID + password |
| Second factor | MFA required | Out of scope (PRD) |
| Session state | Server session or short JWT + refresh token | Server session in MariaDB |
| Password storage | Argon2id or bcrypt, with pepper and breach checks | bcrypt cost 10, common-password list |
| Brute-force defence | Rate limit, CAPTCHA, adaptive risk | Key lock + IP limit; no CAPTCHA |
| CSRF | Framework default (synchronizer token) | HMAC double-submit token bound to the session |
| Cookie hardening | `Secure`, `HttpOnly`, `SameSite` | Same, plus the `__Host-` prefix |
| Audit | Login events sent to a SIEM | Rows in the audit table, same transaction |

See [Architecture](./architecture) for the flows, [Implementation](./implementation) for the code, and [Decision Thinking](./decisions) for why it is built this way.

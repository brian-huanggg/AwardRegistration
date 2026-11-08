# Frontend Architecture
 
| | |
|---|---|
| Version | 0.5 (draft; aligned with `Backend.md` 0.3 and PRD 1.5) |
| Date | 2026-11-08 |
| Companion of | `docs/PRD.md` v1.5, `docs/API/` (`api-endpoints.md`, `error-codes.md`, `response-bodies.md`), `docs/Architecture/Architecture.md` v0.5, `docs/Architecture/Backend.md` v0.3, `docs/Architecture/Sequence.md` v0.3 |
| Status markers | **[Decided]** agreed in discussion · **[Proposed]** drafted by the author, awaiting confirmation · **[Open]** unresolved (tracked in `docs/Open-items.md`) |
 
This document says how the front end is built to meet the PRD. Requirements stay in the PRD; the API contract (endpoints, error codes, response bodies) is in `docs/API/`; system-level decisions stay in the architecture overview; back-end design is in `Backend.md`. The front end is a TypeScript SPA (React + Vite) served from the same origin as the API (PRD §6.1, §8.6).
 
Changes in 0.5:
- §6 rewritten as "Routes and pages": one login entry for Admins and Users (§6.1); **no `/admin/ideas` route**, Idea list and detail are shared and Admin-only actions are driven by `can()` (§6.2); a route table with an access rule per route; a role mismatch shows the **same 404 page** as an unknown route (§6.3); rules for the `next` parameter (§6.4); role-driven layout (§6.5); no prefetch or focus refetch of the Idea detail for Admins, because every Admin detail view is audited (§6.7);
- §1, §3, §5.4, §13 adjusted to match; F16–F20 added.
Changes in 0.4:
- open points (former §14 table) moved to `docs/Open-items.md`; references use OI-nn;
- API contract references point to `docs/API/` (error codes, response bodies);
- session mechanism reference points to `Backend.md` §6 (PRD §6.5 now holds requirements only).
Changes in 0.3:
- session-bound CSRF tokens (§5.2);
- response bodies and `expectedFilenameBase` (§4, §5.5);
- 400 versus 422 handling (§5.6);
- email pre-check (§5.6);
- millisecond timestamps (§5.3);
- idle-timeout warning proposed (§8);
- open points refreshed;
- F13–F15.
Changes in 0.2: companion documents updated; feature folders mapped to the revised back-end modules (§1); 413-first decision applied (§5.2); handling of an unknown write outcome added (§5.2, §5.5); decisions F11–F12 added.
 
---
 
## 1. Principles
 
1. **Feature-based split aligned with the back-end modules.** **[Decided]** Mapping to `Backend.md` §2:
   | Front-end feature | Back-end module(s) |
   |---|---|
   | `auth` | `account` (registration, password policy) + `auth` (login, session, CSRF) |
   | `idea` | `idea` |
   | `team` | `team` |
   | `slides` | `slides` (the upload/download API) and the file rules implemented by `storage` (size, type, expected filename) |
   | `deadline` | `deadline` |
   | `admin` | `admin` (Admin-only pages: deadline setting, bonus statistics). Admin operations on Ideas are not here: they use the ordinary `idea` and `slides` endpoints and live in those features (§6.2) |
   `shared` (front end) is the counterpart of `shared` (back end): it depends on no feature.
2. **The server is the single source of truth.** Permissions, submission state and time are decided by the back end. The UI performs only a first-line check for usability and handles every server rejection gracefully. **[Decided]**
3. **The contract is the single source of truth.** API types come from the OpenAPI snapshot; rules come from `testdata/`; error codes come from `testdata/error-codes.json`. Nothing is copied by hand. **[Decided]**
4. **TypeScript only**, including tool configuration (`vite.config.ts`, ESLint config). A CI check fails the build if `frontend/src/` contains `.js` or `.jsx` files (the PRD does not require this check; we add it). **[Proposed]**
5. **CSP friendly.** The PRD asks for a restrictive Content-Security-Policy (`default-src 'self'; frame-ancestors 'none'`, PRD §8.3), so no runtime CSS-in-JS, no inline scripts, self-hosted fonts. **[Proposed]**
6. **No hidden state in the browser.** No tokens in `localStorage`; the session lives in an HttpOnly cookie, the CSRF token in memory. **[Proposed]**
---
 
## 2. Technology choices **[Proposed]** (OI-24)
 
| Area | Choice | Reason |
|---|---|---|
| Build | Vite + React + TypeScript (`strict`) | Decided (D11) |
| Routing | React Router (data router) | Loaders, guards, lazy routes |
| Server state | TanStack Query | Cache, invalidation, retries; no global store needed |
| Forms | React Hook Form + own validators from `shared/text` | Field-level mapping of `errors[]` |
| i18n | react-i18next (or Lingui) | zh-TW / en; errors localized by `code` |
| UI primitives | Radix UI or React Aria (unstyled) | Keyboard and ARIA behaviour built in (WCAG) |
| Styling | Tailwind or CSS Modules | Build-time CSS, compatible with CSP |
| API types | `openapi-typescript` from the committed OpenAPI snapshot | Prevents drift |
| Upload | `XMLHttpRequest` (or axios) | `fetch` has no upload progress; PRD §8.5 requires a progress bar |
| Unit / component tests | Vitest, Testing Library, MSW, `vitest-axe` | |
| E2E | Playwright (+ `@axe-core/playwright`) | Also serves as the DEV smoke test (PRD §8.4) |
| Charts (bonus) | A small chart library, lazy-loaded | Table alternative required (§8) |
 
---
 
## 3. Structure
 
```
frontend/src/
  app/            entry, providers, router, route table (§6.2), route guards, AppShell, NotFound page, error boundary
  features/
    auth/         login, register, me, csrf, password-rule UI, idle-timeout warning
    idea/         list + filters, detail, create/edit forms, delete (all roles; actions follow can())
    team/         join request, approve / reject / remove, leave, my requests
    slides/       upload component, expected filename, download
    deadline/     useDeadline, countdown, state banner
    admin/        Admin-only pages: deadline setting, (bonus) stats; no Idea pages
  shared/
    api/          http client, problem+json parsing, CSRF, generated types
    text/         NFC, Unicode White_Space, code-point counting (§5.1), email rule
    i18n/         dictionaries, error-code mapping
    ui/           design-system components (Button, Field, Dialog, Toast, Table, Pagination)
    permissions/  can(action, context)
    time/         serverTime offset, local-time formatting
  testdata -> ../../testdata   (Vite alias; shared with the back end)
```
 
Dependency rules, enforced by `eslint-plugin-boundaries` or `dependency-cruiser` in CI (the counterpart of the back end's import-graph test and `depguard` rules, `Backend.md` §2.4):
 
- `shared` depends on no feature.
- A feature exposes only what its `index.ts` exports; other features never import its internals.
- `admin` may use other features; no feature imports `admin`.
- `app/` may import every feature. Admin-only additions to a shared page (for example the bonus Excel export button on the Idea list) are composed there and passed to the page as props or slots **[Proposed]**, so `idea` never imports `admin`.
---
 
## 4. Contract layer
 
| Contract | Source | Front-end use |
|---|---|---|
| Request / response types | OpenAPI snapshot in `docs/API/` (bodies described in `docs/API/response-bodies.md`) | Generated into `shared/api/types`; CI regenerates and fails on a non-empty diff |
| Text clean-up | `testdata/text-cleanup.json` | Table-driven unit tests for `shared/text` |
| Email | `testdata/` email cases | Same cases for the register form pre-check |
| Password policy | `testdata/` password cases (violations in PRD §6.4 order) | Same cases for the live rule checklist |
| Error codes | `testdata/error-codes.json` (`{code, status}` only; human-readable table in `docs/API/error-codes.md`) | Generates a TS union; every code must have zh-TW and en text in the dictionaries, or the build fails |
| Permission matrix | `testdata/permission-matrix.json` | The UI-relevant cells verify `can()` |
| Expected slide filename | `testdata/expected-filename.json`, including NFD uploads **[Proposed]** | Shared cases for the filename builder |
| Schemas | `testdata/schema/*.schema.json` | CI validates every `testdata/` file |
 
---
 
## 5. Core modules
 
### 5.1 `shared/text`
 
- Implements PRD §6.2 exactly: NFC → whitespace handling by the Unicode `White_Space` set (25 code points) → control-character check.
- `String.prototype.trim` MUST NOT be used: its whitespace set differs (it strips U+FEFF and keeps U+0085).
- Length is counted in code points. The HTML `maxlength` attribute counts UTF-16 units and is not used; forms show a `n/75` counter instead.
- Per-field error order follows PRD §6.2 (`REQUIRED` → `INVALID_FORMAT` → `INVALID_NAME_CHARS` → `OUT_OF_RANGE`), so client and server show the same first error.
- Email (PRD §6.3): trim by the `White_Space` set, then reject any whitespace **[Decided]**, then printable ASCII only **[Proposed]** (OI-14), exactly one `@` with non-empty sides, ≤ 254 characters, ASCII lower-casing.
### 5.2 `shared/api`
 
- One `ApiError` type parsed from `application/problem+json` (envelope in `docs/API/error-codes.md`), keeping `code`, `errors[]`, `requestId`.
- **413 first [Decided]:** a transport-level size limit may answer 413 before any other check (PRD §6.1), and that response may **not** be `problem+json` (it can come from the ingress). Non-JSON error bodies are mapped from the HTTP status to the matching code (413 → `FILE_TOO_LARGE`). The UI also pre-checks the 5 MiB limit, so this is rare.
- **CSRF [Decided: session-bound tokens]** (PRD §6.5, D40):
  - The token is fetched lazily from `GET /auth/csrf`, cached in memory and sent as `X-CSRF-Token` on every write.
  - It is bound to the current session, or anonymous before login, so it MUST be refetched after login, after logout and after any 401.
  - On `CSRF_INVALID` the client refetches and retries once. The request was rejected before processing, so the retry is safe. This also covers a token that became stale because the session expired.
- **401:** clears the `me` cache and the CSRF token, and redirects to `/login?next=…` with a message that the session ended.
- **429:** reads `Retry-After` and shows a countdown on the login form.
- **400 `MALFORMED_REQUEST`:** indicates a client bug (missing field, wrong type). It is shown as a generic error with `requestId`, never mapped to a form field.
- **405 / 406:** generic error with `requestId` (client bug).
- **503 `STORAGE_UNAVAILABLE`:** a local error on upload/download; the rest of the app keeps working. The write did not happen.
- **503 `SERVICE_UNAVAILABLE` on a write that carries a file (create, replace):** the outcome may be **unknown** (`Sequence.md` §2). The database commit may have succeeded although the response was lost.
  - The UI must not claim failure. It tells the user the result is uncertain, refreshes the Idea list or detail, and only then offers a retry.
  - If a retried create returns 409 `IDEA_DUPLICATE`, the message explains that the Idea may already have been created by the earlier attempt and links to the list.
- `requestId` is shown in the error details so users can quote it to support.
### 5.3 `deadline`
 
- `GET /deadline` returns `serverTime` (millisecond precision); the client keeps `offset = serverTime − clientNow`.
- The countdown advances with `performance.now()`, so changing the system clock does not affect it.
- Refetch at zero, on window focus, and whenever a write returns `SUBMISSION_CLOSED` or `SUBMISSION_NOT_OPEN`.
- The front-end state only disables controls and drives banners; it never replaces the server check.
- Display uses the browser's time zone (`Intl.DateTimeFormat`). The admin form converts `datetime-local` to ISO 8601 with offset; the server rejects values without an offset (422) and years outside 2000–2099.
### 5.4 `permissions`
 
- `can(action, { me, idea, state })` is a pure function.
  - Leader: `idea.leader.empNo === me.empNo`.
  - Member: present in `idea.members`.
  - Admin: `me.role === 'ADMIN'`. On the shared Idea pages an Admin gets download, replace slides and delete (not limited by the deadline); never create, edit text, or any team action (PRD §7).
  - List items carry `myRelation` (`LEADER`, `MEMBER` or null), so list rows need no extra lookup. It is null for Admins.
- `idea.pendingRequests` is `null` for callers who may not see it, and an array (possibly empty) for the Leader and Admins.
- Hiding a control is never a security measure. A 403 or 404 caused by a wrong UI guess is handled like any other error.
### 5.5 `slides`
 
- The server returns `expectedFilenameBase` (without extension) in the Idea detail (`docs/API/response-bodies.md`, OI-26). The UI appends `.pdf` or `.pptx` and shows both with a copy button.
- For the create form (no Idea yet), the UI builds the same base locally: `{I|O}_{org}_{leaderName}_{leaderEmpNo}_{ideaName}`, by string concatenation, verified against `testdata/expected-filename.json`.
- The uploaded name is NFC-normalized before comparison; the extension is compared case-insensitively. No regular expression parsing.
- Pre-check order mirrors the back end:
  1. file count;
  2. size (5 MiB);
  3. not empty;
  4. magic bytes (`%PDF-` or ZIP);
  5. filename.
  The server remains the final judge; its `FILENAME_MISMATCH` response carries `expected`. The server validates everything **before** it stores the file (PRD §6.8), so a rejected upload never leaves anything behind.
- When text fields are invalid, the server does not validate the file; the UI shows text errors first.
- Download uses `fetch` → blob → `<a download>`, with the filename taken from `Content-Disposition` (`filename*`).
  - Failures (404, 503) show a normal error message instead of a raw JSON page.
  - If the received size is smaller than `Content-Length`, the download is reported as incomplete.
- After a create or replace whose outcome is uncertain, see §5.2 (503 `SERVICE_UNAVAILABLE`).
- The file is **not** renamed automatically before upload **[Proposed]** (OI-21): renaming would weaken the purpose of the filename rule. The UI only shows the expected name with a copy button.
### 5.6 Forms and error display
 
- `errors[].field` maps to form fields; the top-level `code` drives the summary message.
- The server returns one error per field (first in PRD §6.2 order), except `password`, which lists every violated rule.
- UI text is looked up as `errors.<CODE>`.
- The password form shows a live checklist of the 8 rules, matched to the rule ids of `PASSWORD_POLICY`. The Top-10,000 list is loaded as a separate chunk only on the register page, or that one rule is left to the server.
- Optional fields are never sent as `null` for required fields; a required field is always sent (empty string if empty), so the server answers 422 `REQUIRED` rather than 400.
---
 
## 6. Routes and pages
 
### 6.1 One login entry **[Decided]**
 
Admins and Users use the **same login page and the same endpoint** (`POST /auth/login`, PRD §4.2). There is no separate Admin entry.
 
- The role comes from the account on the server; the URL a person logs in from decides nothing, so a second entry would add no security.
- Every login failure is the same 401 `INVALID_CREDENTIALS`, and unknown accounts run a dummy bcrypt comparison (PRD §6.4, §6.5). A dedicated Admin entry would advertise that Admin accounts exist, which works against that design.
- Throttling is shared, so a second entry would not change the known lock-out weakness (PRD §10).
- After login, both roles go to `/ideas`; the role decides what the page shows. The header shows an "Administrator" badge for Admins so they do not mistake the interface for the User view. `me.email` is `null` for Admins and is not shown.
### 6.2 Route table
 
The routes live in one table (`app/routes.ts`). Each route declares an `access` value: `public`, `USER`, `ADMIN` or `ANY` (any logged-in role). Guards, navigation and `next` validation all read this table, so they cannot disagree.
 
| Path | Page | Access | Notes |
|---|---|---|---|
| `/` | Redirect | – | Visitor → `/login`; logged in → `/ideas` |
| `/login` | Login | public (logged in → `/ideas`) | Shows the deadline banner (`GET /deadline` is public); supports `?next=` (§6.4) |
| `/register` | Register | public (logged in → `/ideas`) | Success → `/login` with a notice carried in navigation state, **not** in the URL; no automatic login |
| `/ideas` | Idea list | ANY | User: own Ideas (led or member of). Admin: all Ideas. Filters, `sort`, `order` and page live in the URL; `q` is debounced; `size ≤ 100`; date filters sent as instants (`[from, to)`) |
| `/ideas/new` | Create | USER | |
| `/ideas/:id` | Idea detail | ANY | Members, pending requests (Leader and Admin only), slides, team actions, replace and delete dialogs; a missing or not-visible Idea shows the 404 page |
| `/ideas/:id/edit` | Edit text fields | USER | A Member (not Leader) is redirected to the detail page; an unrelated User gets the 404 page from the loader |
| `/join-requests` | Apply and my requests | USER | Apply form (Leader employee ID + Idea name); my pending requests with withdraw (by `ideaId`) |
| `/admin/deadline` | Set the deadline | ADMIN | |
| `/admin/stats` | Statistics (bonus) | ADMIN | Lazy-loaded together with the chart library |
| `*` | 404 page | – | |
 
**No `/admin/ideas` route [Decided].** The API has no such endpoint either: an Admin lists, views, downloads, replaces and deletes Ideas through the ordinary `/ideas` endpoints (`docs/API/api-endpoints.md`). The same list and detail pages serve both roles, and the actions an Admin sees come from `can()` (§5.4). Operations that are not separate pages (replace slides, delete or dissolve, approve, reject, remove, leave) are dialogs and inline actions on the detail page.
 
### 6.3 Access rules
 
1. **Identity:** the root loader fetches `me` once (`ensureQueryData`). A 401 means visitor.
2. **Visitor on a non-public route** → `/login?next=…`. This applies to every non-public path, `/admin/*` included, so the redirect reveals nothing.
3. **Logged in on `/login` or `/register`** → `/ideas`.
4. **Role mismatch → the same 404 page as an unknown route [Decided].** One component, same title, same text, no hint that the route exists. This mirrors the back end, which answers an unknown path with 404 once a session exists (PRD §6.1). The API itself answers 403 for a wrong role (PRD §7); the UI deliberately does not show a 403 page.
5. **Resource level:** the detail loader maps 404 to the same 404 page. "Does not exist" and "not yours" look identical (PRD §7 rule 3).
6. **Submission state is not a guard.** The deadline can change at any time, so pages stay reachable. A banner explains the state and write controls are disabled (§5.3). For example `/ideas/new` opens in `CLOSED`, with the submit button disabled.
A table-driven test runs over the route table: a visitor is redirected to login, a User on `/admin/*` and an Admin on `/ideas/new` or `/join-requests` get the 404 page, and every route's access value matches the navigation items shown for each role.
 
### 6.4 The `next` parameter
 
- It carries the original path and query when a visitor is sent to `/login`, and after a 401 (§5.2).
- Accepted only as a same-origin relative path: it starts with a single `/`, and contains no `//` prefix, no backslash and no scheme.
- It is resolved against the route table. If no route matches, or the logged-in role has no access to it, it is ignored and the user goes to `/ideas`. A User who logs in with `next=/admin/deadline` therefore lands on the list instead of a 404 page.
### 6.5 Layout
 
- `PublicLayout` for login and register: minimal header, language switch, deadline banner.
- `AppShell` for everything else, **one component** whose navigation is derived from the route table and the role:
  - User: Ideas, Join requests, user menu.
  - Admin: Ideas, Deadline, (bonus) Statistics, an "Administrator" badge, user menu.
- The deadline banner sits in both layouts so every page shows the submission state.
### 6.6 Page states
 
Every page has loading, empty and error states (the error shows `requestId`). Where it applies there is also the "result uncertain" state (§5.2). The Idea list distinguishes "no Ideas yet" from "no results for these filters".
 
### 6.7 Admin views and the audit log
 
Every `GET /ideas/{id}` by an Admin writes an `IDEA_VIEW` audit row (PRD §6.11). To keep the log meaningful:
 
- No hover prefetch of the Idea detail for Admins.
- `refetchOnWindowFocus` is off for the detail query for Admins, and there is no polling of it.
- Writes return the updated resource (`docs/API/response-bodies.md`) and the response is written straight into the cache (§7), so no follow-up GET is needed.
### 6.8 Code splitting
 
The `/admin/*` routes and the chart library are lazy-loaded, so ordinary Users never download them.
 
---
 
## 7. State management
 
- Server state: TanStack Query. Query keys follow the resource (`['ideas', filters]`, `['idea', id]`, `['deadline']`, `['me']`).
- Writes return the updated resource (`docs/API/response-bodies.md`), which is written straight into the cache; affected list keys are invalidated. Team actions invalidate the Idea detail and the join-request list.
- Local UI state stays in components. No global store. **[Proposed]**
---
 
## 8. Accessibility (WCAG 2.1 AA basics, PRD §8.5)
 
### 8.1 What we check
 
**Forms**
- Every field has a programmatically associated `<label>`; required state is announced, not only shown by colour.
- Errors are linked with `aria-describedby` and `aria-invalid`; on a failed submit an error summary receives focus and links to each field (WCAG 3.3.1, 3.3.3).
- `autocomplete` attributes on login and registration fields (`username`, `current-password`, `new-password`, `email`, `name`) (1.3.5).
- Password show/hide is a real button with an accessible name and state.
- Destructive actions (delete / dissolve an Idea, remove a member) use a confirmation dialog (3.3.4).
**Keyboard and focus**
- Every function works by keyboard; no keyboard trap (2.1.1, 2.1.2).
- Visible focus indicator on every control (2.4.7); logical tab order; a "skip to content" link.
- Dialogs trap focus while open, close with Esc and return focus to the trigger.
- After a route change, focus moves to the page heading and `document.title` updates (2.4.2).
**Structure and semantics**
- Landmarks (`header`, `nav`, `main`), one `h1` per page, no skipped heading levels.
- Data tables use `<th scope>` and a caption; pagination is a `<nav>` with `aria-current="page"`.
- Buttons for actions, links for navigation.
**Dynamic content**
- Status messages (saved, deleted, error toast) use `aria-live` (4.1.3): polite for success, assertive for errors.
- **The countdown is not announced every second.** The deadline is also shown as static text; only coarse changes (for example "less than 1 hour left", "closed") are announced.
- Upload progress uses `role="progressbar"` with `aria-valuenow`; loading regions use `aria-busy`.
- The number of results after a filter change is announced through a live region.
**Colour, contrast, layout**
- Contrast at least 4.5:1 for text and 3:1 for UI components and focus indicators (1.4.3, 1.4.11).
- Meaning is never carried by colour alone: the `OPEN` / `CLOSED` banner and status badges use text and an icon too (1.4.1).
- Reflow at 320 CSS px and 200 % zoom without two-dimensional scrolling (1.4.4, 1.4.10); text-spacing overrides do not break layouts (1.4.12).
- `prefers-reduced-motion` is respected; `forced-colors` mode is checked.
- Interactive targets are at least 24 × 24 CSS px (our own rule; WCAG 2.2 level, not required by 2.1).
**Language**
- `<html lang>` follows the UI locale (3.1.1); mixed-language content (for example CJK names inside the English UI) carries its own `lang` (3.1.2).
**Charts (bonus)**
- Every chart has a data-table alternative and a short text summary; series are distinguishable without colour (patterns or direct labels).
**Files and sessions**
- The upload control is a native file input; drag-and-drop is only an extra. The expected filename and the copy button are readable by assistive technology.
- **Idle timeout (2.2.1) [Proposed]** (OI-22)**:** a warning dialog appears 28 minutes after the last server request. "Continue" calls `GET /auth/me`, which refreshes the session. The warning is shown 2 minutes early because, with the 60-second refresh rule (OI-11), the server-side idle time can be up to 1 minute ahead of the client. Without a response, the session ends as before: the `next` route is preserved and a clear message explains the sign-out.
### 8.2 How we check
 
| Layer | Tool | Gate |
|---|---|---|
| Static | `eslint-plugin-jsx-a11y` | CI fails on error |
| Component | `vitest-axe` on shared UI components and forms | CI fails on serious or critical violations |
| Page | Playwright + `@axe-core/playwright` on login, register, list, detail, create, join requests, admin; both locales; desktop and phone viewport | CI fails on serious or critical violations |
| Manual, per release | Keyboard-only walkthrough; screen-reader smoke test (NVDA + Chrome only **[Proposed]**, OI-23); 200 % zoom and 320 px reflow; forced-colors; reduced-motion | Checklist result recorded in `docs/test-report.md` |
 
Automated tools find only part of the problems (roughly a third), so the manual checklist is part of the definition of done for each user-facing feature.
 
---
 
## 9. Security
 
- No `dangerouslySetInnerHTML`; user text is rendered as text; the summary uses `white-space: pre-wrap`.
- No secrets in the bundle; no environment-specific values baked in (same origin, no API base URL).
- CSP, `Referrer-Policy` and `X-Content-Type-Options` are set by the web server (PRD §8.3, **[Proposed]**); HSTS by the ingress.
- Cookies are never read by script: the session and `__Host-csrf` cookies are both HttpOnly. The CSRF token is read from the `GET /auth/csrf` response body (`Backend.md` §6).
- Dependency and image scans run in CI.
---
 
## 10. Internationalization
 
- Dictionaries for zh-TW and en; the default follows the browser, the user can switch, and the choice is remembered per browser.
- Errors, enum labels (category, org, ESG direction) and state names are looked up by code, never by server `detail` (`docs/API/error-codes.md`).
- Official zh-TW labels and Org full names come from the owner (OI-17). Working drafts until then:
  - categories: 酷點子獎 / 卓越影響獎;
  - directions: 綠色製造 / 打造負責任供應鏈 / 營造健康共融職場 / 人才培育 / 關懷弱勢.
- Dates and numbers use `Intl`; names are counted in code points and support CJK and emoji.
---
 
## 11. Build and deployment
 
- Multi-stage Docker build: Node build stage, then an unprivileged nginx serving static files with SPA fallback.
- Hashed assets: long-lived `immutable` cache; `index.html`: `no-cache`.
- One image for K3S and DEV (build once, deploy many); no runtime configuration needed because the API is same-origin.
- Ingress routes `/` to this container and `/api` to the back end; the ingress body-size limit is 6 MiB (PRD §6.8, OI-06).
---
 
## 12. Testing and CI gates
 
| Gate | Detail |
|---|---|
| Type check and lint | `tsc --noEmit`, ESLint (boundaries, a11y, no-JS rule) |
| Unit and component tests | Vitest; table-driven tests over `testdata/`; MSW for API behaviour (including a non-JSON 413, a 503 `SERVICE_UNAVAILABLE` on create, and a `CSRF_INVALID` refetch-and-retry after login); the route-access test of §6.3; a test that an Admin opening an Idea detail sends exactly one `GET /ideas/{id}` (§6.7) |
| Contract drift | Regenerate API types from the OpenAPI snapshot; fail on a diff |
| i18n completeness | Every error code and UI key present in zh-TW and en |
| Accessibility | §8.2 |
| Bundle budget | Initial JS size limit; admin and chart chunks excluded |
| Security | Secret scan, dependency audit, image scan |
| E2E smoke (DEV) | Login → create Idea with slides → list → download → delete (`docs/PR-plan.md`, slice S3) |
 
---
 
## 13. Delivery phases
 
1. **P0 Foundation:** Vite, strict TS, `shared/api`, `shared/text`, i18n, CI pipeline, generated API types, route table with access values, `AppShell`, 404 page.
2. **P1 Auth:** register, login (with throttle countdown), `me`, logout, global 401 handling, session-bound CSRF handling, `next` validation, redirect of logged-in users away from login and register, idle-timeout warning.
3. **P2 Deadline and read-only Ideas:** countdown and banner, list with filters, detail.
4. **P3 Idea writes and slides:** create (multipart, progress, filename pre-check), edit, delete, replace, download.
5. **P4 Team:** apply, approve, reject, remove, leave, my requests.
6. **P5 Admin:** deadline page; Admin actions (download, replace, delete) on the shared Idea list and detail pages.
7. **P6 Hardening:** accessibility pass, E2E, performance.
8. **Bonus** (after PRD §8.1 is satisfied): statistics, Excel export, optimistic-lock conflict handling (412), pending-request badge, join-request rate limit (429).
---
 
## 14. Open points
 
Tracked in `docs/Open-items.md`. Front-end items: OI-24 library choices (§2); OI-21 no automatic renaming (§5.5); OI-22 idle-timeout warning (§8.1); OI-23 screen-reader matrix (§8.2); OI-17 official zh-TW labels (§10); OI-26 `expectedFilenameBase` and the other response bodies (§5.5).
 
Resolved:
- 413 vs state-403 precedence for oversized bodies: 413 first, no draining (D39); the UI pre-checks size and maps a non-JSON 413 (§5.2).
- Separate Admin login entry: no, one login page and endpoint (§6.1, F16).
- Separate `/admin/ideas` route: dropped, shared Idea pages (§6.2, F17).
- Role mismatch on a route: the same 404 page as an unknown route (§6.3, F18).
---
 
## 15. Decision log
 
| # | Decision | Rationale | Status |
|---|---|---|---|
| F1 | Feature-based packages aligned with back-end modules (mapping in §1) | Traceability to requirements and tests | Decided |
| F2 | Server is the single source of truth; UI checks are first-line only | Security and consistency | Decided |
| F3 | API types, text rules, error codes and permission matrix come from the contract (OpenAPI + `testdata/`) | No drift between front end and back end | Decided |
| F4 | TanStack Query for server state; no global store | Little client-only state | Proposed |
| F5 | Own text clean-up, never built-in `trim` or `maxlength` | Must equal PRD §6.2 | Proposed |
| F6 | Upload through `XMLHttpRequest` | Progress bar | Proposed |
| F7 | Download through fetch → blob | Friendly errors | Proposed |
| F8 | Unstyled accessible primitives + build-time CSS | WCAG support and CSP | Proposed |
| F9 | Accessibility gated by lint, axe (component and page) and a manual per-release checklist | Automation covers only part of WCAG | Proposed |
| F10 | One same-origin image for all environments | Build once, deploy many | Proposed |
| F11 | A non-JSON 413 (transport limit, answered before any other check) is mapped from the HTTP status to `FILE_TOO_LARGE` | Follows back-end D39 | Decided |
| F12 | A 503 `SERVICE_UNAVAILABLE` on create/replace is shown as "result uncertain": refresh first, then offer retry; a later 409 `IDEA_DUPLICATE` is explained as a possible earlier success | Follows back-end D36 (unknown transaction outcome) | Proposed |
| F13 | CSRF token refetched after login, logout and 401; one refetch-and-retry on `CSRF_INVALID` | Tokens are bound to the session (D40) | Decided |
| F14 | Expected filename taken from `expectedFilenameBase` in the Idea detail; built locally only on the create form | One implementation of the rule on the server; shared test cases for the local copy | Proposed (OI-26) |
| F15 | Idle-timeout warning at 28 minutes; "Continue" calls `GET /auth/me` | WCAG 2.2.1; covers the 60-second refresh rule (D42) | Proposed (OI-22) |
| F16 | Admins and Users share one login page and endpoint; both land on `/ideas`; the header shows an "Administrator" badge | PRD §4.2; a separate entry adds no security and would reveal that Admin accounts exist | Decided |
| F17 | No `/admin/ideas` route; Idea list and detail are shared and Admin actions come from `can()` | The API has no such endpoint; one set of pages to build, test and keep accessible | Decided (owner, 2026-11-08) |
| F18 | A route the role may not use shows the same 404 page as an unknown route; no 403 page | Does not reveal that admin routes exist; mirrors the back end's uniform 404 | Decided (owner, 2026-11-08) |
| F19 | `next` accepted only as a same-origin relative path that resolves to a route the role may use; otherwise `/ideas` | Prevents open redirects and a 404 flash after login | Proposed |
| F20 | Submission state is a banner and disabled controls, never a route guard | The deadline can change at any time | Proposed |
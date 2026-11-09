# UI-02 — Login

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.2` |
| Pencil | Frame `ZziSC` "Login" (1440 × 960) |
| Code | `app/pages/auth.tsx` (`Login`, `Register`), `features/auth/LoginForm.tsx` |
| Spec | PRD §6.3–6.4 (login, throttling), `Frontend.md` §5.2, §6.4 |
| Depends on | UI-01 tokens; countdown block depends on bead `i5o.8` (F2-02) |
| Status | **[Proposed]** |

## Goal

Two-panel login: a dark green brand panel on the left with the contest pitch and submission countdown, and the sign-in form on the right. The behaviour of `LoginForm` (validation, throttle countdown, `next` handling, notices) does not change.

## Design summary

**Brand panel** (left half, deep green gradient, white text):

- Brand row: logo square with Lucide `leaf` + "ESG Idea Contest".
- Pitch: eyebrow "2026 ESG AWARD · IT DIVISION" (small caps, letter-spaced), headline "Small teams. Real impact." (large, bold), body "Form a team of up to four, submit one Idea with your slides, and help …".
- Deadline block (bottom, translucent card): state dot + "Submissions open", "Closes Mon, Nov 30, 2026 · 18:00 GMT+8", and four countdown cells (days, hours, minutes, seconds).

**Form side** (right half, light background):

- Top right: language button (`languages` icon, "EN / 中文").
- Registered notice (green alert, `circle-check`): "Account created. Sign in with your employee ID."
- Heading "Sign in", description "Use your 6-digit employee ID and password."
- Fields: Employee ID, Password.
- Full-width primary button "Sign in".
- Footer: "New here? Create an account".

## Implementation

| Area | Change |
|---|---|
| `auth.tsx` `Login` | Replace the current panel layout (`pages.login.panelTitle` / `panelBody`) with a two-column grid. The left column is a new `LoginBrandPanel`. The right column keeps `LoginForm` and the notices. |
| Notices | The `registered` and `sessionEnded` notices from router state render as the design's alert above the heading. `role="status"` for registered, `role="alert"` for session ended (keep the roles the tests use today). |
| Heading | "Sign in" stays the page `<h1>` (`nav.signIn`). The design's description maps to `pages.login.description`; update the en value to "Use your 6-digit employee ID and password." and the zh-TW value to match. |
| Language button | Reuse `LanguageSwitch` from UI-01. Visitors need it most, so it must be on the login page. |
| Button | The frame shows a `plus` leading icon on "Sign in", which looks like a component default left on by mistake. Use no icon, or `log-in`. Confirm with the designer. |
| Register link | Keep the existing link to `/register` (`pages.login.noAccount` + `nav.register`). Change the link text to "Create an account" only if zh-TW gets an equivalent. |
| Register page | `Register` uses the same two-column layout and brand panel, so the two public pages match. No separate design exists. |

**Brand panel copy** (new keys under `pages.login.brand.*`, both locales): eyebrow, headline, body. The eyebrow "IT DIVISION" is organisation-specific. Confirm it with the owner, because `site/` and the repo must not expose internal details (`AGENTS.md`). The app is internal, so this is a copy decision rather than a security one.

**Deadline block.** Reads `useDeadline()` (public endpoint, works for visitors). It shows state text and the formatted deadline in the user's time zone. The four ticking cells need F2-02 (bead `i5o.8`): until that lands, render the block without the cells. Never tick from `Date.now()` alone. The cells are `aria-hidden`, and one visually hidden sentence ("Closes in 21 days 6 hours") is updated at most once per minute, so screen readers are not flooded.

**Contrast.** White and light text on the dark green gradient must pass 4.5:1 at the lightest point of the gradient. Check the eyebrow and body text (smaller and lighter) with a contrast tool.

## Responsive behaviour

- ≥ 1024 px: two columns, 50 / 50.
- < 1024 px: the brand panel stacks above the form and shrinks to brand row + headline + deadline state (no body, no cells). The form comes first in reading order only if the owner asks for it. By default the panel stays first, as in the frame.
- 320 px: no horizontal scroll, the countdown text wraps.

## Design vs contract

| Item | Decision |
|---|---|
| "Forgot password" | Not in the design and not in the product (PRD: no password reset). Do not add it. |
| Countdown cells | Wait for F2-02. |
| Time zone label "GMT+8" | Format with `Intl.DateTimeFormat` `timeZoneName: 'shortOffset'` in the user's zone, not a fixed string. |

## Tests

- `app/pages/login.test.tsx` keeps passing. Update queries that depended on `panelTitle` / `panelBody`.
- New:
  - The brand panel shows the deadline state for NOT_OPEN, OPEN, CLOSED and a deadline that is not set.
  - The registered and session-ended notices render in the alert slot.
  - The language button switches language on the login page.
  - axe passes for both locales.
- `register.test.tsx` keeps passing with the shared layout.

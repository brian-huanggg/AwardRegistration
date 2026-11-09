# UI-01 — App Shell and Theme Tokens

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.1` |
| Pencil | Sidebar component `O:PV1ln` (instances in `VpC8a`, `rsQOw`, `UfPqk`, `eoK6b`, `qSvEA`); Top Bar `RZBjS` in frame `VpC8a` |
| Spec | `Frontend.md` §6.5 (layouts), §6.2 (route table), §8 (accessibility) |
| Status | **[Proposed]** |

## Goal

Every signed-in page renders inside one `AppShell`: a fixed left sidebar, a top bar with breadcrumb, submission state and language switch, and a padded content area. Today `app/layouts.tsx` `Screen` wraps every page in a bare `<main>`, and nothing lets the user sign out or change language after login. `Frontend.md` §6.5 already asks for this shell.

## Design summary

**Overall frame.** 1440 × 960. Sidebar 256 px on the left, main column fills the rest. Main column = Top Bar (56 px) followed by Content (padding 32, vertical gap 24).

**Sidebar** (`fill #fafafa`, right border `#e4e4e7`, padding 8, gap 16):

| Part | Content |
|---|---|
| Header | 32 px green square logo with Lucide `leaf`; "ESG Idea Contest" (14/500) over "2026 ESG Award" (12/400); `chevrons-up-down` icon on the right |
| Section label | 12/500 muted text: "Contest" for User, "Administration" for Admin |
| Nav item | 16 px icon + label 14/400, padding 6 × 8, radius 6. Active item has `fill #f4f4f4` (`--sidebar-accent`) |
| Footer | 32 px round avatar with the first character of the name; name (14/500) over "`empNo` · Role" (12/400); `log-out` icon button |

Nav items by role:

| Role | Items (icon) |
|---|---|
| User | My Ideas (`lightbulb`) → `/ideas`; New Idea (`plus-circle`) → `/ideas/new`; Join requests (`user-plus`) → `/join-requests` |
| Admin | All Ideas (`lightbulb`) → `/ideas`; Deadline (`calendar-clock`) → `/admin/deadline`; Statistics (`chart-column`) → `/admin/stats` |

**Top Bar** (height 56, padding 0 × 32, bottom border):

- Left: breadcrumb, for example "Contest › My Ideas". Section crumb is muted 14/400, current crumb 14/500, separator `chevron-right` 14 px.
- Right: deadline pill and language button, gap 12.
  - Deadline pill: radius full, padding 6 × 12, 8 px dot + state (13/600) + countdown (13/400), for example "Submissions open · closes in 21d 06h 14m". The frame shows the OPEN colours: green tint fill `#16a34a14`, border `#16a34a40`, text `#15803d`.
  - Language button: ghost button, Lucide `languages` icon, label "EN / 中文".

## Implementation

**Components** (all in `frontend/src/app/`, since `Frontend.md` §4 places the AppShell in `app/`):

| Component | Notes |
|---|---|
| `AppShell` | Skip link → sidebar → main column. Owns the one `<main id="main">`. |
| `Sidebar` | `<nav aria-label={t('nav.main')}>` with a list of `NavLink`s. Active link gets `aria-current="page"` from `NavLink`. |
| `TopBar` | Breadcrumb, `DeadlinePill`, `LanguageSwitch`. |
| `Breadcrumb` | `<nav aria-label="Breadcrumb">` containing an `<ol>`. Last item has `aria-current="page"` and is not a link. |
| `UserMenu` (sidebar footer) | Name, employee ID, role, sign-out button. |

**Route table.** Navigation keeps coming from `app/routes.ts` (`navFor(role)`), so guards and nav cannot disagree (`Frontend.md` §6.2):

- Add `navKey: 'ideaNew'` to `/ideas/new`. Its access is `USER`, so Admins never see it.
- Add an `icon` field to `RouteDef`, or keep a `navKey → icon` map next to `Sidebar`. The map is preferred, because it keeps `routes.ts` free of React imports.
- The table-driven test "every route's access value matches the navigation items shown for each role" must cover the new item.

**Labels.** The design says "My Ideas" for Users and "All Ideas" for Admins. That matches PRD §6.6 (Users see only Ideas they lead or belong to, Admins see all). Add `nav.myIdeas` and `nav.allIdeas` and choose by role. Keep `nav.ideas` as the generic page title. New keys: `nav.ideaNew`, `nav.sectionContest`, `nav.sectionAdmin`, `nav.breadcrumb`, `nav.switchLanguage`. Reuse the existing `nav.signOut` and `nav.administrator`.

**Breadcrumb.** Derived from the matched route. No page has to pass it in.

| Route | Crumbs |
|---|---|
| `/ideas` | Section › My Ideas / All Ideas |
| `/ideas/new` | Section › My Ideas (link) › `pages.ideaCreate.title` |
| `/ideas/:id` | Section › My/All Ideas (link) › `pages.ideaDetail.title` |
| `/ideas/:id/edit` | Section › My Ideas (link) › `pages.ideaEdit.title` |
| `/join-requests`, `/admin/deadline`, `/admin/stats` | Section › page label |
| 404 | Section › `notFound.title` |

The Idea name is not shown in the breadcrumb. The shell would have to fetch the Idea, which writes an `IDEA_VIEW` audit row for Admins (`Frontend.md` §6.7). The page `<h1>` already shows the name.

**Deadline pill.** Replaces the full-width `DeadlineBanner` strip for signed-in pages. It keeps the banner's contract: `role="region"` with `aria-label={t('deadlineBanner.label')}`, an icon plus text for state (never colour alone), and the loading and error texts. The pill has three state variants: OPEN (green), NOT_OPEN (neutral / amber), CLOSED (muted / red tint). Text on each tint must pass 4.5:1.

The countdown in the pill ("closes in 21d 06h 14m") needs the server-time countdown from bead `i5o.8` (F2-02: `serverTime` + `performance.now()`), which is still open. Until it lands, the pill shows the formatted deadline (`deadlineBanner.until`) in place of the countdown. When F2-02 merges, the pill switches to its hook. A countdown must not tick from the client clock alone (`AGENTS.md`: the database time is the only clock).

**Language switch.** A single button that toggles between `zh-TW` and `en` via `setLocale()` in `shared/i18n`. Accessible name: `t('nav.switchLanguage')`. The visible "EN / 中文" label carries `lang` attributes on each part so screen readers pronounce it correctly. The choice is already persisted by `setLocale`.

**Sign out.** Calls `logout()` from `features/auth/login.ts`, then `replaceIdentity(client, null)` from `app/session.ts`, then navigates to `/login`. `replaceIdentity` sets `leaving = "signedOut"`, so the route gate redirects to plain `/login` without `next`. If `logout()` fails because of the network, still clear the identity locally and go to `/login` (the server session expires on its own). Show a busy state while the request runs, and do not allow double submission.

**Router.** In `app/router.tsx` `Gate`, render `AppShell` when `me` is not null and the route is not public. Visitors (login, register, and a 404 seen by a visitor) keep the bare `Screen`. `PageHeading` keeps moving focus to the `<h1>` on route change. Its `mb-6` margin moves into the content area gap.

**Admin page.** `app/pages/admin.tsx` renders its own `DeadlineBanner` inside the page. Leave it until UI-07, which replaces it with the "Current deadline" card.

**Theme tokens** (`frontend/src/app/index.css`):

| Token | Today | Change |
|---|---|---|
| `--sidebar-foreground` | `oklch(0.5452 0 0)` (grey) | Same as `--foreground`: the design uses near-black nav text |
| `--sidebar-accent` | `oklch(0.9461 0 0)` | Keep (≈ `#f4f4f4` in the design) |
| `--sidebar` | `oklch(0.9911 0 0)` | Keep (≈ `#fafafa`) |
| `--primary` | green, 4.5:1 checked | Keep (see README, primary colour contrast) |
| `--font-sans` | `Outfit, sans-serif` (not loaded) | `Inter, ui-sans-serif, system-ui, sans-serif`, plus `'Noto Sans TC'` / `'PingFang TC'` / `'Microsoft JhengHei'` for zh-TW glyphs. Inter is used only if installed or self-hosted, never from a third-party CDN. |
| new `--success`, `--success-foreground`, `--warning`, `--warning-foreground` | – | For pills and badges in this and later slices. Contrast-check each pair. |

## Responsive behaviour

- ≥ 1024 px: sidebar always visible, 256 px.
- < 1024 px: the sidebar is hidden. The top bar shows a menu button (`aria-expanded`, `aria-controls`) that opens the sidebar as a drawer. Focus moves into the drawer, `Esc` closes it and returns focus to the button.
- < 640 px: the breadcrumb shows only the current crumb. The pill drops the date and keeps state text and icon.

## Design vs contract

| Item in the design | Decision |
|---|---|
| Sidebar "Help" section: Slide naming guide, User manual | Not built. No route or document exists. Candidate target: the public `site/` (bead `7ma.5`, F7-01). File a follow-up bead if the owner wants the links. |
| `chevrons-up-down` in the sidebar header (workspace switcher) | Decorative only. There is one contest, so it is dropped. |
| Countdown in the pill | Waits for F2-02, see above. |
| Admin footer "Admin 陳主任" | Shows `me.name` plus the `nav.administrator` badge, which `Frontend.md` §6.5 requires. |

## Tests

- `AppShell.test.tsx`:
  - User and Admin see exactly `navFor(role)` items, in order, with the right labels.
  - The active link has `aria-current="page"`.
  - The skip link moves focus to `#main`.
  - Sign out calls `POST /auth/logout`, clears cached queries and lands on `/login` without `next`. Sign out still lands on `/login` when the request fails.
  - The language button switches `document.documentElement.lang` and the visible text.
  - The pill shows state text for each of NOT_OPEN, OPEN, CLOSED, loading and error.
  - Breadcrumb crumbs per route (table-driven).
  - Drawer: opens, traps focus, closes on `Esc`.
- `app/App.test.tsx`: the axe runs over every route still pass with the shell in place.
- Page tests that query `getByText('Ideas')` or similar may now match twice (nav and heading). Scope those queries to `main` or to the heading role.

## Out of scope

Page content restyling (UI-02 to UI-07), the countdown hook (F2-02), and dark mode visuals beyond keeping the `.dark` token block valid.

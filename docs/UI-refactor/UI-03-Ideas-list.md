# UI-03 — Ideas List

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.3` |
| Pencil | Frame `VpC8a` "Ideas — User", content node `mgFWo` |
| Code | `app/pages/ideas.tsx` (`IdeaList`), `features/idea/IdeaList.tsx`, `features/idea/list.ts` |
| Spec | PRD §6.6 (list, filters, sort), `Frontend.md` §6.6 (empty states), §6.7 (Admin views), `shared/permissions/can.ts` |
| Depends on | UI-01 |
| Status | **[Proposed]** |

## Goal

Replace the current filter form and table with the design's page header, compact filter bar and data table. Query parameters, URL state, paging, `can()`-driven actions and the empty, error and loading states stay.

## Design summary

**Page header.** Title "My Ideas" with the description "Ideas you lead or have joined as a member. One Idea = one team of up to 4 …". Actions on the right: outline button "Join a team" (`user-plus`) and primary button "New Idea" (`plus`).

**Filter bar** (one row):

- Search input with a `search` icon, placeholder "Search Idea name…".
- Four dashed "faceted filter" buttons with a `circle-plus` icon: Org, Category, ESG direction, Created.
- Spacer.
- Sort button with an `arrow-down-wide-narrow` icon: "Newest first".

**Table** (bordered card):

| Column | Content |
|---|---|
| Idea | Name (500) with a one-line muted summary below it |
| Your role | Badge: "Leader" (filled green) or "Member" (outline) |
| Category | Icon + label: `sparkles` Cool Idea, `trophy` Outstanding Impact |
| Org | Code, for example BSID |
| ESG direction | Label |
| Team | Overlapping avatars with initials + "3/4" |
| Created | Date |
| (actions) | `chevron-right` |

**Footer.** "Showing 1–5 of 5 Ideas · 3 led by you", with "Page 1 of 1" and previous / next icon buttons on the right.

## Implementation

| Area | Change |
|---|---|
| Header actions | "New Idea" links to `/ideas/new`, "Join a team" links to `/join-requests`. Both are User-only: hide them for Admins with `can()`, as the current create button does. Admins see the title "All Ideas" (`nav.allIdeas`) and a matching description. |
| Search | Keeps the `q` parameter and the current debounce / submit behaviour. Accessible name from a visually hidden label (`pages.ideaList.search`), not only the placeholder. |
| Faceted filters | Each opens a popover (Radix `Popover`, from the installed `radix-ui`) with options as a checkbox or radio list. **Single choice per filter**, because the API takes one value for `org`, `category` and `direction`. "Created" opens two date inputs (from / to) that map to `createdFrom` / `createdTo` as today. A selected filter shows its value inside the button and a clear control. "Clear" for all filters stays (`pages.ideaList.clear`). |
| Sort | Menu with four options mapped to the API: Newest first (`sort=createdAt&order=desc`), Oldest first (`createdAt asc`), Name A–Z (`name asc`), Name Z–A (`name desc`). New keys under `pages.ideaList.sort.*`. Write them to the URL like the other parameters. |
| Table | Keep a real `<table>` with `<caption>` (visually hidden) and `<th scope="col">`. The whole row is not a click target. The Idea name is the link to `/ideas/:id`, and the chevron is decorative (`aria-hidden`) inside the same link or omitted. This keeps one tab stop per row. |
| Your role | From `myRelation` (`LEADER` / `MEMBER`). The column is hidden for Admins (`myRelation` is null). Badge text, not colour alone. |
| Category | Icon + `enums.category.*` label. |
| Team | `memberCount` of the list item + 1 (the Leader), shown as "n/4". Text only, see **Design vs contract**. |
| Created | `Intl.DateTimeFormat(locale, {dateStyle: 'medium'})`. |
| Footer | "Showing {from}–{to} of {total}" from `page`, `size`, `total`. Keep the per-page selector (`pages.ideaList.perPage`) in the footer, because the design's footer has room for it and the feature exists. Pagination keeps `<nav aria-label={t('pages.ideaList.paging')}>`. |
| Admin columns | Admins see a "Leader" column (`fields.leader`, name + employee ID) in place of "Your role". The design has no Admin frame, so this follows `Frontend.md` §6.7 and the current list. |
| Admin actions | Keep the existing download, replace and delete actions from `can()` (F6-01) as a row action menu (`ellipsis` icon button with an accessible name naming the Idea). No hover prefetch of detail for Admins (`Frontend.md` §6.7). |

## Responsive behaviour

- ≥ 1024 px: full table.
- 640–1023 px: hide the ESG direction and Created columns, and wrap the filter bar onto two rows.
- < 640 px: the table becomes a list of cards (name, role badge, category, n/4), one per Idea. The cards use the same data and the same link.

## Design vs contract

| Item in the design | Contract | Decision |
|---|---|---|
| One-line summary under the Idea name | `IdeaSummary` has no `summary` field | Not shown. Adding it is an API change; file a follow-up bead if wanted. |
| Team avatars with initials | `IdeaSummary` has `memberCount` only, no member names | Show "n/4" text only. |
| "3 led by you" in the footer | No count from the API; only the current page is known | Drop it, or count only within the current page with wording that says so. Default: drop. |
| Multi-select faceted filters (shadcn data-table pattern) | One value per filter | Single choice. |
| "Newest first" sort | `sort` + `order` | Supported, see the table above. |

## Tests

- `app/pages/ideaList.test.tsx` keeps passing. Update queries for the new filter controls (popover options instead of native selects).
- New:
  - Each faceted filter writes its parameter to the URL and the request, and clears it again.
  - Sort writes `sort` and `order`.
  - The "Your role" badge for LEADER and MEMBER, and the column is absent for Admins.
  - "n/4" uses `memberCount + 1`.
  - Header actions are hidden for Admins.
  - Card layout at a narrow viewport (`matchMedia` mock), or cover it in Playwright.
  - "No Ideas yet" vs "no results for these filters" (`Frontend.md` §6.6).
  - axe on filled, empty and no-match states.

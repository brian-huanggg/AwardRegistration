# UI Refactor to the Pencil Design

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-09 |
| Design source | `design/ui-design.pen` (open only through the Pencil MCP tools; the file is encrypted) |
| Epic | `ESG-Award-Registration-System-u35` (beads) |
| Status | **[Proposed]**: every decision in these documents waits for owner review |

This folder holds one implementation document per slice. Each slice is one PR, built in its own git worktree, and changes only how the existing pages look. Behaviour, API calls, permissions, i18n coverage and accessibility stay as they are unless a slice document says otherwise.

If these documents disagree with `docs/PRD.md` or `docs/Architecture/Frontend.md`, those documents win (see `AGENTS.md`). Gaps between the design and the API are listed in each slice under **Design vs contract**. They are never closed by inventing data on the client.

## Slices

| Slice | Bead | Pencil frame | Pages touched | Depends on |
|---|---|---|---|---|
| [UI-01](UI-01-App-shell.md) App shell and theme tokens | `u35.1` | Sidebar `O:PV1ln`, Top Bar in `VpC8a` | every signed-in page | – |
| [UI-02](UI-02-Login.md) Login | `u35.2` | `ZziSC` | `/login` (and `/register` inherits the panel) | UI-01 tokens |
| [UI-03](UI-03-Ideas-list.md) Ideas list | `u35.3` | `VpC8a` | `/ideas` | UI-01 |
| [UI-04](UI-04-Idea-detail.md) Idea detail | `u35.4` | `rsQOw` | `/ideas/:id` | UI-01 |
| [UI-05](UI-05-Idea-form.md) New and Edit Idea | `u35.5` | `UfPqk` | `/ideas/new`, `/ideas/:id/edit` | UI-01 |
| [UI-06](UI-06-Join-requests.md) Join requests | `u35.6` | `eoK6b` | `/join-requests` | UI-01 |
| [UI-07](UI-07-Admin-deadline.md) Admin deadline | `u35.7` | `qSvEA` | `/admin/deadline` | UI-01 |

UI-02 to UI-07 are independent of each other after UI-01 merges and can run in parallel worktrees.

Pages without a design frame (`/register`, `/admin/stats`, the 404 page) take the shell from UI-01 and the form and card styles from the nearest slice. They get no separate slice.

## Cross-slice rules

**Design tokens.** The Pencil file uses the shadcn variables (`--background`, `--primary`, `--muted-foreground`, …) with theme `Base: Zinc`, `Accent: Green`. `frontend/src/app/index.css` already defines the same variable names, so the code keeps its tokens and changes values only where UI-01 says so. Do not hard-code design hex values in components.

**Primary colour contrast.** The design's green accent (`#65a30d`) with white text reaches about 3:1. `Frontend.md` §8.1 requires 4.5:1 for text. The code keeps its current `--primary` (`oklch(0.5203 0.1364 144.2)`), which passes. Pale tints that only carry state (pills, badges) may follow the design if the text on them passes 4.5:1.

**Typography.** The design uses Inter. No web font is loaded today (`--font-sans: Outfit, sans-serif` falls back to the system font). UI-01 decides the stack. No slice loads fonts from a third-party CDN.

**Icons.** The design uses Lucide icon names, which match the `lucide-react` package already installed. Use the same icon the frame names. Icons are decorative (`aria-hidden`) unless they are the only content of a control, in which case the control needs an accessible name.

**Copy.** The text in the design is English sample copy. Every visible string goes through i18n with both `en` and `zh-TW` entries. When the design wording differs from an existing key, change the value only if the meaning stays the same. Otherwise add a new key. Sample data in the frames (names, IDs, Idea names, dates) is not copy.

**Viewport.** Every frame is 1440 px wide. The code must also reflow at 320 px and 200 % zoom (`Frontend.md` §8). Each slice states how its layout collapses.

**Tests.** Existing tests keep passing. Where a test queries by text or role that the redesign changes, update the query, not the behaviour it checks. Each slice adds axe checks for its new states and keeps the route-table test in `app/App.test.tsx` green.

**Out of scope for every slice.** Backend or OpenAPI changes, new endpoints, new error codes, dark mode polish beyond keeping the existing `.dark` tokens working, and the sidebar "Help" links (see UI-01).

## Definition of Done (per slice PR)

- Visual comparison with the Pencil frame at 1440 px. Attach a screenshot of the page next to a `TakeScreenshot` of the frame to the PR.
- Layout checked at 320 px and at 200 % zoom.
- `npm run lint`, `npm run typecheck` and `npm test` pass in `frontend/`.
- Both locales checked for overflow (zh-TW strings are shorter, en strings are longer).
- Each **Design vs contract** item in the slice is either built as written or recorded as a follow-up bead.
- The slice bead is closed with the PR link.

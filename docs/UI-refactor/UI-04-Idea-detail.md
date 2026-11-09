# UI-04 — Idea Detail

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.4` |
| Pencil | Frame `rsQOw` "Idea Detail — Leader" |
| Code | `app/pages/ideas.tsx` (`IdeaDetail` page, dialogs), `features/idea/IdeaDetail.tsx`, `ReplaceSlides.tsx`, `DeleteIdea.tsx`, `features/team/MemberActions.tsx` |
| Spec | PRD §6.5–6.9, permission matrix; `Frontend.md` §5.4–5.5, §6.7; `shared/permissions/can.ts` |
| Depends on | UI-01; countdown text depends on bead `i5o.8` (F2-02) |
| Status | **[Proposed]** |

## Goal

Two-column detail page: Idea content and slides on the left, team and join requests on the right. Every action keeps its current `can()` rule, confirmation dialog, error mapping and audit-safe behaviour.

## Design summary

**Header:**

- Back link "Back to My Ideas" (`arrow-left`).
- `<h1>` Idea name.
- Badge row: "You are the Leader" (filled green), category badge "Cool Idea", org "BSID", direction "Building a Responsible Supply Chain".
- Actions on the right: outline "Edit details" (`pencil`), destructive outline "Dissolve team" (`trash-2`).

**Main column:**

1. **Idea summary** card. Summary text, then a 3 × 2 definition grid: Category ("Cool Idea (I)"), Org, ESG direction, Lead proposer ("王小明 · 089290"), Created, Last updated.
2. **Slides** card:
   - Hint "One PDF or PPTX, max 5 MiB. Re-uploading replaces the current file."
   - File row: `file-text` icon, download filename, "PDF · 3.2 MB · uploaded Nov 6, 2026 09:05", and the buttons "Replace" (`upload`) and "Download" (`download`, primary).
   - Info line (`info`): "Expected filename: {I|O}_{Org}_{Leader name}_{Employee ID}_{Idea name}".
3. **Deadline warning** (amber alert, `timer`): "Changes lock on Mon, Nov 30, 2026 · 18:00 (GMT+8)". Body: "After the deadline, editing, slide uploads, approvals and leaving the team are locked."

**Side column:**

1. **Team** card: "Team", "3 of 4 seats filled". Member rows show avatar, name ("(you)" for self), employee ID, a "Leader" badge or a remove button (`user-minus`). Footer (`plus`): "1 open seat — teammates apply with your employee ID 089290 and this Idea name".
2. **Join requests** card (Leader and Admin only): title with a count badge "2" and the hint "Visible only to you and admins". Request rows show avatar, name, "054616 · requested 2 hours ago", and the buttons "Reject" (`x`, outline) and "Approve" (`check`, primary). Footer: "Approving checks capacity — a full team (4/4) can't accept more."

## Implementation

| Area | Change |
|---|---|
| Layout | `grid-cols-[1fr_360px]` at ≥ 1024 px. The side column comes after the main column in DOM order. |
| Back link | Label follows the role: "Back to My Ideas" or "Back to All Ideas" (`pages.ideaDetail.back` gets a role variant). Return to the list URL with its filters if it is in history state. |
| Relation badge | From `IdeaDetail` + `me`: Leader, Member, or the `nav.administrator` badge for Admin. New key `pages.ideaDetail.youAreLeader` / `youAreMember`. |
| Header actions | "Edit details" → `/ideas/:id/edit` (Leader only, open state, per `can()`). Destructive action → existing `DeleteIdea` dialog. Member gets "Leave team" here (existing leave dialog). Admin keeps delete. |
| Dissolve vs delete | The design says "Dissolve team" for what the API calls delete Idea. Keep the existing delete semantics and dialog text, which says that the Idea and slides are deleted. The button label may say "Delete Idea" or "Dissolve team" in en; pick one wording and use it in both the button and the dialog. Owner decision, default: keep "Delete Idea". |
| Summary grid | `<dl>` with `<dt>`/`<dd>` pairs, 3 columns ≥ 1024 px, 2 columns ≥ 640 px, 1 column below. "Cool Idea (I)" = category label + filename prefix. |
| Slides card | Restyle the existing download and `ReplaceSlides` controls. Size via the existing formatter, "uploaded {date}" from `slides.updatedAt`. The expected-filename line keeps the copy button (`pages.ideaForm.copy`) that exists today and shows the real expected name (`expectedFilenameBase` + extension) rather than the pattern. The design shows the pattern; the concrete name is more useful and is already implemented. |
| Download | Keep the existing progress / incomplete-download states (`pages.ideaDetail.downloading`, `downloadIncomplete`). |
| Deadline warning | Shown while the state is OPEN. Text from `useDeadline()`. A CLOSED state shows a neutral "Changes are locked" alert. The live countdown variant waits for F2-02. |
| Team card | `<ul>` of members. Remove buttons (Leader, open state) need an accessible name naming the member (`pages.ideaDetail.removeNamed`, exists). "(you)" is a visually distinct suffix, also read by screen readers. Seat count = 1 + `members.length` of 4. |
| Open-seat hint | Shown to the Leader only, when seats < 4. Shows the Leader's employee ID and the Idea name, with a copy button for the Idea name (applicants need it exactly, PRD §6.7). |
| Join requests card | Rendered when `pendingRequests !== null` (Leader or Admin). Empty list → `pages.ideaDetail.noPending`. Approve and Reject keep `MemberActions` and the reject confirmation dialog. Buttons are named per person (`approveNamed`, `rejectNamed`, exist). Hide approve when the team is full, and keep the server's 409 `TEAM_FULL` handling as the source of truth. |
| Relative time | "requested 2 hours ago" via `Intl.RelativeTimeFormat`, with the absolute time in a `<time dateTime>` / `title`. Computed against the deadline response's `serverTime` offset (`shared/time/offset.ts`), not the raw client clock. |
| Admin view | Same layout. No hover prefetch. No extra `GET /ideas/{id}` from the shell (each one writes an `IDEA_VIEW` audit row). |

## Responsive behaviour

< 1024 px: one column in the order header → summary → slides → team → join requests → deadline alert. Header actions wrap below the badges.

## Design vs contract

| Item | Decision |
|---|---|
| "Dissolve team" wording | See above; semantics stay delete Idea. |
| Expected filename shown as a pattern | Show the concrete name (already available). |
| Countdown "Changes lock on …" | Static date now, live countdown after F2-02. |

## Tests

- `ideaDetail.test.tsx`, `ideaMembers.test.tsx` and `ideaSlides.test.tsx` keep passing. Update their queries for the new labels.
- New:
  - Leader, Member and Admin each see the correct header actions and cards (one row per permission-matrix cell touched).
  - The join requests card is absent when `pendingRequests` is null and shows the empty text when it is `[]`.
  - The seat count and open-seat hint.
  - The approve button is hidden at 4/4.
  - The deadline alert for OPEN and CLOSED.
  - axe for Leader, Member and Admin.

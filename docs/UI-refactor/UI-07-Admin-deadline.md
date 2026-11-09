# UI-07 — Admin Deadline

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.7` |
| Pencil | Frame `qSvEA` "Admin — Set Deadline" |
| Code | `app/pages/admin.tsx` (`AdminDeadline`), `features/deadline/DeadlineForm.tsx`, `features/deadline/set.ts`, `shared/time/local.ts`, `shared/time/offset.ts` |
| Spec | PRD (deadline and SubmissionGate), `Frontend.md` §5.3; `PUT /admin/deadline` in `docs/API/` |
| Depends on | UI-01 |
| Status | **[Proposed]** |

## Goal

Replace the small banner and single input with the design's deadline form card and a side card that explains the three submission states. Validation, the save request, error mapping and the "deadline can be moved but never cleared" rule stay.

## Design summary

**Header.** "Submission deadline" with the description "One global deadline for every team. It can be moved, but never cleared …".

**Deadline form card:**

- **Current deadline** block: "Mon, Nov 30, 2026 · 18:00 GMT+8", the UTC value "2026-11-30T10:00:00Z (UTC)" in muted mono, and a state badge "OPEN".
- **New deadline** fields, in one row: Date (`calendar`), Time (`clock-4`), Time zone (`globe`) "Asia/Taipei (GMT+8)".
- Preview row (`eye`): "Users will see: Submissions open · closes in 28d 06h".
- Warning alert (`triangle-alert`): "A time in the past closes submissions immediately — every user write is blocked …"
- Actions: "Reset" (outline; stray `plus` icon in the frame) and "Save deadline" (`save`, primary).

**States card.** "How submission state works", "Derived from database time — no grace period." Three entries, each a badge plus text:

| Badge | Text |
|---|---|
| NOT OPEN | No deadline set yet. All user writes are blocked. |
| OPEN | Now is before the deadline. Users can create, edit, upload and manage … |
| CLOSED | Deadline passed. View and download only. |

## Implementation

| Area | Change |
|---|---|
| Layout | Form card ~2/3 and states card ~1/3 at ≥ 1024 px, stacked below that. Remove the page-level `DeadlineBanner` from `admin.tsx`; the shell pill (UI-01) already shows the state everywhere. |
| Current deadline | From `useDeadline()`: local formatted date with `timeZoneName: 'shortOffset'`, the raw UTC ISO string in `<code>`, and a state badge (text + colour). "Not set" variant when `deadline` is null. |
| Date + Time inputs | Two native inputs (`type="date"`, `type="time"`) replace the single `datetime-local`. They are combined into one instant by the existing `shared/time/local.ts` helpers. Both labels are visible. The error summary and field errors (`pages.adminDeadline.problems`) are kept. |
| Time zone | See **Design vs contract**. Default: show the browser's zone as read-only text ("Asia/Taipei (GMT+8)"), not a select. |
| Preview | Client-side sentence computed from the entered instant and the server-time offset (`shared/time/offset.ts`): the state it would produce and the duration to it. Live region `aria-live="polite"`, updated on change (debounced), not every second. A past instant shows the CLOSED preview and the warning alert becomes prominent. |
| Warning alert | Always visible as guidance, as in the frame. Use the design text as new key `pages.adminDeadline.pastWarning`. |
| Reset | Restores the inputs to the current deadline. It is a `type="button"`, not a form reset that would also clear the error state wrongly. Drop the stray icon. |
| Save | Keeps `common.save` → design label "Save deadline" (new key `pages.adminDeadline.save`), `saving`, `saved` status. After success the current block and the shell pill refresh (invalidate `deadlineKey`, as today). |
| States card | Static explanatory `<dl>`. Badge colours reuse the UI-01 state tokens. New keys `pages.adminDeadline.states.*`. |

## Responsive behaviour

- < 1024 px: cards stack (form first).
- < 640 px: Date, Time and Time zone stack vertically.

## Design vs contract

| Item | Decision |
|---|---|
| Time zone selector | The API stores an instant (UTC); the zone only affects how the Admin enters it. A free choice of zone is new behaviour that the docs do not mention. Default: read-only browser zone. Making it selectable is an owner decision (add an OI). |
| "closes in 28d 06h" in the preview | Computed with the server-time offset, so no dependency on F2-02 for a static preview. A ticking preview is not needed. |
| "Never cleared" | Matches the current API: there is no clear action, and none is added. |

## Tests

- `adminDeadline.test.tsx` keeps passing. Update queries for the split Date / Time inputs.
- New:
  - Date + Time combine into the right UTC instant (table-driven, including a DST-free zone and UTC).
  - The preview shows OPEN for a future instant and CLOSED for a past one.
  - Reset restores the inputs.
  - The current block for a deadline that is not set.
  - The pill and current block refresh after save.
  - axe on default, invalid and saved states.

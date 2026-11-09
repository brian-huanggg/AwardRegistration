# UI-06 — Join Requests

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.6` |
| Pencil | Frame `eoK6b` "Join Requests" |
| Code | `app/pages/JoinRequests.tsx`, `features/team/JoinRequests.tsx`, `features/team/requests.ts` |
| Spec | PRD §6.7 (apply, withdraw, capacity), `docs/API/response-bodies.md` (JoinRequest) |
| Depends on | UI-01 |
| Status | **[Proposed]** |

## Goal

Two cards side by side (apply form, my pending requests) and a three-step explainer below them. The apply and withdraw behaviour, error mapping and CLOSED-state handling stay.

## Design summary

**Header.** "Join requests" with the description "Ask a Leader to add you to their team. You'll see the Idea once they approve …".

**Apply card:**

- Title "Apply to join a team", hint "Ask your Leader for their employee ID and the exact Idea name."
- Fields: Leader employee ID, Idea name.
- Inline error example (`circle-alert`): "No Idea matches this Leader ID and name. Check the exact spelling."
- Full-width primary button "Send request" (`send`).

**My pending requests card:**

- Title "My pending requests", hint "Withdraw any time before the deadline. Approved requests move to My Ideas." Count badge "3 pending".
- Rows: `hourglass` icon, Idea name, "Leader 莊育成 · 054616 · sent Today, 10:42", outline button "Withdraw" (`undo-2`). One row also shows a "Team full 4/4" badge.
- Footer (`lock`): "Pending applicants can't open the Idea or its slides until approved."

**How it works** (three steps, numbered circles):

1. Apply — "Enter the Leader's employee ID and the exact Idea name."
2. Leader reviews — "The Leader approves or rejects. Teams are capped at 4 people."
3. You're in — "The Idea appears under My Ideas. You can view, download slides, or leave …"

## Implementation

| Area | Change |
|---|---|
| Layout | Two columns ≥ 1024 px (apply ~5/12, requests ~7/12), explainer as a three-column row below. |
| Apply form | Restyle the existing form. `404 NOT_FOUND` maps to the design's message under the Idea name field (new key `pages.joinRequests.notFound`, both locales). The message must not reveal whether the Leader ID or the name was wrong, because the 404 is uniform (`AGENTS.md`). Other codes (`ALREADY_MEMBER`, `JOIN_REQUEST_EXISTS`, `CANNOT_JOIN_OWN_IDEA`, `SUBMISSION_*`) keep their current `errors.*` mapping. |
| Leader employee ID | `inputMode="numeric"`, `autoComplete="off"`, 6-digit pattern hint. The validation stays as it is. |
| Success | After 201, the new request appears in the list, and the existing `pages.joinRequests.sent` status message is announced. |
| Requests list | `<ul>` with one `<li>` per request. The count badge is `items.length` with a visible "pending" label. Each Withdraw button is named per Idea (`pages.joinRequests.withdrawNamed`, exists). Keep the current confirmation and error behaviour. |
| Sent time | "Today, 10:42" / "Nov 7, 2026" from `requestedAt`: relative day for today and yesterday, medium date otherwise, `<time dateTime>`. |
| Lock footer | Static text (new key `pages.joinRequests.lockedHint`). |
| Explainer | `<ol>` with three items. Numbers come from the list, and the circles are decorative. New keys under `pages.joinRequests.steps.*`. |
| CLOSED state | The form and the Withdraw buttons are disabled with `pages.joinRequests.closedHint`, as today. |
| Empty list | `pages.joinRequests.empty` inside the card, with the count badge hidden. |

## Responsive behaviour

< 1024 px: cards stack (apply first), and the explainer becomes a vertical list.

## Design vs contract

| Item in the design | Contract | Decision |
|---|---|---|
| "Team full 4/4" badge on a pending request | `JoinRequest` has no member count, and applicants may not see the Idea (PRD §6.7) | Not built. Exposing team size to applicants is a product and API decision; file a follow-up bead if wanted. |
| Leader name in the row | `JoinRequest.leader.name` exists | Built. |

## Tests

- `joinRequests.test.tsx` keeps passing.
- New:
  - The 404 message under the Idea name field and its `aria-describedby` link.
  - The count badge matches the list length.
  - The explainer renders as an ordered list.
  - The CLOSED state disables apply and withdraw.
  - axe on empty, filled and error states.

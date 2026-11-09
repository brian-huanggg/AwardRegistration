# UI-05 — New and Edit Idea

| | |
|---|---|
| Bead | `ESG-Award-Registration-System-u35.5` |
| Pencil | Frame `UfPqk` "New Idea" (1440 × 1120) |
| Code | `app/pages/ideas.tsx` (`IdeaCreate`, `IdeaEdit`), `features/idea/IdeaForm.tsx`, `features/idea/create.ts`, `shared/slides`, `shared/text` |
| Spec | PRD §5.1 (text rules), §6.5, §6.8 (slides and filename); `Frontend.md` §5.1, §5.2 (uncertain result), §5.5 (upload) |
| Depends on | UI-01 |
| Status | **[Proposed]** |

## Goal

Restyle the create form into three labelled sections with a short description column on the left and fields on the right, and use the same layout for edit. Validation, multipart upload with progress, the "result uncertain" path and the CLOSED-state disabling stay as they are.

## Design summary

**Heading.** "Submit a new Idea" with the description "You'll be the Leader. Teammates join later by request — they aren't added here …".

Each section is a card with a description column on the left and fields on the right.

| Section | Left column | Fields |
|---|---|---|
| Classification | "Category and Org become part of the slide filename." | **Category** as two radio cards: `sparkles` Cool Idea ("A fresh, creative concept. Filename prefix I.") and `trophy` Outstanding Impact ("Proven, measurable results. Filename prefix O."). **Org** select, **ESG direction** select, side by side. |
| About the Idea | "Name 1–75 characters, summary 1–300. No / \ : * ? " < > \|" | **Idea name** with the counter "9 / 75". **Idea summary** textarea with the counter "176 / 300". |
| Slides | "Required. PDF or PPTX, up to 5 MiB. The filename must match exactly." | **Expected filename** box with a Copy button. Selected file row: `file-text` icon, filename, "3.2 MB of 5 MiB", progress bar, `circle-check` "Filename matches". |

**Actions.** Footer on the right: "Cancel" (outline; the frame shows a stray `plus` icon) and "Submit Idea" (`send`, primary).

## Implementation

| Area | Change |
|---|---|
| Section layout | New local `FormSection` component (title, description, children). Each section is a `<fieldset>` with `<legend>` = section title, or a `<section aria-labelledby>`. Prefer `fieldset` for Classification, because it groups the category radios. |
| Category radio cards | Native `<input type="radio">` inside a `<label>` card, so keyboard and screen reader behaviour is native. Checked state is shown with border + check icon, not colour alone. The description sentence is linked with `aria-describedby`. |
| Org / ESG direction | Keep `NativeSelect` (accessible, works at 320 px). Restyle to the design's select look with a `chevron-down` icon. A Radix `Select` is not needed. |
| Counters | Existing `pages.ideaForm.count`. Counting stays code-point based after NFC (`shared/text`). The counter is announced politely only near the limit, not on every keystroke. |
| Forbidden characters hint | The left-column text reuses `pages.ideaForm.nameHint`. The list of characters must match `shared/text` exactly. |
| Expected filename box | Reuse the existing expected-filename UI (`expectedTitle`, `expectedIs`, `expectedPending`) and `CopyButton`. It shows the pending text until Category, Org and name are valid. |
| File row | Restyle the existing file input and progress. The hidden native `<input type="file">` stays the control, with a visible styled `<label>` button (`pages.ideaForm.choose`). States: none chosen, chosen + filename matches (success), chosen + mismatch (error with the expected name, OI-21: no automatic rename), too large, wrong type, uploading with `<progress>` / `role="progressbar"` and a percentage. |
| Errors | Keep the error summary at the top on submit (`pages.register.summaryTitle` pattern) with links to fields. Field errors render under each field and are linked with `aria-describedby`. |
| Actions | Cancel → back to `/ideas` (create) or `/ideas/:id` (edit). Drop the stray `plus` icon. "Submit Idea" keeps `pages.ideaCreate.submit` / `submitting` / `uploading`. Disabled when CLOSED, with the existing reason text. |
| Uncertain result | `pages.ideaCreate.uncertain` / `duplicateAfterUncertain` render as an alert above the actions, unchanged in behaviour. |
| Edit page | Same sections. The Slides section is replaced by `pages.ideaEdit.slidesNote` (slides are replaced on the detail page). The heading is `pages.ideaEdit.title`, and the submit button is `common.save` / `pages.ideaEdit.saving`. |
| Leader hint | The design's description "You'll be the Leader …" maps to `pages.ideaForm.leaderHint`. Update the en value only if the meaning is kept, and keep zh-TW in step. |

## Responsive behaviour

- ≥ 1024 px: section description column ~280 px, fields fill the rest; Org and ESG direction side by side.
- < 1024 px: description above the fields; category cards stay side by side down to 640 px.
- < 640 px: everything stacks; the actions become full width with the submit button first in visual order and last in DOM order only if that matches focus order. Default: keep DOM order Cancel, Submit.

## Design vs contract

| Item | Decision |
|---|---|
| "5 MiB" limit | Matches PRD §6.8. Use the same constant as the size check. |
| "Filename matches" success state | Already computed client-side. The server still decides (422). |
| Stray `plus` icon on Cancel | Not built. |

## Tests

- `ideaCreate.test.tsx` and `ideaEdit.test.tsx` keep passing. Update queries from select to radio for the category.
- New:
  - Arrow keys move between the category radio cards.
  - The expected filename updates when category, org or name changes.
  - The success and error file states.
  - The submit button is disabled when CLOSED (existing test in `App.test.tsx`).
  - axe in empty, invalid, uploading and uncertain states.

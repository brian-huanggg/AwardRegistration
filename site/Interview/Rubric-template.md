# Question and Rubric Template

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-10 |
| Status | **[Proposed]** |

Every question has two parts:

1. The **question entry**, in a public file in this folder (`modules/` or `scenarios/`).
2. The **rubric entry**, in the private answer key, under the same question ID.

**Never commit a rubric entry to this repository.** The repository is public, and this folder is published on the documentation site.

## 1. Question entry (public)

Copy this block into the module or scenario file. Questions in a file are ordered by level, then by number.

```markdown
### <PREFIX>-L<level>-<NN> — <short title>

| | |
|---|---|
| Level | L1 / L2 / L3 / L4 |
| Roles | Junior / Senior / Architect |
| Time | <minutes> |
| Status | Draft / Active / Retired |
| Sources | <D-numbers, document and section, code path> |

**Prompt**

<A concrete situation in this system. State what the candidate is given and what they must answer or build.>

**Follow-ups**

1. <Goes one step deeper than the prompt.>
2. <Changes one condition: concurrency, failure, scale or an attacker.>
3. <Asks for the trade-off or the alternative.>
```

### Field rules

| Field | Rule |
|---|---|
| Title | Names the situation, not the answer. "Upload commit fails", not "Keep the object on unknown outcome". |
| Level | One level. See the level table in `site/Interview/README.md`. |
| Roles | The roles this question is useful for. Most L4 questions do not fit Junior. |
| Time | Prompt plus follow-ups. L1: 2–3 min, L2: 10–15 min, L3: 8–10 min, L4: 10–15 min, `SCN`: 20–30 min. |
| Status | `Draft` until a second person has reviewed the question and its rubric. Only `Active` questions are used in interviews. |
| Sources | At least one. Prefer a decision number (`D44`) plus the document section. Add a code path when the question is about code. |
| Prompt | Does not contain the answer, the decision number or the name of the pattern being tested. |
| Follow-ups | L1: optional. L2: at least one. L3, L4, `SCN`: at least three. |

### Example (public part only)

```markdown
### STOR-L4-01 — Upload commit fails

| | |
|---|---|
| Level | L4 |
| Roles | Senior / Architect |
| Time | 12 |
| Status | Draft |
| Sources | D36, D44, `docs/Architecture/Sequence.md` §2, §4, `docs/Architecture/Backend.md` §4.4 |

**Prompt**

A team leader replaces the slide file of their Idea. The backend has already stored the new file in object storage. It then runs `COMMIT` for the database transaction that points the Idea to the new file, and `COMMIT` returns an error. What does the backend do with the new object and the old object, and what does the user see?

**Follow-ups**

1. How do you tell the different kinds of `COMMIT` error apart?
2. The database connection drops during `COMMIT`, so you cannot know whether it succeeded. What is the safe choice, and why?
3. Two replicas of the backend run at the same time. Who cleans up objects that are no longer needed, and how do you stop two replicas from doing the same work?
```

## 2. Rubric entry (private answer key only)

The rubric entry uses the same ID and title as the question entry.

```markdown
### <ID> — <title>

**Expected points**

- <Fact or reasoning step the answer must contain.>
- <…>

**Scoring**

| Score | Label | The answer … |
|---|---|---|
| 0 | Not met | <misses the core point, or is wrong in a way that would cause a defect> |
| 1 | Pass | <gets the core point; needs help on the follow-ups> |
| 2 | Good | <gets the core point and most follow-ups without help> |
| 3 | Excellent | <also names the trade-off or a better alternative, with reasons> |

**Red flags**

- <An answer that would cause data loss, a security hole or an information leak in this system.>

**Follow-up notes**

1. <What a good answer to follow-up 1 contains.>
2. <…>
```

### Scoring rules

- Score every question from 0 to 3. Do not use half points.
- A **red flag** caps the question at 1, even if the rest of the answer is strong. Record which red flag it was.
- A different design than ours can score 3 if the candidate argues it well and names what it costs.
- Record one sentence of evidence for each score: what the candidate said or did, not your impression.

### Overall result

Average the scores of all questions asked, then apply the role bar.

| Role | Hire bar | Also required |
|---|---|---|
| Junior | average ≥ 1.5 | no 0 on an L1 question |
| Senior | average ≥ 2.0 | at least one 2 on an L4 question |
| Architect | average ≥ 2.3 | at least one 3 on an L4 or `SCN` question |

**[Proposed]**: the hire bars are a starting point. Adjust them after the first interviews.

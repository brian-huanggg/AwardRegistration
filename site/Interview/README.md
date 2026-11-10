# Interview Question Bank

| | |
|---|---|
| Version | 0.1 |
| Date | 2026-11-10 |
| Bead | `ESG-Award-Registration-System-s7y` |
| Status | **[Proposed]**: structure and rules wait for owner review |

This folder holds interview questions for full-stack engineers and architects. Every question comes from a real decision, rule or failure path in this project, so a question can always be checked against the design documents.

The bank tests reasoning, not recall. Most questions ask *why* the design is the way it is, what breaks if it changes, and what happens when a component fails.

## Public repository: no answers here

This repository is public. **Rubrics, model answers and red-flag lists are never committed.** They live in a private answer key, keyed by question ID (see [Rubric-template.md](Rubric-template.md)). **[Proposed]**: the location of the private answer key is not decided yet.

The question files in this folder hold only the prompt, the follow-ups and the sources. The documentation site publishes them, and the design documents are public anyway, so a strong candidate may have read them. That is acceptable: the follow-ups test whether the candidate can reason past the written answer.

This README is a maintainer index. It stays in the repository but is not rendered: `site/.vitepress/config.mts` lists it in `srcExclude`. Because it is not rendered, published pages must not link to it. When a new question file is added, also add it to the `Interview` sidebar group in that config.

## Files

| File | Content |
|---|---|
| `README.md` | This index: levels, IDs, usage, maintenance |
| [Rubric-template.md](Rubric-template.md) | Question format (public) and rubric format (private answer key) |
| `Coverage.md` | Module × level counts; decision-log entries without a question *(not written yet)* |
| `modules/<module>.md` | Questions about one module *(`auth-session.md`, `account.md`, `idea.md`, `storage.md`, `deadline.md` and `audit.md` written so far)* |
| `scenarios/<scenario>.md` | Questions that cross several modules *(not written yet)* |

### Module files

One file per backend module or concern. The file name and the ID prefix are fixed.

| File | Prefix | Scope | Main sources |
|---|---|---|---|
| `modules/api-contract.md` | `API` | OpenAPI spec-first, error codes, JSON binding rules | `docs/API/`, D46, D54, D55, D58 |
| `modules/server-pipeline.md` | `PIPE` | Middleware order, size limits, check order before the body is read | Backend.md §3, D39, D59, D60 |
| `modules/auth-session.md` | `AUTH` | Session, CSRF, login throttling, logout | Backend.md §6, D24, D40–D43 |
| `modules/account.md` | `ACCT` | Registration, Vault sync, `SessionRevoker` | D30, D51 |
| `modules/team.md` | `TEAM` | Join requests, approval, team capacity | Backend.md §5, D18–D21 |
| `modules/idea.md` | `IDEA` | Idea lifecycle, visibility, uniform 404 | Backend.md §8, D8, D9, D22, D56 |
| `modules/storage.md` | `STOR` | Upload, download, MinIO, storage consistency | Sequence.md, D5, D36–D38, D47, D50 |
| `modules/deadline.md` | `DEAD` | Submission gate, database time | D7, D34 |
| `modules/audit.md` | `AUDT` | Track A / track B, hash chain, retention | Backend.md §7, D2, D3, D13, D23, D48, D53 |
| `modules/architecture.md` | `ARCH` | Modular monolith, module boundaries, `shared`, Go choice | ADR 0001, D1, D32, D57, D61 |
| `modules/data-model.md` | `DATA` | Schema, constraints, collation, IDs, migrations | Backend.md §10, D14–D17, D45, D62 |
| `modules/frontend.md` | `FE` | React app, i18n, error mapping, CSRF on the client | Frontend.md, D11, D52 |
| `modules/testing.md` | `TEST` | Unit, integration, fuzz, abuse tests, permission matrix | Backend.md §11 |
| `modules/operations.md` | `OPS` | Multi-replica behaviour, jobs, deployment | Architecture.md §12, Backend.md §9, D10, D25, D26 |

### Scenario files

| File | Prefix | Scenario |
|---|---|---|
| `scenarios/request-lifecycle-upload.md` | `SCN` | Walk one slide upload through every layer and every failure |
| `scenarios/failure-injection.md` | `SCN` | MinIO timeout, deadlock, unknown commit outcome |
| `scenarios/threat-modeling.md` | `SCN` | Enumeration, CSRF, brute force, oversized files |
| `scenarios/scale-evolution.md` | `SCN` | 100× more users: which decisions change, in which order |

## Levels

| Level | Name | The candidate shows that they can … | Typical prompt |
|---|---|---|---|
| L1 | Know | explain a concept correctly | "What does `SELECT … FOR UPDATE` lock?" |
| L2 | Do | build or change code for a given case | "Write the handler that rejects a duplicate join request." |
| L3 | Trade off | explain why a design was chosen and compare it with alternatives | "Why does a hidden Idea answer 404 and not 403?" |
| L4 | Fail | reason about concurrency, partial failure and attacks | "The `COMMIT` fails after the upload. What do you do with the object?" |

## Question IDs

Format: `<PREFIX>-L<level>-<NN>`, for example `STOR-L4-03`. `SCN` questions have no level in the ID (`SCN-07`) because they run through several levels.

- An ID is never changed and never reused.
- A question that no longer fits the design is marked **Retired**, not deleted. Interview records and the private answer key keep their references.
- If a question moves to a different level, it is retired and a new question with a new ID replaces it.

## Using the bank in an interview

### Session mix by role

| Role | L1–L2 | L3 | L4 and `SCN` | Example 60-minute session |
|---|---|---|---|---|
| Junior | about 70 % | about 25 % | about 5 % | 2 × L1, 2 × L2, 1 × L3 |
| Senior | about 20 % | about 50 % | about 30 % | 1 × L2, 2 × L3, 1 × L4 |
| Architect | about 10 % | about 40 % | about 50 % | 1 × L3, 1 × L4, 1 × `SCN` |

Pick questions from at least three different modules. One question with deep follow-ups tells you more than three questions answered on the surface.

### During the interview

1. Read the prompt. Do not mention the decision number or the document.
2. Let the candidate answer completely before the first follow-up.
3. Ask the follow-ups in order and stop when the candidate is clearly above or below the level.
4. Do not read the rubric aloud and do not lead the candidate to the expected answer.
5. Record the question ID, the score and one sentence of evidence for each question.

A candidate may propose a different design than this project uses. Score the reasoning, not the match with our choice. A well-argued alternative can score Excellent.

## Writing and maintaining questions

- Use the format in [Rubric-template.md](Rubric-template.md). Every question has at least one entry in **Sources**.
- Write the prompt as a situation, not as a definition ("A user uploads a 6 MiB file …", not "What is a 413?").
- Each L3 and L4 question has at least three follow-ups that go deeper, not sideways.
- Write in English, like every repository document.
- Do not put secrets, Vault paths or internal hostnames in a prompt.

When a design decision changes, search this folder for its decision number (for example `D44`). Update or retire every question that cites it, and update the private answer key in the same change.

If a question and a design document disagree, the design document wins. Fix the question.

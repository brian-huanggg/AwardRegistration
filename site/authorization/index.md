# Authorization & Uniform 404: Concepts

[Authentication](../authentication/) answers **who is making this request?** Authorization answers **what may they do?** This page explains the second question and one rule that makes this system different from a plain "403 if not allowed" design: **a resource you may not see looks exactly like a resource that does not exist.**

## Authorization in one table

| Question | Answer | Status |
|---|---|---|
| Is there a valid session? | No | `401` |
| Is the CSRF token valid (writes only)? | No | `403` |
| Does the resource exist **and may the caller see it**? | No | `404` |
| May the caller do this to a resource they can see? | No | `403` |
| Is the contest in the right state (before the deadline)? | No | `403` |
| Is the input valid? | No | `400` / `422` |

The order is fixed: **401 → CSRF 403 → 404 → role 403 → state 403 → validation**. Each check runs only if the earlier ones passed. A test pins the order.

## 403 versus 404

| | `403 Forbidden` | `404 Not Found` |
|---|---|---|
| Says | "It exists. You may not." | "There is nothing here for you." |
| Leaks existence? | **Yes** | No |
| Used here when | The caller can already see the resource but lacks the right (a Member tries to edit) | The caller cannot see the resource, **or** it does not exist |

If a stranger asks for an Idea and gets `403`, they now know that Idea exists. With a random UUID this looks harmless, but the same pattern leaks which employee IDs, team names or file names are real. This system returns `404` in both cases, so the response carries no information.

## The visibility rule

An Idea is visible to exactly three kinds of caller:

| Caller | Sees the Idea? |
|---|---|
| Leader of the Idea | Yes |
| Member whose request is **approved** | Yes |
| Admin | Yes |
| Anyone else, including a user with a **pending** request | No → `404` |

Everything else follows from this. "Not yours" and "does not exist" are the same case.

## Uniform means three things

A `404` is only uniform if an attacker cannot tell the two cases apart by any channel:

| Channel | How it is kept identical |
|---|---|
| Body | Same problem+json: `code: NOT_FOUND`, same `detail` (only `requestId` differs) |
| Headers | Same set of headers |
| Time | Same database work, including the audit write, in both cases |
| Audit log | Both are written as `DENIED` with the same reason `NOT_FOUND`, so the log cannot be used to tell them apart either |

Timing is the subtle one. If "missing" skipped the audit insert and "hidden" did not, one answer would be measurably slower. That is why both are audited.

## The permission matrix

The PRD (§7) lists every operation against five roles. Excerpt:

| Operation | Visitor | User (no relation) | Member | Leader | Admin |
|---|---|---|---|---|---|
| view Idea | 401 | 404 | 200 | 200 | 200 |
| edit text fields | 401 | 404 | 403 | 200* | 403 |
| delete Idea | 401 | 404 | 403 | 204* | 204 |
| approve / reject member | 401 | 404 | 403 | 2xx* | 403 |
| query Audit Log | 401 | 403 | 403 | 403 | 200 |

`*` = only while submissions are open. Read a row from left to right to see the rule: a User with no relation gets `404`, a Member who can see but not edit gets `403`.

The matrix is stored as `testdata/permission-matrix.json`. **Every cell is an automated test case**, so changing a rule means changing the data file first.

## Enterprise practice compared with this project

| Topic | Common practice | This project |
|---|---|---|
| Hidden resource | Often `403` | `404`, identical to missing |
| Role model | RBAC with a policy engine (OPA, Casbin) | Fixed five-role matrix in code and a data file |
| Where checks live | Gateway or annotations | In the service, before the body is read |
| Audit of denials | Often only successes | Denials are first-class (`DENIED` rows) |
| Test of the policy | Spot checks | The whole matrix, cell by cell |

See [Architecture](./architecture) for the pipeline, [Implementation](./implementation) for the code, [Decision Thinking](./decisions) for the reasons and [Technical FAQ](./faq) for quick answers.

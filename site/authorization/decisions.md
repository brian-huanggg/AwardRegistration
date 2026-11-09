# Authorization & Uniform 404: Decision Thinking

This page records **why** authorization looks the way it does. Entries marked *(documented)* follow reasons written in the design documents (decision-log numbers in brackets). Entries marked *(inferred)* are the likely reason read from the design; confirm them with the owner before quoting them as fact.

## A framework for authorization decisions

1. **What does a refusal reveal?** Status, body, headers, time and logs are all channels.
2. **Who can already see the resource?** Only those callers may be told "forbidden".
3. **Where is the truth?** One predicate, in one place.
4. **Is the check still true at write time?** Re-check inside the transaction.
5. **Can a test pin every cell?**

## Decisions

### Z1. Hidden resources answer 404, not 403 *(documented, D9, PRD §7 rule 2)*

| | |
|---|---|
| Question | What does a caller get for an Idea they may not see? |
| Options | `403`; `404` |
| Choice | `404`, identical to "does not exist" |
| Why | `403` confirms the Idea exists. The system must not allow enumeration of teams or Ideas. |
| Cost | A user with a pending request cannot be told "you are not approved yet" on the Idea page. They see pending requests in their own list instead. |

### Z2. One visibility predicate *(documented, D9)*

| | |
|---|---|
| Question | How do we make sure no endpoint forgets the rule? |
| Options | Check in each handler; middleware; one SQL fragment used by every read |
| Choice | One SQL fragment, `VisibleWhere`, and `GetVisible` / `ListVisible` as the only read paths |
| Why | The database returns no row for both cases, so code cannot leak the difference. The list and the detail share the rule. |
| Cost | Reads that need extra joins must still go through the fragment. Code review has to watch for raw `SELECT … FROM idea`. |

### Z3. Audit both cases the same way *(documented, D48, OI-16 proposed)*

| | |
|---|---|
| Question | Should a "missing" 404 be audited? |
| Options | Audit only "not yours"; audit neither; audit both |
| Choice | Audit both as `DENIED`, reason `NOT_FOUND` |
| Why | Auditing one case adds a write to one path only. That makes it slower and breaks the uniform 404 by timing. |
| Cost | A probe of random UUIDs fills the audit log. Retention must cope. |

Auditing neither would hide probing from administrators, which is worse.

### Z4. A malformed ID is also a 404 *(documented, D56, owner 2026-11-08)*

| | |
|---|---|
| Question | What does `/ideas/not-a-uuid` return? |
| Options | `400`; `404` |
| Choice | `404`, audited as DENIED with `targetId` null and the route template recorded |
| Why | One rule for every Idea path. No special case to test or explain. |
| Cost | A client bug (bad ID) looks like a missing resource. The audit row keeps the route for debugging. |

### Z5. 404 comes before role 403 *(documented, PRD §6)*

| | |
|---|---|
| Question | If the caller can neither see nor edit, which status? |
| Choice | `404` first, then `403` |
| Why | `403` only makes sense for a caller who is allowed to know the resource exists. |
| Cost | None beyond the fixed order, which a test pins. |

### Z6. Check order before reading the body *(documented, D58, D39)*

| | |
|---|---|
| Question | When is the request body read? |
| Choice | After 401, CSRF, 404, role and state. The generated server is non-strict, so handlers decode the body themselves. |
| Why | A strict server decodes JSON first, so a bad body would answer `400` before `404`. It also stops anonymous callers from forcing a 5 MiB upload. |
| Cost | Each handler decodes its own body (with one shared strict decoder). |

An oversized body may still answer `413` first; bodies are not drained to produce an earlier `401` or `404` (D39).

### Z7. Re-check role and state inside the transaction *(documented)*

| | |
|---|---|
| Question | Is the pre-check enough? |
| Choice | No. Lock the row, then check again. |
| Why | The deadline can pass, or the Leader can change, between the pre-check and the write. The database clock is the only clock. |
| Cost | Two checks per write. The first is a cheap read and mostly saves work. |

### Z8. The matrix is data, and every cell is a test *(documented, PRD §7 rule 4)*

| | |
|---|---|
| Question | How do we avoid drift between spec and code? |
| Choice | `testdata/permission-matrix.json` drives a test for every cell |
| Why | A hand-written test list forgets cells. Changing a rule starts with the data. |
| Cost | The file must be kept in step with the PRD; a PR that touches a cell must touch the file. |

### Z9. Fixed roles, no policy engine *(inferred)*

| | |
|---|---|
| Question | Use OPA, Casbin or similar? |
| Choice | Five fixed roles in code |
| Why | Only one resource type has per-object rules, and the rules are in the PRD. An engine adds a dependency and a second language for a few conditions. |
| Cost | A new role needs a code change. Revisit if the role set grows. |

## Known limits

- Timing equality is designed in, but the statistical timing test is planned work (Q7-02).
- The `DENIED` volume from probing is not rate-limited by this feature.

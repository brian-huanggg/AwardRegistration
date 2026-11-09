# Team & Join Request: Concepts

A team is one **Leader** plus up to **three Members**. Members are not chosen on the Idea form. Each person **asks to join** an existing Idea, and the Leader decides. This page explains the model, the five actions on a request, and the limits.

## Roles on one Idea

| Role | How you get it | Sees the Idea? |
|---|---|---|
| Leader | You created the Idea | Yes |
| Applicant | You sent a join request; the Leader has not answered | **No** (404) |
| Member | The Leader approved your request | Yes |

A person can be Leader of several Ideas, Member of several, and Applicant on several. An Admin is none of these and cannot apply.

## The request lifecycle

One table row, `idea_member`, is both the request and the membership. Its `status` is `PENDING` or `ACCEPTED`.

| Action | Who | From | To |
|---|---|---|---|
| Apply | User | no row | `PENDING` |
| Approve | Leader | `PENDING` | `ACCEPTED` |
| Reject | Leader | `PENDING` | row deleted |
| Withdraw | Applicant | `PENDING` | row deleted |
| Remove | Leader | `ACCEPTED` | row deleted |
| Leave | Member | `ACCEPTED` | row deleted |

Every exit **deletes the row**. History lives only in the Audit Log. After a reject or withdraw, the person can apply again at once.

## How to apply

The applicant types the Leader's **employee ID** and the **Idea name**. There is no search and no list of Ideas to browse. The pair (Leader, name) identifies one Idea because a Leader cannot own two Ideas with the same name.

## Rules at a glance

| Rule | Result when broken |
|---|---|
| Admin cannot apply | `403 FORBIDDEN` |
| Cannot join your own Idea | `422 CANNOT_JOIN_OWN_IDEA` |
| Already a Member | `409 ALREADY_MEMBER` |
| Already have a pending request | `409 JOIN_REQUEST_EXISTS` |
| No Idea matches | `404 NOT_FOUND` (same as any missing resource) |
| Approving a fourth Member | `409 TEAM_FULL` |
| Approving a row that is not pending | `409 JOIN_REQUEST_NOT_PENDING` |
| A Leader tries to leave | Not possible; delete the Idea instead |
| Outside the `OPEN` window | `403 SUBMISSION_NOT_OPEN` / `SUBMISSION_CLOSED` |

## Capacity: pending does not count

Only **approved** Members count toward the limit of three. Pending requests take no seat and are not capped. A person can apply to a full team; the Leader just cannot approve until a seat frees up. Capacity is checked at **approval**, not at application.

## What an applicant sees

`GET /join-requests` lists the caller's own pending requests with minimal Idea information: Idea ID, name, category, org, direction, Leader and request time. The Idea ID is what the applicant needs to withdraw. The applicant cannot open the Idea or its slides: `GET /ideas/{id}` returns 404 (see [Authorization & Uniform 404](../authorization/)).

## Known trade-off

Anyone logged in who knows a Leader's employee ID and an Idea name can learn that the Idea exists, because the apply call answers differently for a match and for no match. The design accepts this. Pending requests are also not capped, so a Leader can be flooded. Both are recorded as accepted risks (D21, PRD §10).

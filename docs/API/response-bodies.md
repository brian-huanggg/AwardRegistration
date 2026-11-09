# Response Bodies
 
| | |
|---|---|
| Version | 1.0 |
| Date | 2026-11-08 |
| Companion of | `docs/PRD.md` v1.5, `docs/API/api-endpoints.md`, `docs/API/error-codes.md` |
| Moved from | PRD v1.4 §6.14 |
| Status | **[Proposed]**, owner confirmation: OI-26 (see `docs/Open-items.md`) |
 
Principle: a write returns the current state of the resource it changed. The OpenAPI snapshot is authoritative for types (`docs/API/README.md`).
 
## 1. Bodies by endpoint
 
| Endpoint | Body |
|---|---|
| `POST /auth/register` 201 | `{empNo, name, email}`; no `Location` |
| `POST /auth/login` 200, `GET /auth/me` 200 | `{empNo, name, email, role}` (`email` null for Admin) |
| `GET /auth/csrf` 200 | `{token}` |
| `POST /ideas` 201 | IdeaDetail, with `Location: /api/v1/ideas/{id}` |
| `GET /ideas/{id}`, `PUT /ideas/{id}`, `PUT /ideas/{id}/slides`, approve | IdeaDetail |
| `GET /ideas` | `{items: IdeaSummary[], page, size, total}` |
| `POST /join-requests` 201 | JoinRequest |
| `GET /join-requests` | `{items: JoinRequest[]}` |
| `GET /deadline`, `PUT /admin/deadline` | `{deadline, state, serverTime}` |
| `GET /admin/audit-logs` | `{items, nextCursor}` |
 
Timestamps are ISO 8601 UTC with `Z` and millisecond precision; employee IDs are strings; resource IDs are UUIDs (PRD §6.1).
 
## 2. Shapes
 
```jsonc
// IdeaDetail
{ "id", "category", "org", "direction", "name", "summary",
  "leader":  { "empNo", "name" },
  "members": [ { "empNo", "name", "joinedAt" } ],          // approved members; joinedAt = approval time
  "pendingRequests": [ { "empNo", "name", "requestedAt" } ] | null,  // null unless Leader or Admin
  "slides": { "ext", "size", "sha256", "updatedAt", "downloadFilename" },
  "expectedFilenameBase": "I_BSID_王小明_089290_成立供應商永續學院",  // without extension
  "createdAt", "updatedAt", "version" }
 
// IdeaSummary (no summary text)
{ "id", "name", "category", "org", "direction", "leader": { "empNo", "name" },
  "memberCount", "myRelation": "LEADER" | "MEMBER" | null,  // null for Admin
  "createdAt", "slidesUpdatedAt" }
 
// JoinRequest
{ "ideaId", "ideaName", "category", "org", "direction",
  "leader": { "empNo", "name" }, "requestedAt" }

// AuditLog (GET /admin/audit-logs; every key is always present, null when the column is null)
{ "id", "occurredAt", "requestId", "actorEmpNo", "actorRole", "action", "outcome",
  "targetType", "targetId", "method", "route", "status", "errorCode",
  "ip", "userAgent", "submittedEmpNo", "before", "after" }
```
 
## 3. Rules
 
- `pendingRequests` is `null` (not `[]`) for callers who may not see it, so "no requests" and "not visible" differ.
- Email addresses of other people are never returned.
- `expectedFilenameBase` is the expected slide filename without extension (PRD §6.8); the client appends `.pdf` or `.pptx` (`Frontend.md` §5.5).
- `GET /join-requests` returns the Idea ID, which the applicant needs to withdraw, and only minimal Idea information; `GET /ideas/{id}` stays 404 for applicants (PRD §6.7).
- `GET /admin/audit-logs`: `occurredAt` has microsecond precision (`2026-11-09T01:02:03.456789Z`), the only exception to the millisecond rule; `nextCursor` is an opaque string, `null` on the last page; `before` / `after` are the stored JSON documents or `null`; `ip` is dotted for IPv4.
- `version` supports the optimistic-locking bonus (PRD §9 #3).
 
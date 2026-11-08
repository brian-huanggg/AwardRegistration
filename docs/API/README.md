# API Documentation
 
| | |
|---|---|
| Version | 1.0 |
| Date | 2026-11-08 |
| Companion of | `docs/PRD.md` v1.5 |
| Status markers | **[Decided]** agreed in discussion · **[Proposed]** drafted by the author, awaiting confirmation · **[Open]** unresolved (tracked in `docs/Open-items.md`) |
 
This folder holds the technical API contract. It was split out of PRD §6.12–§6.14 in PRD 1.5. The PRD keeps the behavioural requirements (conventions, check order, validation rules, permission matrix); this folder describes the shape of the interface.
 
| File | Content | Machine-readable counterpart |
|---|---|---|
| `api-endpoints.md` | The API set: every endpoint, who may call it, success status, main errors | OpenAPI snapshot `openapi.json` (generated from code) |
| `error-codes.md` | Error envelope (RFC 9457), rules for `errors[]`, the table of all codes by HTTP status | `testdata/error-codes.json` |
| `response-bodies.md` | Response bodies of every endpoint, with the IdeaDetail, IdeaSummary and JoinRequest shapes | OpenAPI schemas |
| `openapi.json` | Committed snapshot of `/v3/api-docs`; CI compares it with the generated document and lints it with Spectral (`Backend.md` §11) | – |
 
## Source of truth
 
1. **Behaviour** (what the server must do, in which order) is defined by the PRD. These files describe it and never override it.
2. **Schemas** (field names, types) are defined by the OpenAPI snapshot. The tables here are for reading; when they disagree with the snapshot, the snapshot wins and the table is fixed.
3. **Error codes** are defined by `testdata/error-codes.json` (`{code, status}`). Every code in it MUST appear in `error-codes.md`, and the reverse; a CI check compares the two lists. **[Proposed]**
4. The front end generates its types from the snapshot (`Frontend.md` §4) and looks up UI texts by `code`.
## Rules for changing the API
 
- A new endpoint, field or error code is added in the same pull request to: the OpenAPI snapshot, the matching file in this folder, `testdata/error-codes.json` (for codes), and the permission matrix (`testdata/permission-matrix.json`, for endpoints).
- Open questions about the contract are tracked in `docs/Open-items.md` (OI-15 error codes and list parameter, OI-26 response bodies).
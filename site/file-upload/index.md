# File Upload: Concepts

Each Idea has exactly one slide file. The file is the only user-supplied binary the system accepts, so it is the largest untrusted input. Upload checks decide **what is allowed in**, in what **order**, and what happens to the stored object when the database write that should reference it fails.

Two things are stored and they can disagree:

- the **object** in MinIO (the bytes);
- the **row** in MariaDB (`object_key`, `ext`, `size`, `sha256`, version).

Most of the design is about keeping those two consistent without a distributed transaction.

## What the rules accept

| Rule | Value | Failure |
|---|---|---|
| Exactly one `slides` file part | one | `400 MALFORMED_REQUEST` |
| Size | 1 byte to exactly 5 MiB (5,242,880) | `413 FILE_TOO_LARGE` / `422 EMPTY_FILE` |
| Extension | `.pdf` or `.pptx` (case-insensitive) | `415 UNSUPPORTED_FILE_TYPE` |
| Content matches extension | magic bytes, see below | `415 UNSUPPORTED_FILE_TYPE` |
| File name | equals the expected name, after NFC | `422 FILENAME_MISMATCH` |

The expected name is built from the Idea, not chosen by the user: `{I\|O}_{org}_{leaderName}_{leaderEmpNo}_{ideaName}` plus the extension. The response of a mismatch carries the expected name so the client can show it.

## Why check content, not only the extension

An extension is a claim by the client. The server checks that the bytes agree with the claim:

| Type | Check |
|---|---|
| PDF | First five bytes are `%PDF-` |
| PPTX | Opens as a ZIP, and the central directory lists `[Content_Types].xml` and `ppt/presentation.xml` |

This rejects renamed files (an `.exe` called `.pdf`) cheaply. It does **not** prove the file is safe. See [what is not checked](./architecture#what-is-deliberately-not-checked).

## Order of checks

Cheap checks run first, and everything runs **before** the first byte reaches MinIO.

1. Authentication, CSRF, role and state (before the body is read).
2. Body size limit while streaming.
3. Text fields, then file rules in the order above.
4. Duplicate-name pre-check (create).
5. Upload to MinIO.
6. Database transaction: row change and audit row.

A request that fails at step 1 to 4 never touches storage. This keeps rejected uploads cheap and keeps the object store free of files that were never going to be accepted.

## Enterprise practice compared with this project

| Topic | Common enterprise practice | This project |
|---|---|---|
| Large files | Direct-to-storage upload with a pre-signed URL | Streamed through the API into a temp file, then to MinIO |
| Content check | Magic bytes plus a document parser | Magic bytes and ZIP structure only |
| Malware | Antivirus or sandbox scan before release | None in v1 (PRD) |
| Storage key | Random, unrelated to user input | `slides/{ideaId}/{uuid}.{ext}`; the user file name is never part of the key |
| Consistency | Upload first, then a reconciler removes orphans | Upload first, compensate by outcome, reconciler table |
| Download | Pre-signed URL | Streamed through the API so authorization and audit apply |

## Where to read next

- [Architecture](./architecture): request pipeline, limits at each layer, storage consistency.
- [Implementation](./implementation): code walk-through and how to change a rule.
- [Decision Thinking](./decisions): why each rule is what it is.
- [Technical FAQ](./faq): common questions.

# File Upload: Decision Thinking

This page records **why** the upload checks look the way they do: the question, the options, the choice and what it costs. Entries marked *(documented)* follow reasons written in the code or design documents. Entries marked *(inferred)* are the likely reason read from the code; confirm them with the owner before quoting them as fact.

## A framework for upload decisions

1. **What is the worst thing a file can do here?** Be stored and served to other users? Exhaust disk or memory? Mislead a reviewer?
2. **Who is the sender?** Authenticated employees, so abuse is traceable. This lowers, not removes, the risk.
3. **Can the check be done before any side effect?** Cheap, side-effect-free checks go first.
4. **What fails if the check is wrong?** A false reject annoys a user. A false accept stores a bad file.
5. **Which layer should own the rule?** The layer that can see the full information, once.
6. **Can a test pin it?** Prefer a rule a test checks.

## Decisions

### U1. Validate everything before uploading *(documented, D37)*

| | |
|---|---|
| Question | When may the first byte go to MinIO? |
| Options | Validate first; upload first and delete if invalid |
| Choice | All text and file validation completes first |
| Why | A rejected request leaves no object to clean up, and storage load follows accepted work only. |
| Cost | The file is spooled to local disk first. |

### U2. Spool to a temp file instead of memory or a straight pipe *(documented)*

| | |
|---|---|
| Question | Where does the body live while it is checked? |
| Options | Memory; stream straight to MinIO; temp file |
| Choice | Temp file via `MultipartReader`, size and SHA-256 computed while copying |
| Why | The ZIP check needs random access to the end of the file. Memory use stays constant. A direct pipe would upload before validation (U1). |
| Cost | Disk I/O and a bounded `emptyDir`; concurrent large uploads compete for it. |

### U3. Magic bytes plus ZIP markers, no deeper parse *(documented: no antivirus or macro scan)*

| | |
|---|---|
| Question | How much of the file's content should the server trust itself to judge? |
| Options | Extension only; magic bytes; full parser; antivirus |
| Choice | Magic bytes, and for PPTX the two required ZIP entries |
| Why | Stops renamed files at low cost and low parser attack surface. A full parser is a vulnerability source of its own. |
| Cost | A well-formed but malicious document passes. Residual risk sits with the viewer. |

### U4. Two limits: transport looser than business *(documented)*

| | |
|---|---|
| Question | Where does exactly 5 MiB get decided? |
| Choice | Transport limit 5 MiB + 64 KiB; the validator enforces 5,242,880 exactly |
| Why | One place owns the rule, and framing overhead cannot cause a false 413. |
| Cost | The server may read up to 64 KiB more than the business limit before rejecting. |

### U5. Keep the 413 check before the body is read *(documented)*

| | |
|---|---|
| Question | In what order do auth, size and body reading run? |
| Choice | 413 → 401 → CSRF 403 → 404 / role / state, all before the multipart body is read |
| Why | A user who may not upload, or an anonymous caller, cannot make the server read 5 MiB. |
| Cost | Role and state are checked twice: before the body and again inside the transaction. |

### U6. The server decides the file name *(inferred)*

| | |
|---|---|
| Question | Who chooses the stored file's name? |
| Options | Accept the user's name; rename on receipt; require the user's name to equal an expected name |
| Choice | Require equality with `{I\|O}_{org}_{leader}_{empNo}_{idea}` (after NFC) |
| Why | Organizers get files that sort and identify themselves. The rule is checkable and gives the client the expected name on failure. |
| Cost | Users must rename files; any change to the Idea name changes the expected name. |

### U7. NFC normalization before comparison *(documented: NFD names covered by contract tests)*

| | |
|---|---|
| Question | Which "same" name counts as the same? |
| Choice | Compare after Unicode NFC |
| Why | macOS can send decomposed (NFD) names for the same visible text. Without NFC a user would see a mismatch between identical-looking names. |
| Cost | Names that differ only in normalization are treated as equal. |

### U8. Fresh object key per upload; never the user's name *(documented)*

| | |
|---|---|
| Question | What is the key in storage? |
| Choice | `slides/{ideaId}/{uuid}.{ext}` |
| Why | No path traversal or collision from user input, and a retried upload cannot overwrite an object that an unknown-outcome attempt may reference. |
| Cost | Old keys must be tracked and removed (U9). |

### U9. Upload first, compensate by outcome *(documented, D25, D36)*

| | |
|---|---|
| Question | How do the database and storage stay consistent? |
| Options | Two-phase commit; database first then upload; upload first then compensate |
| Choice | Upload first; delete now on rollback; keep and record on unknown; a pending table with a job |
| Why | A row never points to a missing object. Orphan objects cost money, not correctness. |
| Cost | Orphans exist for a while. The unknown-outcome grace is 10 minutes (OI-19, proposed). The clean-up job is not written yet. |

### U10. Stream downloads through the API *(inferred)*

| | |
|---|---|
| Question | Download through the app or a pre-signed URL? |
| Choice | Through the app |
| Why | The role check, the uniform 404 and the `SLIDES_DOWNLOAD` audit row all stay on the path. A pre-signed URL could be shared and would bypass the audit. |
| Cost | The API pays the bandwidth and holds a connection for up to 30 s per call. |

### U11. One error code for both "wrong type" and "content lies" *(inferred)*

| | |
|---|---|
| Question | Should the client learn which check failed? |
| Choice | Both are `UNSUPPORTED_FILE_TYPE` |
| Why | The client needs to know what to fix, not how the check works. Less detail helps a probing client less. |
| Cost | The message is less specific for a user whose file is corrupt. |

## Worked example: adding a "no password-protected PDF" rule

1. **Question**: Reviewers cannot open protected files. Is that a security rule or a quality rule? Quality.
2. **Where**: Needs reading the PDF trailer, so it cannot be done from the first 5 bytes.
3. **Cost of being wrong**: False rejects block valid submissions near a deadline.
4. **Option**: Reject only if the trailer contains `/Encrypt`; keep the code as `UNSUPPORTED_FILE_TYPE` or add a new one.
5. **Do it as**: a new case in `Validate` after the magic check, a table test, and a catalogue update if a code is added.

## Known gaps

- No malware or macro scanning.
- No clean-up job yet for `pending_file_deletion`.
- The 10-minute grace after an unknown outcome is proposed, not agreed (OI-19).
- Behaviour under many concurrent large uploads depends on the `/tmp` size limit; no load test result is recorded here.

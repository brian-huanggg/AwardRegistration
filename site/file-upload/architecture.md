# File Upload: Architecture

## Request pipeline

```text
Client
  │  multipart/form-data
  ▼
Ingress (6 MiB)
  ▼
Handler
  1. Role and state pre-check          (before the body is read)
  2. Content-Length > 5 MiB + 64 KiB ? → 413
  3. http.MaxBytesReader(5 MiB + 64 KiB)
  4. web.ReadMultipart: stream the file part into a temp file,
     computing size and SHA-256 on the way
  ▼
Service
  5. Text validation (422)
  6. storage.Validate: size, empty, extension + magic bytes, file name
  7. Duplicate pre-check (create)
  8. Put to MinIO under a fresh key
  9. Transaction: row change + pending row + audit row
 10. After the outcome: delete or keep the object
```

The handler removes the temp file in a `defer`, whether the request succeeded or failed.

## Size limits by layer

| Layer | Limit | Purpose |
|---|---|---|
| Ingress | 6 MiB | Protects the whole cluster edge. Its 413 body may be HTML. |
| `Content-Length` check | 5 MiB + 64 KiB | Rejects an honest oversized request before reading it. |
| `http.MaxBytesReader` | 5 MiB + 64 KiB | Stops a request that lies about or omits its length. |
| `storage.Validate` | exactly 5,242,880 bytes | The business rule. The extra 64 KiB covers text parts and multipart framing. |

The transport limit is deliberately looser than the business limit so the exact boundary is decided in one place, the validator.

## Spooling to disk

`ReadMultipart` uses `r.MultipartReader()` and copies the file part with `io.Copy` into `os.CreateTemp`, through `io.MultiWriter(file, sha256)`.

- `ParseMultipartForm` is not used. It buffers in memory and creates temp files the code does not control.
- Memory use per request is constant, not proportional to file size.
- The temp directory is the pod's `/tmp` `emptyDir`, with a size limit of about 256 MiB. Concurrent uploads therefore share a bounded disk budget.
- Text parts are capped at 64 KiB each and must be valid UTF-8. Unknown parts are drained and ignored.
- The file name is read from `Content-Disposition` as sent. `Part.FileName()` is avoided because it applies `filepath.Base`, which would hide path components that the file-name rule must see.

Spooling lets the validator use random access (the ZIP central directory is at the end of the file) and lets the upload to MinIO use a known size.

## Storage consistency

The database and MinIO cannot commit together. The design uploads first and compensates according to the **outcome** of the transaction.

```text
Put(new key)  ──►  BEGIN … COMMIT
                      │
        ┌─────────────┼───────────────────┐
     Committed     RolledBack          Unknown (COMMIT error)
        │             │                    │
   delete old     delete new now      keep new object,
   object (best   (if that fails,     record UPLOAD_ABORTED
   effort)        record it as        due after 10 minutes
                  UPLOAD_ABORTED)
```

| Outcome | New object | Old object (replace/delete) | Response |
|---|---|---|---|
| Committed | Referenced by the row | Deleted after commit; if that fails, the `pending_file_deletion` row stays for the job | Success |
| RolledBack | Deleted now; on failure a pending row, due at once | Untouched | The error that caused the rollback |
| Unknown | Kept; pending row due after a 10-minute grace | Untouched | `503 SERVICE_UNAVAILABLE` |

Rules that make this safe:

- **Keys are never reused.** Every upload gets `slides/{ideaId}/{uuid}.{ext}`. A retry cannot overwrite an object that an earlier, possibly committed, attempt references.
- **The pending row for an old object is written inside the transaction** that moves the pointer, so it exists exactly when the change committed.
- **Compensation ignores client cancellation** (`context.WithoutCancel`), so a disconnecting client cannot skip the cleanup.
- **Compensation never fails the request.** It is logged. The request is already failing, or already committed.
- **After an unknown outcome the object is kept.** The COMMIT may have been applied, and deleting a referenced object would break the Idea. The clean-up job removes the key only if no Idea row references it.

## Download

Downloads stream through the API:

- The role check and the `SLIDES_DOWNLOAD` audit row share one transaction; the object is opened only after they succeed.
- Responses set `Content-Type` from the stored extension, a `Content-Disposition` built from the expected file name, and `X-Content-Type-Options: nosniff`.

## Failure behaviour

| Failure | Result |
|---|---|
| MinIO unreachable (2 s dial timeout, one retry, 30 s per call) | `503 STORAGE_UNAVAILABLE`; no row changed |
| Temp disk full | Plain error, `500`; the client is not blamed |
| Client disconnects mid-body | Read error, `400`; temp file removed |
| Database down after upload | Rolled back or unknown; compensated as above |

## What is deliberately not checked

- **Malware and macros.** There is no antivirus or macro scan in v1 (PRD §6.8). A valid-looking PDF or PPTX can still contain active content.
- **Document structure beyond the markers.** A ZIP with the two required entries passes, even if the rest is not a usable presentation.
- **Decompression size.** The central directory is read, but entries are never extracted, so a zip bomb is not expanded by the server.
- **Content type from the client.** The `Content-Type` header of the part is ignored; the extension and bytes decide.

## Current status

- Implemented: validator, multipart spooling, create and replace with compensation, delete with pending row, download with audit, MinIO adapter.
- Not in the code yet: the clean-up job that drains `pending_file_deletion` (named "next phase" in the source). Until it exists, rows with a due time are not processed.

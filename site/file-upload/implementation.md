# File Upload: Implementation

Paths are relative to `backend/internal/`.

## Where the code lives

| Concern | File |
|---|---|
| Streaming multipart, temp file, SHA-256 | `shared/web/multipart.go` |
| Size, empty, extension, magic bytes, file name | `storage/validator.go` |
| Expected file name and its length rule | `storage/expectedfilename.go` |
| MinIO client, timeouts, error mapping | `storage/minio.go` |
| `pending_file_deletion` and compensation | `storage/pending.go` |
| Create flow and handler | `idea/create.go` |
| Replace flow | `idea/replace.go` |
| Adapters between `idea` and `storage` | `server/slides.go` |

`idea` does not import `storage`. `server/slides.go` adapts the storage functions into the small interfaces `idea` declares (`SlideRules`, `ObjectStore`, `UploadAborter`), which keeps the dependency direction the import-graph test requires.

## The validator

`storage.Validate(parts, expectedBase)` stops at the first failure, in this order:

```go
if len(parts) != 1            { /* 400 MALFORMED_REQUEST */ }
if p.Size > MaxSlideBytes     { /* 413 FILE_TOO_LARGE */ }
if p.Size == 0                { /* 422 EMPTY_FILE */ }
kind, ok := kindFromExtension(p.Filename)
if !ok || !magicMatches(kind, p) { /* 415 UNSUPPORTED_FILE_TYPE */ }
got := norm.NFC.String(p.Filename)
if got[:len(got)-len(kind)-1] != expectedBase { /* 422 FILENAME_MISMATCH */ }
```

Details worth knowing:

- Extension is matched case-insensitively. The extension is ASCII, so slicing the base off after NFC normalization is safe.
- A wrong type and a type that does not match its bytes share one code, `UNSUPPORTED_FILE_TYPE`, so the response does not reveal which check failed.
- `Part.Data` is an `io.ReaderAt`. The PDF check reads five bytes. The PPTX check lets `archive/zip` read the central directory. Neither reads the whole file.
- The mismatch error carries `Expected` in its field error so the client can display the correct name.

## Expected file name

`ExpectedFilenameBase` concatenates `{I|O}_{org}_{leaderName}_{leaderEmpNo}_{ideaName}` and normalizes to NFC, so the result does not depend on how the inputs were stored. `CheckFilenameLength` assumes the longer extension `.pptx`, because the type is unknown when an Idea is created or edited, and limits the name to 255 UTF-8 bytes. An unknown category is a programming error and returns `INTERNAL_ERROR`, not a guess.

## Streaming

`ReadMultipart(r, spec)` takes a `MultipartSpec` that describes the one shape a route accepts:

```go
var createSpec = web.MultipartSpec{
    FileField:  "slides",
    TextFields: []string{"category", "org", "direction", "name", "summary"},
    TooLarge:   web.CodeFileTooLarge,
}
```

It returns a `*Form` that **must** be closed. `Close` removes the temp file, is safe on nil, and is safe to call twice. On any error inside `ReadMultipart` nothing stays on disk.

## Create, step by step

`Service.Create` in `idea/create.go`:

1. Load the leader name, validate text, build the expected base.
2. `Slides.Validate` (file rules).
3. Duplicate-name pre-check. The order puts 422 before 409.
4. Build the object key with a fresh UUID and `Put` to MinIO.
5. `db.InTx`: re-check the deadline (`AssertOpenInTx`), insert the Idea, write `IDEA_CREATE`, read back.
6. By outcome: return the Idea, or `AbortUpload` and return the error.

`ReplaceSlides` follows the same pattern but locks the Idea row, re-checks leader and state inside the transaction, switches the pointer, writes the `REPLACED` pending row and `SLIDES_REPLACE`, and after commit deletes the old object best effort.

## Audit of uploads

| Event | When | Slide data stored |
|---|---|---|
| `IDEA_CREATE` | Create commits | `ext`, `size`, `sha256` in `after` |
| `SLIDES_REPLACE` | Replace commits | Old and new metadata and the uploaded file name |
| `SLIDES_DOWNLOAD` | Download authorized | Target Idea |
| Denied variants | Role, state, hidden resource | Reason; written by the central error adapter |

The audit row holds **metadata, never content**. See [Audit Log](/audit-log/) for how it is written.

## Adding or changing a rule

1. Add the check to `storage.Validate` at the right position. Cheaper checks go earlier.
2. If it needs a new error code, update the catalogue, `testdata/error-codes.json`, `docs/API/error-codes.md` and both dictionaries (project rule).
3. Add a table-driven case to `storage/validator_test.go`.
4. If the rule changes what a client must show, update the front-end message.
5. Check the permission matrix cells the route touches.

## Tests

| File | What it covers (from test names) |
|---|---|
| `storage/validator_test.go` | `TestValidate`: the rules and their order |
| `shared/web/multipart_test.go` | Success, non-multipart, malformed body removes the temp file, truncated body, body limit, unwritable temp dir, `Close` on nil |
| `idea/replace_integration_test.go` | Replace success, file errors, validation against the current name, upload failure, failed old-object delete keeps the pending row, concurrent replaces leave no orphan |
| `idea/fault_integration_test.go` | Lost COMMIT on replace and delete; storage down answers 503 |
| `storage/pending_integration_test.go` | Pending rows, abort outcomes, behaviour around a lost commit |
| `storage/minio_integration_test.go` | MinIO adapter against a real server |

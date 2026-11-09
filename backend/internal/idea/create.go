package idea

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"slices"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/text"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Text limits in code points (PRD 6.6).
const (
	maxIdeaName = 75
	maxSummary  = 300
)

// Field names as they appear in errors[]; the order is the form order.
const (
	fieldCategory  = "category"
	fieldOrg       = "org"
	fieldDirection = "direction"
	fieldName      = "name"
	fieldSummary   = "summary"
	fieldSlides    = "slides"
)

// Audit reasons of a refused create (PRD 6.11: role and state are DENIED).
const (
	auditReasonRole  = "ROLE"
	auditReasonState = "STATE"
)

// contentTypes are the stored object types per accepted extension.
var contentTypes = map[string]string{
	"pdf":  "application/pdf",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

// SlideFile is the spooled slides part as the storage rules need it.
type SlideFile struct {
	Filename string // as sent
	Size     int64
	SHA256   [32]byte
	Data     io.ReaderAt      // random access for the validator
	Open     func() io.Reader // a fresh reader from the start, for the upload
}

// SlideRules is the storage-side slide logic (storage.ExpectedFilenameBase,
// CheckFilenameLength, Validate). It is injected by main because idea must not
// import storage (D29); every function returns *web.Error for client faults.
type SlideRules struct {
	ExpectedBase func(category, org, leaderName, leaderEmpNo, ideaName string) (string, error)
	CheckLength  func(base string) error
	// Validate runs PRD 6.8 steps 2–5 and returns the extension ("pdf"/"pptx").
	Validate func(f SlideFile, expectedBase string) (ext string, err error)
}

// ObjectStore is the part of the MinIO port idea needs (storage.SlideStore):
// Put for create, Get for download. Outages are *web.Error
// STORAGE_UNAVAILABLE; any other Get error (a missing object) answers 500.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	// Get opens key; the caller closes the reader. size is the object length.
	Get(ctx context.Context, key string) (rc io.ReadCloser, size int64, err error)
}

// UploadAborter compensates an uploaded object whose transaction did not
// commit (storage.PendingDeletion.AbortUpload, Sequence.md §2): delete it now
// after a known rollback, keep it and record UPLOAD_ABORTED after an unknown
// outcome. It is best effort and never fails the request.
type UploadAborter interface {
	AbortUpload(ctx context.Context, key string, outcome db.Outcome)
}

// CreateCommand is the raw, unvalidated form. The Leader is the caller.
type CreateCommand struct {
	Category, Org, Direction, Name, Summary string
	Slides                                  SlideFile
}

// Request is the transport facts the audit row needs.
type Request struct {
	Method, Route, UserAgent string
	IP                       netip.Addr
}

// validText is the cleaned, validated text of a create.
type validText struct {
	category, org, direction, name, summary string
}

// Create runs POST /ideas after the role check and Precheck (both done by the
// handler before the body is read). Order (PRD 6.1, 6.8, Sequence.md §3):
// text 422 (incl. FILENAME_TOO_LONG) → file 413/422/415/422 → duplicate
// pre-check 409 → upload (503) → transaction: AssertOpenInTx, INSERT,
// IDEA_CREATE. The returned Idea is read back inside the same transaction.
// A transaction that does not commit is compensated by its outcome (B3-09);
// an unknown outcome answers 503 SERVICE_UNAVAILABLE.
func (s *Service) Create(ctx context.Context, caller security.Principal, cmd CreateCommand, req Request) (Idea, error) {
	leaderName, err := store.LeaderName(ctx, s.DB, caller.UserID)
	if err != nil {
		return Idea{}, err
	}
	v, base, err := s.validateText(cmd, leaderName, caller.EmpNo)
	if err != nil {
		return Idea{}, err
	}
	ext, err := s.Slides.Validate(cmd.Slides, base)
	if err != nil {
		return Idea{}, err
	}
	// 422 wins over 409 (PRD 6.1), so the pre-check runs after every validation.
	taken, err := store.NameTaken(ctx, s.DB, caller.UserID, v.name)
	if err != nil {
		return Idea{}, err
	}
	if taken {
		return Idea{}, duplicateErr(nil)
	}

	n := store.NewIdea{
		ID: uuid.New(), LeaderID: caller.UserID,
		Category: v.category, Org: v.org, Direction: v.direction, Name: v.name, Summary: v.summary,
	}
	n.Slides = store.Slides{
		ObjectKey: "slides/" + n.ID.String() + "/" + uuid.NewString() + "." + ext,
		Ext:       ext,
		Size:      cmd.Slides.Size,
		SHA256:    hex.EncodeToString(cmd.Slides.SHA256[:]),
	}
	// A failed upload leaves nothing: no row exists and no later step runs.
	if err := s.Objects.Put(ctx, n.Slides.ObjectKey, cmd.Slides.Open(), n.Slides.Size, contentTypes[ext]); err != nil {
		return Idea{}, err
	}

	after, err := createAfter(n)
	if err != nil {
		return Idea{}, err
	}
	var got Idea
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return denied(err, auditReasonState)
		}
		if err := store.InsertIdea(ctx, tx, n); err != nil {
			return err
		}
		if err := s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.IdeaCreate, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: n.ID,
			Method: req.Method, Route: req.Route, Status: http.StatusCreated,
			IP: req.IP, UserAgent: req.UserAgent,
			After: after,
		}); err != nil {
			return err
		}
		got, err = store.GetVisible(ctx, tx, n.ID, caller.UserID, caller.IsAdmin())
		return err
	})
	if out == db.Committed {
		return got, nil
	}
	// Known rollback: the object is deleted now. Unknown: the COMMIT may have
	// been applied, so the object stays and the clean-up job keeps it if an
	// idea row references the key.
	s.Aborter.AbortUpload(ctx, n.Slides.ObjectKey, out)
	if out == db.Unknown {
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check your Ideas before retrying.")
		e.Cause = err
		return Idea{}, e
	}
	if _, dup := db.DuplicateKey(err); dup {
		return Idea{}, duplicateErr(err)
	}
	return Idea{}, err
}

// validateText cleans and checks every text field, collecting each field's
// first error in form order (PRD 6.2). When category, org and name are valid,
// the expected filename is built and its length checked; FILENAME_TOO_LONG is
// reported in the name slot. On success it returns the expected base.
func (s *Service) validateText(cmd CreateCommand, leaderName, leaderEmpNo string) (validText, string, error) {
	var (
		v      validText
		fields []web.FieldError
	)
	add := func(field string, code web.ErrorCode) {
		fields = append(fields, web.FieldError{Field: field, Code: code})
	}
	enum := func(field, raw string, allowed []string, dst *string) {
		// Codes are compared exactly, never cleaned up into a match; only
		// "empty after clean-up" is REQUIRED (PRD 6.1).
		switch {
		case text.Trim(raw) == "":
			add(field, web.CodeRequired)
		case !slices.Contains(allowed, raw):
			add(field, web.CodeInvalidEnum)
		default:
			*dst = raw
		}
	}
	enum(fieldCategory, cmd.Category, categories, &v.category)
	enum(fieldOrg, cmd.Org, orgs, &v.org)
	enum(fieldDirection, cmd.Direction, directions, &v.direction)

	var base string
	name, p := text.CheckName(cmd.Name, maxIdeaName)
	switch {
	case p != text.None:
		add(fieldName, web.ErrorCode(p))
	case v.category != "" && v.org != "":
		b, err := s.Slides.ExpectedBase(v.category, v.org, leaderName, leaderEmpNo, name)
		if err != nil {
			return validText{}, "", err
		}
		if err := s.Slides.CheckLength(b); err != nil {
			var e *web.Error
			if !errors.As(err, &e) {
				return validText{}, "", err
			}
			add(fieldName, e.Code)
		} else {
			base = b
		}
	}
	v.name = name

	summary, p := text.CheckSummary(cmd.Summary, maxSummary)
	if p != text.None {
		add(fieldSummary, web.ErrorCode(p))
	}
	v.summary = summary

	if len(fields) > 0 {
		e := web.NewError(topCode(fields), "the request has invalid fields")
		e.Fields = fields
		return validText{}, "", e
	}
	return v, base, nil
}

// topCode is the response code of a 422: the first field error's code when it
// is a specific registered code, else VALIDATION_FAILED (error-codes.md §2).
func topCode(fields []web.FieldError) web.ErrorCode {
	switch c := fields[0].Code; c {
	case web.CodeInvalidNameChars, web.CodeFilenameTooLong:
		return c
	}
	return web.CodeValidationFailed
}

// createAfter is the IDEA_CREATE after document: every stored field, with the
// slide metadata but never the content (PRD 6.11).
func createAfter(n store.NewIdea) (json.RawMessage, error) {
	b, err := json.Marshal(map[string]any{
		fieldCategory: n.Category, fieldOrg: n.Org, fieldDirection: n.Direction,
		fieldName: n.Name, fieldSummary: n.Summary,
		fieldSlides: map[string]any{"ext": n.Slides.Ext, "size": n.Slides.Size, "sha256": n.Slides.SHA256},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal IDEA_CREATE after: %w", err)
	}
	return b, nil
}

func duplicateErr(cause error) *web.Error {
	e := web.NewError(web.CodeIdeaDuplicate, "You already lead an Idea with this name.")
	e.Cause = cause
	return e
}

// denied marks a role or state refusal of a create for the DENIED audit row
// (track B, written by the central error adapter). Other errors pass through.
func denied(err error, reason string) error {
	var e *web.Error
	if errors.As(err, &e) && (e.Code == web.CodeForbidden ||
		e.Code == web.CodeSubmissionNotOpen || e.Code == web.CodeSubmissionClosed) {
		e.AuditAction = string(audit.IdeaCreate)
		e.AuditReason = reason
	}
	return err
}

// maxCreateBody is the application's body limit for a create: one 5 MiB file
// plus 64 KiB for the text parts and multipart framing (PRD 6.8). The exact
// 5 MiB file boundary is enforced by the validator, not by this limit.
const maxCreateBody = 5*1024*1024 + 64*1024

// createSpec is the one multipart shape of POST /ideas. Any other part (for
// example a client-sent leader) is drained and ignored (PRD 6.6).
var createSpec = web.MultipartSpec{
	FileField:  fieldSlides,
	TextFields: []string{fieldCategory, fieldOrg, fieldDirection, fieldName, fieldSummary},
	TooLarge:   web.CodeFileTooLarge,
}

// CreateIdea serves POST /ideas: 201 + Location + IdeaDetail. Role 403 and the
// state pre-check run before the body is read (Backend.md §3).
func (h Handler) CreateIdea(w http.ResponseWriter, r *http.Request) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	if p.IsAdmin() {
		h.Fail(w, r, denied(web.NewError(web.CodeForbidden, "Admins cannot create Ideas."), auditReasonRole))
		return
	}
	if err := h.Service.Gate.Precheck(r.Context(), p); err != nil {
		h.Fail(w, r, denied(err, auditReasonState))
		return
	}
	if r.ContentLength > maxCreateBody {
		h.Fail(w, r, web.NewError(web.CodeFileTooLarge, "Request body too large."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	form, err := web.ReadMultipart(r, createSpec)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	defer func() { _ = form.Close() }()

	f := form.File
	got, err := h.Service.Create(r.Context(), p, CreateCommand{
		Category: form.Fields[fieldCategory], Org: form.Fields[fieldOrg], Direction: form.Fields[fieldDirection],
		Name: form.Fields[fieldName], Summary: form.Fields[fieldSummary],
		Slides: SlideFile{Filename: f.Filename, Size: f.Size, SHA256: f.SHA256, Data: f.ReaderAt(), Open: f.Open},
	}, Request{Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r)})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	body, err := h.toDetail(r.Context(), got, p)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/ideas/"+got.ID.String())
	web.WriteJSON(w, http.StatusCreated, body)
}

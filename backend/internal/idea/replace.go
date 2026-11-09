package idea

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// ReplacedKeys records and removes the old object of a replacement
// (storage.PendingDeletion, adapted by main because idea must not import
// storage, D29). RecordReplaced runs inside the switching transaction, so the
// row exists exactly when the pointer moved. DeleteAfterCommit is best effort
// and never fails the request.
type ReplacedKeys interface {
	RecordReplaced(ctx context.Context, tx *sql.Tx, key string) (id int64, err error)
	DeleteAfterCommit(ctx context.Context, id int64, key string)
}

// AuthorizeReplace runs the checks that come before the body is read (PRD 6.1,
// Sequence.md §4): 404 for a missing or hidden Idea → 403 FORBIDDEN for a
// viewer who is neither Leader nor Admin (Member) → 403 SUBMISSION_* by the
// read-only state pre-check (Admin is exempt). Each refusal is audited as
// DENIED.
func (s *Service) AuthorizeReplace(ctx context.Context, id uuid.UUID, caller security.Principal) error {
	got, err := s.Get(ctx, id, caller)
	if err != nil {
		return markDenied(err, audit.SlidesReplace, id, auditReasonHidden)
	}
	if !caller.IsAdmin() && got.LeaderID != caller.UserID {
		return markDenied(web.NewError(web.CodeForbidden, "Only the Leader or an Admin can replace the slides."),
			audit.SlidesReplace, id, auditReasonRole)
	}
	return markDenied(s.Gate.Precheck(ctx, caller), audit.SlidesReplace, id, auditReasonState)
}

// ReplaceSlides runs PUT /ideas/{id}/slides after AuthorizeReplace. Order:
// file 413/422/415/422 against the CURRENT Idea data → upload (503) →
// transaction: lock row, leader/Admin and AssertOpenInTx again, switch the
// pointer (+1 version), REPLACED row, SLIDES_REPLACE, read back. After commit
// the old object is deleted best effort. A transaction that does not commit
// is compensated by its outcome (B3-09); an unknown outcome answers 503.
func (s *Service) ReplaceSlides(ctx context.Context, id uuid.UUID, caller security.Principal, file SlideFile, req Request) (Idea, error) {
	cur, err := s.Get(ctx, id, caller)
	if err != nil {
		return Idea{}, markDenied(err, audit.SlidesReplace, id, auditReasonHidden)
	}
	base, err := s.Slides.ExpectedBase(cur.Category, cur.Org, cur.LeaderName, cur.LeaderEmpNo, cur.Name)
	if err != nil {
		return Idea{}, err
	}
	ext, err := s.Slides.Validate(file, base)
	if err != nil {
		return Idea{}, err
	}
	next := store.Slides{
		ObjectKey: "slides/" + id.String() + "/" + uuid.NewString() + "." + ext,
		Ext:       ext,
		Size:      file.Size,
		SHA256:    hex.EncodeToString(file.SHA256[:]),
	}
	// A failed upload leaves nothing: no row changed and no later step runs.
	if err := s.Objects.Put(ctx, next.ObjectKey, file.Open(), next.Size, contentTypes[ext]); err != nil {
		return Idea{}, err
	}

	var (
		got       Idea
		oldKey    string
		pendingID int64
	)
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		leader, old, err := store.LockSlides(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			return markDenied(web.NewError(web.CodeNotFound, ""), audit.SlidesReplace, id, auditReasonHidden)
		}
		if err != nil {
			return err
		}
		if !caller.IsAdmin() && leader != caller.UserID {
			return markDenied(web.NewError(web.CodeForbidden, "Only the Leader or an Admin can replace the slides."),
				audit.SlidesReplace, id, auditReasonRole)
		}
		if err := s.Gate.AssertOpenInTx(ctx, tx, caller); err != nil {
			return markDenied(err, audit.SlidesReplace, id, auditReasonState)
		}
		if err := store.SwitchSlides(ctx, tx, id, next); err != nil {
			return err
		}
		if pendingID, err = s.Replaced.RecordReplaced(ctx, tx, old.ObjectKey); err != nil {
			return err
		}
		oldKey = old.ObjectKey
		before, after, err := replaceDocs(old, next, file.Filename)
		if err != nil {
			return err
		}
		if err := s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.SlidesReplace, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusOK,
			IP: req.IP, UserAgent: req.UserAgent,
			Before: before, After: after,
		}); err != nil {
			return err
		}
		got, err = store.GetVisible(ctx, tx, id, caller.UserID, caller.IsAdmin())
		return err
	})
	if out == db.Committed {
		s.Replaced.DeleteAfterCommit(ctx, pendingID, oldKey)
		return got, nil
	}
	s.Aborter.AbortUpload(ctx, next.ObjectKey, out)
	if out == db.Unknown {
		e := web.NewError(web.CodeServiceUnavailable, "The result of the request is unknown; check your Idea before retrying.")
		e.Cause = err
		return Idea{}, e
	}
	return Idea{}, err
}

// replaceDocs are the SLIDES_REPLACE documents: metadata only, never content
// (PRD 6.11). The new side also carries the filename as sent.
func replaceDocs(old, next store.Slides, filename string) (before, after json.RawMessage, err error) {
	if before, err = json.Marshal(map[string]any{"ext": old.Ext, "size": old.Size, "sha256": old.SHA256}); err != nil {
		return nil, nil, fmt.Errorf("marshal SLIDES_REPLACE before: %w", err)
	}
	if after, err = json.Marshal(map[string]any{
		"filename": filename, "ext": next.Ext, "size": next.Size, "sha256": next.SHA256,
	}); err != nil {
		return nil, nil, fmt.Errorf("marshal SLIDES_REPLACE after: %w", err)
	}
	return before, after, nil
}

// replaceSpec is the one multipart shape of PUT /ideas/{id}/slides; any other
// part is drained and ignored.
var replaceSpec = web.MultipartSpec{FileField: fieldSlides, TooLarge: web.CodeFileTooLarge}

// ReplaceSlides serves PUT /ideas/{id}/slides: 200 + IdeaDetail. 404, role and
// state run before the body is read (Backend.md §3).
func (h Handler) ReplaceSlides(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	if err := h.Service.AuthorizeReplace(r.Context(), id, p); err != nil {
		h.Fail(w, r, err)
		return
	}
	if r.ContentLength > maxCreateBody {
		h.Fail(w, r, web.NewError(web.CodeFileTooLarge, "Request body too large."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)
	form, err := web.ReadMultipart(r, replaceSpec)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	defer func() { _ = form.Close() }()

	f := form.File
	got, err := h.Service.ReplaceSlides(r.Context(), id, p,
		SlideFile{Filename: f.Filename, Size: f.Size, SHA256: f.SHA256, Data: f.ReaderAt(), Open: f.Open},
		Request{Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r)})
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	body, err := h.toDetail(r.Context(), got, p)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, body)
}

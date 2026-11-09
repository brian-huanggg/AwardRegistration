package idea

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// Download is an opened slide file. Body yields exactly Idea.Slides.Size bytes
// unless storage fails mid-stream; the caller must Close it.
type Download struct {
	Idea Idea
	Body io.ReadCloser
}

// Download runs GET /ideas/{id}/slides (Sequence.md §6): the visibility query
// and the SLIDES_DOWNLOAD row share one transaction, and the object is opened
// only after that transaction has committed, so no byte is served without a
// durable audit row (fail-closed, PRD 6.11). A missing and a hidden Idea both
// return NOT_FOUND. An unknown commit outcome serves nothing (503), because
// the audit row may not exist. Downloads are never state-limited.
//
// If storage fails after the commit, the audit row stays for a download that
// answered 503: accepted, the intent was real (Sequence.md §6).
func (s *Service) Download(ctx context.Context, id uuid.UUID, caller security.Principal, req Request) (Download, error) {
	var got Idea
	out, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		if got, err = store.GetVisible(ctx, tx, id, caller.UserID, caller.IsAdmin()); err != nil {
			return err
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.SlidesDownload, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
			Method: req.Method, Route: req.Route, Status: http.StatusOK,
			IP: req.IP, UserAgent: req.UserAgent,
		})
	})
	switch {
	case out == db.Unknown:
		e := web.NewError(web.CodeServiceUnavailable, "The download could not be recorded; please retry.")
		e.Cause = err
		return Download{}, e
	case errors.Is(err, store.ErrNotFound):
		return Download{}, web.NewError(web.CodeNotFound, "")
	case err != nil:
		return Download{}, err
	}

	// Storage outages are *web.Error STORAGE_UNAVAILABLE (503); a missing
	// object is a consistency fault and answers 500.
	body, size, err := s.Objects.Get(ctx, got.Slides.ObjectKey)
	if err != nil {
		return Download{}, fmt.Errorf("open slides of idea %s: %w", id, err)
	}
	// Content-Length is the stored slides_size; an object of another length
	// would make every client see a truncated or overlong body.
	if size != got.Slides.Size {
		_ = body.Close()
		return Download{}, fmt.Errorf("slides of idea %s: object is %d bytes, row says %d", id, size, got.Slides.Size)
	}
	return Download{Idea: got, Body: body}, nil
}

// DownloadSlides serves GET /ideas/{id}/slides: 200 with the file named by the
// current expected filename (PRD 6.6, 6.8). HEAD never reaches this handler
// (explicit 405 route, D47/D59).
func (h Handler) DownloadSlides(w http.ResponseWriter, r *http.Request, id api.IdeaId) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	d, err := h.Service.Download(r.Context(), uuid.UUID(id), p,
		Request{Method: r.Method, Route: web.RoutePattern(r), UserAgent: r.UserAgent(), IP: web.ClientIP(r)})
	if err != nil {
		var e *web.Error
		switch {
		case errors.As(err, &e) && e.Code == web.CodeNotFound:
			e.AuditAction = string(audit.SlidesDownload)
			e.AuditReason = auditReasonHidden
			e.AuditTargetType = string(audit.TargetIdea)
			e.AuditTargetID = uuid.UUID(id)
		case e == nil:
			h.log().ErrorContext(r.Context(), "slides download failed", "ideaId", uuid.UUID(id).String(), "error", err)
		}
		h.Fail(w, r, err)
		return
	}
	defer func() { _ = d.Body.Close() }()

	i := d.Idea
	base, err := h.ExpectedFilename(i.Category, i.Org, i.LeaderName, i.LeaderEmpNo, i.Name)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", contentTypes[i.Slides.Ext])
	hd.Set("Content-Length", strconv.FormatInt(i.Slides.Size, 10))
	hd.Set("Content-Disposition", ContentDisposition(base+"."+i.Slides.Ext))
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	src := &readErrRecorder{r: d.Body}
	n, err := io.CopyN(w, src, i.Slides.Size)
	if err == nil {
		return
	}
	if src.err != nil || errors.Is(err, io.EOF) {
		// Headers are out, so no error body can follow. Abort the connection
		// (HTTP/1.1 close, HTTP/2 RST_STREAM): with Content-Length set the
		// client sees an incomplete body instead of a silently short file.
		h.log().ErrorContext(r.Context(), "slides download aborted: storage failed mid-stream",
			"ideaId", i.ID.String(), "sent", n, "size", i.Slides.Size, "error", err)
		panic(http.ErrAbortHandler)
	}
	// The client went away; nothing left to tell it.
	h.log().InfoContext(r.Context(), "slides download interrupted by client",
		"ideaId", i.ID.String(), "sent", n, "size", i.Slides.Size, "error", err)
}

func (h Handler) log() *slog.Logger {
	if h.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return h.Log
}

// readErrRecorder remembers the source's first non-EOF error, so a failed copy
// can be blamed on storage (abort) or on the client (just stop).
type readErrRecorder struct {
	r   io.Reader
	err error
}

func (x *readErrRecorder) Read(p []byte) (int, error) {
	n, err := x.r.Read(p)
	if err != nil && !errors.Is(err, io.EOF) && x.err == nil {
		x.err = err
	}
	return n, err
}

// ContentDisposition is the RFC 6266 attachment header for name: an ASCII
// fallback filename (non-ASCII, quotes, backslashes and controls become "_")
// and the exact UTF-8 name as filename* (RFC 8187 percent-encoding).
func ContentDisposition(name string) string {
	var fallback, ext strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			fallback.WriteByte('_')
		} else {
			fallback.WriteRune(r)
		}
	}
	const hex = "0123456789ABCDEF"
	for _, b := range []byte(name) {
		if isAttrChar(b) {
			ext.WriteByte(b)
		} else {
			ext.WriteByte('%')
			ext.WriteByte(hex[b>>4])
			ext.WriteByte(hex[b&0x0f])
		}
	}
	return `attachment; filename="` + fallback.String() + `"; filename*=UTF-8''` + ext.String()
}

// isAttrChar is RFC 8187 attr-char: ALPHA / DIGIT / "!#$&+-.^_`|~".
func isAttrChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", b) >= 0
}

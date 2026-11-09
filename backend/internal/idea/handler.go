package idea

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/audit"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/idea/internal/store"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/db"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

// auditReasonHidden is the one reason stored for a missing and a hidden Idea
// alike: the log must not tell them apart either (PRD 6.6).
const auditReasonHidden = "NOT_FOUND"

// View returns the Idea for GET /ideas/{id}. An Admin's successful view writes
// IDEA_VIEW in the same transaction as the read (fail-closed, PRD 6.11).
func (s *Service) View(ctx context.Context, id uuid.UUID, caller security.Principal) (Idea, error) {
	if !caller.IsAdmin() {
		return s.Get(ctx, id, caller)
	}
	var got Idea
	_, err := db.InTx(ctx, s.DB, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		if got, err = store.GetVisible(ctx, tx, id, caller.UserID, true); err != nil {
			return err
		}
		return s.Audit.Record(ctx, tx, audit.Event{
			Action: audit.IdeaView, Outcome: audit.Success,
			TargetType: audit.TargetIdea, TargetID: id,
		})
	})
	if errors.Is(err, store.ErrNotFound) {
		return Idea{}, web.NewError(web.CodeNotFound, "")
	}
	return got, err
}

// Handler serves the Idea endpoints.
type Handler struct {
	Service *Service
	// ExpectedFilename builds the expected slide filename without extension
	// (storage.ExpectedFilenameBase; injected because idea must not import
	// storage, D29).
	ExpectedFilename func(category, org, leaderName, leaderEmpNo, ideaName string) (string, error)
	Fail             func(w http.ResponseWriter, r *http.Request, err error)
	// Log receives download failures the response cannot carry (nil = discard).
	Log *slog.Logger
}

type person struct {
	EmpNo string `json:"empNo"`
	Name  string `json:"name"`
}

type member struct {
	EmpNo    string `json:"empNo"`
	Name     string `json:"name"`
	JoinedAt string `json:"joinedAt"`
}

type pending struct {
	EmpNo       string `json:"empNo"`
	Name        string `json:"name"`
	RequestedAt string `json:"requestedAt"`
}

type slides struct {
	Ext              string `json:"ext"`
	Size             int64  `json:"size"`
	SHA256           string `json:"sha256"`
	UpdatedAt        string `json:"updatedAt"`
	DownloadFilename string `json:"downloadFilename"`
}

// detail is IdeaDetail (docs/API/response-bodies.md). It is local so that
// timestamps use web.FormatTime (fixed milliseconds) instead of time.Time.
type detail struct {
	ID                   uuid.UUID  `json:"id"`
	Category             string     `json:"category"`
	Org                  string     `json:"org"`
	Direction            string     `json:"direction"`
	Name                 string     `json:"name"`
	Summary              string     `json:"summary"`
	Leader               person     `json:"leader"`
	Members              []member   `json:"members"`
	PendingRequests      *[]pending `json:"pendingRequests"`
	Slides               slides     `json:"slides"`
	ExpectedFilenameBase string     `json:"expectedFilenameBase"`
	CreatedAt            string     `json:"createdAt"`
	UpdatedAt            string     `json:"updatedAt"`
	Version              int64      `json:"version"`
}

// GetIdea serves GET /ideas/{id}.
func (h Handler) GetIdea(w http.ResponseWriter, r *http.Request, id api.IdeaId) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	got, err := h.Service.View(r.Context(), uuid.UUID(id), p)
	if err != nil {
		var e *web.Error
		if errors.As(err, &e) && e.Code == web.CodeNotFound {
			e.AuditAction = string(audit.IdeaView)
			e.AuditReason = auditReasonHidden
			e.AuditTargetType = string(audit.TargetIdea)
			e.AuditTargetID = uuid.UUID(id)
		}
		h.Fail(w, r, err)
		return
	}
	body, err := h.toDetail(got, p)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	web.WriteJSON(w, http.StatusOK, body)
}

func (h Handler) toDetail(i Idea, caller security.Principal) (detail, error) {
	base, err := h.ExpectedFilename(i.Category, i.Org, i.LeaderName, i.LeaderEmpNo, i.Name)
	if err != nil {
		return detail{}, err
	}
	d := detail{
		ID: i.ID, Category: i.Category, Org: i.Org, Direction: i.Direction, Name: i.Name, Summary: i.Summary,
		Leader: person{EmpNo: i.LeaderEmpNo, Name: i.LeaderName},
		// TODO(B5-01): approved Members and pending requests come from idea_member.
		Members: []member{},
		Slides: slides{Ext: i.Slides.Ext, Size: i.Slides.Size, SHA256: i.Slides.SHA256,
			UpdatedAt: web.FormatTime(i.SlidesUpdatedAt), DownloadFilename: base + "." + i.Slides.Ext},
		ExpectedFilenameBase: base,
		CreatedAt:            web.FormatTime(i.CreatedAt),
		UpdatedAt:            web.FormatTime(i.UpdatedAt),
		Version:              int64(i.Version),
	}
	if caller.IsAdmin() || caller.UserID == i.LeaderID {
		d.PendingRequests = &[]pending{}
	}
	return d, nil
}

// Package store holds the SQL of the idea module. Only the idea module may
// import it (Go's internal rule); no other module touches the idea table.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when no row exists or the viewer may not see it. The
// two cases are deliberately indistinguishable (PRD 6.6, Backend.md 8).
var ErrNotFound = errors.New("idea not found")

// Querier is satisfied by *sql.DB, *sql.Tx and *sql.Conn.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// NewIdea is a row to insert. Every value is already validated and cleaned;
// the table's CHECKs are the second line of defence (V005).
type NewIdea struct {
	ID        uuid.UUID
	LeaderID  uuid.UUID
	Category  string
	Org       string
	Direction string
	Name      string
	Summary   string
	Slides    Slides
}

// Slides is the slide metadata stored on the idea row (D22).
type Slides struct {
	ObjectKey string // slides/{ideaId}/{uuid}.{ext}
	Ext       string // pdf | pptx
	Size      int64
	SHA256    string // lower-case hex
}

// Idea is a row of idea joined with its Leader's public fields.
type Idea struct {
	ID              uuid.UUID
	Category        string
	Org             string
	Direction       string
	Name            string
	Summary         string
	LeaderID        uuid.UUID
	LeaderEmpNo     string
	LeaderName      string
	Slides          Slides
	SlidesUpdatedAt time.Time
	Version         uint32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InsertIdea inserts an Idea in the caller's transaction. A unique-key
// violation on uq_idea_leader_name is returned as the driver error; the
// service maps it with db.DuplicateKey to IDEA_DUPLICATE.
func InsertIdea(ctx context.Context, tx *sql.Tx, n NewIdea) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO idea (id, leader_id, category, org, direction, name, summary,
		                   slides_object_key, slides_ext, slides_size, slides_sha256)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID.String(), n.LeaderID.String(), n.Category, n.Org, n.Direction, n.Name, n.Summary,
		n.Slides.ObjectKey, n.Slides.Ext, n.Slides.Size, n.Slides.SHA256)
	if err != nil {
		return fmt.Errorf("insert idea: %w", err)
	}
	return nil
}

// VisibleWhere is the one visibility predicate (PRD 6.6: an Idea is visible to
// its Leader, its approved Members and Admins). Every read of an Idea, single
// or list, is built on it; no other code decides who may see an Idea.
// Arguments, in order: is_admin (0/1, CAST AS SIGNED in sqlc terms, rule R4),
// viewer user id. It assumes the idea table is aliased i.
//
// TODO(B5-01): add the approved-Member branch
//
//	OR EXISTS (SELECT 1 FROM idea_member m WHERE m.idea_id = i.id
//	           AND m.user_id = ? AND m.status = 'ACCEPTED')
//
// when the idea_member table exists. Until then Members cannot exist.
const VisibleWhere = `(? = 1 OR i.leader_id = ?)`

const selectIdea = `
SELECT i.id, i.category, i.org, i.direction, i.name, i.summary,
       i.leader_id, u.emp_no, u.name,
       i.slides_object_key, i.slides_ext, i.slides_size, i.slides_sha256, i.slides_updated_at,
       i.version, i.created_at, i.updated_at
  FROM idea i
  JOIN app_user u ON u.id = i.leader_id`

// GetVisible is the single visibility query for one Idea. A missing Idea and
// an Idea the viewer may not see both return ErrNotFound.
func GetVisible(ctx context.Context, q Querier, id, viewer uuid.UUID, isAdmin bool) (Idea, error) {
	var (
		out           Idea
		sid, lid, vid string
		admin         int
	)
	admin = 0
	if isAdmin {
		admin = 1
	}
	vid = viewer.String()
	err := q.QueryRowContext(ctx, selectIdea+` WHERE i.id = ? AND `+VisibleWhere,
		id.String(), admin, vid).Scan(
		&sid, &out.Category, &out.Org, &out.Direction, &out.Name, &out.Summary,
		&lid, &out.LeaderEmpNo, &out.LeaderName,
		&out.Slides.ObjectKey, &out.Slides.Ext, &out.Slides.Size, &out.Slides.SHA256, &out.SlidesUpdatedAt,
		&out.Version, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Idea{}, ErrNotFound
	}
	if err != nil {
		return Idea{}, fmt.Errorf("select idea: %w", err)
	}
	if out.ID, err = uuid.Parse(sid); err != nil {
		return Idea{}, fmt.Errorf("idea id: %w", err)
	}
	if out.LeaderID, err = uuid.Parse(lid); err != nil {
		return Idea{}, fmt.Errorf("idea leader id: %w", err)
	}
	return out, nil
}

// ErrNoLeader is returned by LeaderName when the user row is gone, which a
// valid session rules out; the caller treats it as an internal error.
var ErrNoLeader = errors.New("leader account not found")

// LeaderName returns the account name of user id: the expected filename takes
// the Leader's name from the account, never from the form (PRD 6.6).
func LeaderName(ctx context.Context, q Querier, id uuid.UUID) (string, error) {
	var name string
	err := q.QueryRowContext(ctx, `SELECT name FROM app_user WHERE id = ?`, id.String()).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoLeader
	}
	if err != nil {
		return "", fmt.Errorf("select leader name: %w", err)
	}
	return name, nil
}

// NameTaken is the read-only duplicate pre-check of create: it saves an upload
// in the common case, while uq_idea_leader_name stays authoritative.
func NameTaken(ctx context.Context, q Querier, leader uuid.UUID, name string) (bool, error) {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM idea WHERE leader_id = ? AND name = ?`,
		leader.String(), name).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select idea name: %w", err)
	}
	return true, nil
}

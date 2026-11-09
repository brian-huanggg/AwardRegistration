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
// Arguments, in order: is_admin (0/1), viewer user id, viewer user id
// (see VisibleArgs). A PENDING applicant does not see the Idea (PRD 6.7).
// It assumes the idea table is aliased i.
const VisibleWhere = `(? = 1 OR i.leader_id = ?
	OR EXISTS (SELECT 1 FROM idea_member m WHERE m.idea_id = i.id
	           AND m.user_id = ? AND m.status = 'ACCEPTED'))`

// VisibleArgs returns the arguments of VisibleWhere, in order.
func VisibleArgs(viewer uuid.UUID, isAdmin bool) []any {
	admin := 0
	if isAdmin {
		admin = 1
	}
	v := viewer.String()
	return []any{admin, v, v}
}

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
		out      Idea
		sid, lid string
	)
	err := q.QueryRowContext(ctx, selectIdea+` WHERE i.id = ? AND `+VisibleWhere,
		append([]any{id.String()}, VisibleArgs(viewer, isAdmin)...)...).Scan(
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

// FindByLeader resolves an Idea by its Leader's employee ID and exact name, the
// address of a join request (PRD 6.7). It is not a visibility query: the
// applicant may not see the Idea, so the caller must expose only what a
// JoinRequest carries. No match is ErrNotFound.
func FindByLeader(ctx context.Context, q Querier, leaderEmpNo, name string) (Idea, error) {
	var id string
	err := q.QueryRowContext(ctx,
		`SELECT i.id FROM idea i JOIN app_user u ON u.id = i.leader_id WHERE u.emp_no = ? AND i.name = ?`,
		leaderEmpNo, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Idea{}, ErrNotFound
	}
	if err != nil {
		return Idea{}, fmt.Errorf("select idea by leader: %w", err)
	}
	pid, err := uuid.Parse(id)
	if err != nil {
		return Idea{}, fmt.Errorf("idea id: %w", err)
	}
	return GetVisible(ctx, q, pid, uuid.Nil, true)
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

// Text is the five editable fields of an Idea (PRD 6.6).
type Text struct {
	Category, Org, Direction, Name, Summary string
}

// LockText locks the idea row and returns its leader and current text fields.
// It is the first statement of an edit transaction, so concurrent edits and a
// concurrent delete serialize here. A row deleted in the meantime is
// ErrNotFound (the uniform 404). Only the idea row is locked, never the Leader's
// account row, which is why this is not built on the joined selectIdea.
func LockText(ctx context.Context, tx *sql.Tx, id uuid.UUID) (leader uuid.UUID, cur Text, err error) {
	var lid string
	err = tx.QueryRowContext(ctx,
		`SELECT leader_id, category, org, direction, name, summary FROM idea WHERE id = ? FOR UPDATE`,
		id.String()).Scan(&lid, &cur.Category, &cur.Org, &cur.Direction, &cur.Name, &cur.Summary)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, Text{}, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, Text{}, fmt.Errorf("lock idea: %w", err)
	}
	if leader, err = uuid.Parse(lid); err != nil {
		return uuid.Nil, Text{}, fmt.Errorf("idea leader id: %w", err)
	}
	return leader, cur, nil
}

// UpdateText replaces the five text fields of a locked idea row and adds 1 to
// version (V005). A unique-key violation on uq_idea_leader_name is returned as
// the driver error; the service maps it with db.DuplicateKey.
func UpdateText(ctx context.Context, tx *sql.Tx, id uuid.UUID, t Text) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE idea SET category = ?, org = ?, direction = ?, name = ?, summary = ?, version = version + 1
		  WHERE id = ?`,
		t.Category, t.Org, t.Direction, t.Name, t.Summary, id.String())
	if err != nil {
		return fmt.Errorf("update idea: %w", err)
	}
	return nil
}

// LockSlides locks the idea row and returns its leader and current slide
// pointer. It is the first statement of a replace transaction, so two
// concurrent replacements cannot read the same old key and orphan one object
// (Sequence.md §4). A row deleted in the meantime is ErrNotFound.
func LockSlides(ctx context.Context, tx *sql.Tx, id uuid.UUID) (leader uuid.UUID, cur Slides, err error) {
	var lid string
	err = tx.QueryRowContext(ctx,
		`SELECT leader_id, slides_object_key, slides_ext, slides_size, slides_sha256 FROM idea WHERE id = ? FOR UPDATE`,
		id.String()).Scan(&lid, &cur.ObjectKey, &cur.Ext, &cur.Size, &cur.SHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, Slides{}, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, Slides{}, fmt.Errorf("lock idea slides: %w", err)
	}
	if leader, err = uuid.Parse(lid); err != nil {
		return uuid.Nil, Slides{}, fmt.Errorf("idea leader id: %w", err)
	}
	return leader, cur, nil
}

// SwitchSlides points a locked idea row at a new object, stamps
// slides_updated_at with the database clock and adds 1 to version.
func SwitchSlides(ctx context.Context, tx *sql.Tx, id uuid.UUID, s Slides) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE idea SET slides_object_key = ?, slides_ext = ?, slides_size = ?, slides_sha256 = ?,
		                 slides_updated_at = CURRENT_TIMESTAMP(6), version = version + 1
		  WHERE id = ?`,
		s.ObjectKey, s.Ext, s.Size, s.SHA256, id.String())
	if err != nil {
		return fmt.Errorf("switch idea slides: %w", err)
	}
	return nil
}

// DeleteIdea removes a locked idea row. Dependent member rows go with it by
// foreign-key cascade (Sequence.md §5).
func DeleteIdea(ctx context.Context, tx *sql.Tx, id uuid.UUID) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM idea WHERE id = ?`, id.String()); err != nil {
		return fmt.Errorf("delete idea: %w", err)
	}
	return nil
}

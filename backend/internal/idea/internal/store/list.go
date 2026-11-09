package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ListFilter is the validated filter set of GET /ideas (PRD 6.6). A nil or
// empty field means "no filter". CreatedFrom is inclusive, CreatedTo exclusive.
type ListFilter struct {
	Org, Category, Direction string
	LeaderEmpNo              string
	CreatedFrom, CreatedTo   *time.Time
}

// Summary is one row of the list (IdeaSummary: no summary text).
type Summary struct {
	ID              uuid.UUID
	Name            string
	Category        string
	Org             string
	Direction       string
	LeaderID        uuid.UUID
	LeaderEmpNo     string
	LeaderName      string
	MemberCount     int
	CreatedAt       time.Time
	SlidesUpdatedAt time.Time
}

// dbTime is the DATETIME(6) literal of an instant; the database stores UTC.
func dbTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05.000000") }

// ListVisible returns one page of the Ideas the viewer may see, newest first
// (id is the final tie-breaker), and the total number of matches. It is built
// on VisibleWhere like every other read. A page beyond the last is empty.
func ListVisible(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, viewer uuid.UUID, isAdmin bool, f ListFilter, limit, offset int) ([]Summary, int, error) {
	admin := 0
	if isAdmin {
		admin = 1
	}
	where := []string{VisibleWhere}
	args := []any{admin, viewer.String()}
	add := func(cond string, v any) {
		where = append(where, cond)
		args = append(args, v)
	}
	if f.Org != "" {
		add("i.org = ?", f.Org)
	}
	if f.Category != "" {
		add("i.category = ?", f.Category)
	}
	if f.Direction != "" {
		add("i.direction = ?", f.Direction)
	}
	if f.LeaderEmpNo != "" {
		add("u.emp_no = ?", f.LeaderEmpNo)
	}
	if f.CreatedFrom != nil {
		add("i.created_at >= ?", dbTime(*f.CreatedFrom))
	}
	if f.CreatedTo != nil {
		add("i.created_at < ?", dbTime(*f.CreatedTo))
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM idea i JOIN app_user u ON u.id = i.leader_id WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ideas: %w", err)
	}

	// TODO(B5-01): memberCount counts ACCEPTED idea_member rows (Leader excluded).
	rows, err := q.QueryContext(ctx,
		`SELECT i.id, i.name, i.category, i.org, i.direction, i.leader_id, u.emp_no, u.name,
		        0, i.created_at, i.slides_updated_at
		   FROM idea i JOIN app_user u ON u.id = i.leader_id
		  WHERE `+cond+`
		  ORDER BY i.created_at DESC, i.id DESC
		  LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list ideas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Summary{}
	for rows.Next() {
		var s Summary
		var id, lid string
		if err := rows.Scan(&id, &s.Name, &s.Category, &s.Org, &s.Direction, &lid, &s.LeaderEmpNo,
			&s.LeaderName, &s.MemberCount, &s.CreatedAt, &s.SlidesUpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan idea: %w", err)
		}
		if s.ID, err = uuid.Parse(id); err != nil {
			return nil, 0, fmt.Errorf("idea id: %w", err)
		}
		if s.LeaderID, err = uuid.Parse(lid); err != nil {
			return nil, 0, fmt.Errorf("idea leader id: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list ideas: %w", err)
	}
	return out, total, nil
}

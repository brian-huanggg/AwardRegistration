package idea

import (
	"math"
	"net/http"
	"regexp"
	"slices"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/security"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxPage         = math.MaxInt32 // keeps (page-1)*size far from int overflow
)

// Closed value sets (PRD 6.6); the idea table's CHECKs list the same values.
var (
	categories = []string{"COOL_IDEA", "OUTSTANDING_IMPACT"}
	orgs       = []string{"AAID", "BSID", "ICSD", "TSID", "PLED", "DEID"}
	directions = []string{"GREEN_MANUFACTURING", "RESPONSIBLE_SUPPLY_CHAIN", "INCLUSIVE_WORKPLACE",
		"TALENT_DEVELOPMENT", "CARING_FOR_DISADVANTAGED"}
	sorts       = []string{"createdAt", "name"}
	order       = []string{"asc", "desc"}
	empNoFormat = regexp.MustCompile(`^[0-9]{6}$`)
)

type summary struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	Org             string `json:"org"`
	Direction       string `json:"direction"`
	Leader          person `json:"leader"`
	MemberCount     int    `json:"memberCount"`
	MyRelation      any    `json:"myRelation"`
	CreatedAt       string `json:"createdAt"`
	SlidesUpdatedAt string `json:"slidesUpdatedAt"`
}

type page struct {
	Items []summary `json:"items"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
	Total int       `json:"total"`
}

// listQuery is the validated query of GET /ideas.
type listQuery struct {
	page, size int
	filter     ListFilter
}

// parseList validates every parameter and reports all failures at once, in
// parameter order, each parameter with its first error (PRD 6.2, 6.6). No
// invalid value is ignored.
func parseList(p api.ListIdeasParams) (listQuery, []web.FieldError) {
	q := listQuery{page: 1, size: defaultPageSize}
	var errs []web.FieldError
	bad := func(field string, code web.ErrorCode) {
		errs = append(errs, web.FieldError{Field: field, Code: code})
	}
	if p.Page != nil {
		if *p.Page < 1 || *p.Page > maxPage {
			bad("page", web.CodeOutOfRange)
		} else {
			q.page = *p.Page
		}
	}
	if p.Size != nil {
		if *p.Size < 1 || *p.Size > maxPageSize {
			bad("size", web.CodeOutOfRange)
		} else {
			q.size = *p.Size
		}
	}
	enum := func(field string, v *string, allowed []string, dst *string) {
		if v == nil {
			return
		}
		if !slices.Contains(allowed, *v) {
			bad(field, web.CodeInvalidEnum)
			return
		}
		*dst = *v
	}
	enum("org", p.Org, orgs, &q.filter.Org)
	enum("category", p.Category, categories, &q.filter.Category)
	enum("direction", p.Direction, directions, &q.filter.Direction)
	if p.LeaderEmpNo != nil {
		if !empNoFormat.MatchString(*p.LeaderEmpNo) {
			bad("leaderEmpNo", web.CodeInvalidFormat)
		} else {
			q.filter.LeaderEmpNo = *p.LeaderEmpNo
		}
	}
	instant := func(field string, v *string) *time.Time {
		if v == nil {
			return nil
		}
		// RFC 3339 requires the offset or Z, so a bare date or local time fails.
		t, err := time.Parse(time.RFC3339, *v)
		if err != nil {
			bad(field, web.CodeInvalidFormat)
			return nil
		}
		return &t
	}
	q.filter.CreatedFrom = instant("createdFrom", p.CreatedFrom)
	q.filter.CreatedTo = instant("createdTo", p.CreatedTo)
	if p.Q != nil {
		q.filter.Q = *p.Q
	}
	sort := "createdAt"
	enum("sort", p.Sort, sorts, &sort)
	q.filter.SortName = sort == "name"
	dir := "desc"
	if q.filter.SortName {
		dir = "asc"
	}
	enum("order", p.Order, order, &dir)
	q.filter.Desc = dir == "desc"
	return q, errs
}

// ListIdeas serves GET /ideas.
func (h Handler) ListIdeas(w http.ResponseWriter, r *http.Request, params api.ListIdeasParams) {
	p, ok := security.PrincipalFrom(r.Context())
	if !ok {
		h.Fail(w, r, web.NewError(web.CodeUnauthenticated, "authentication required"))
		return
	}
	q, errs := parseList(params)
	if len(errs) > 0 {
		e := web.NewError(web.CodeValidationFailed, "invalid query parameter")
		e.Fields = errs
		h.Fail(w, r, e)
		return
	}
	rows, total, err := h.Service.List(r.Context(), p, q.filter, q.size, (q.page-1)*q.size)
	if err != nil {
		h.Fail(w, r, err)
		return
	}
	out := page{Items: make([]summary, 0, len(rows)), Page: q.page, Size: q.size, Total: total}
	for _, s := range rows {
		var rel any // null for Admin
		if !p.IsAdmin() {
			rel = "MEMBER"
			if s.LeaderID == p.UserID {
				rel = "LEADER"
			}
		}
		out.Items = append(out.Items, summary{
			ID: s.ID.String(), Name: s.Name, Category: s.Category, Org: s.Org, Direction: s.Direction,
			Leader:      person{EmpNo: s.LeaderEmpNo, Name: s.LeaderName},
			MemberCount: s.MemberCount, MyRelation: rel,
			CreatedAt: web.FormatTime(s.CreatedAt), SlidesUpdatedAt: web.FormatTime(s.SlidesUpdatedAt),
		})
	}
	web.WriteJSON(w, http.StatusOK, out)
}

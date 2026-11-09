package idea

import (
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/api"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func ptr[T any](v T) *T { return &v }

func TestParseListDefaults(t *testing.T) {
	q, errs := parseList(api.ListIdeasParams{})
	if len(errs) != 0 || q.page != 1 || q.size != 20 {
		t.Fatalf("q=%+v errs=%v", q, errs)
	}
}

func TestParseListErrors(t *testing.T) {
	tests := []struct {
		name string
		p    api.ListIdeasParams
		want []web.FieldError
	}{
		{"page 0", api.ListIdeasParams{Page: ptr(0)}, []web.FieldError{{Field: "page", Code: web.CodeOutOfRange}}},
		{"size 101", api.ListIdeasParams{Size: ptr(101)}, []web.FieldError{{Field: "size", Code: web.CodeOutOfRange}}},
		{"size 0", api.ListIdeasParams{Size: ptr(0)}, []web.FieldError{{Field: "size", Code: web.CodeOutOfRange}}},
		{"bad org", api.ListIdeasParams{Org: ptr("XXXX")}, []web.FieldError{{Field: "org", Code: web.CodeInvalidEnum}}},
		{"empty org not ignored", api.ListIdeasParams{Org: ptr("")}, []web.FieldError{{Field: "org", Code: web.CodeInvalidEnum}}},
		{"lower-case category", api.ListIdeasParams{Category: ptr("cool_idea")}, []web.FieldError{{Field: "category", Code: web.CodeInvalidEnum}}},
		{"bad direction", api.ListIdeasParams{Direction: ptr("X")}, []web.FieldError{{Field: "direction", Code: web.CodeInvalidEnum}}},
		{"short emp no", api.ListIdeasParams{LeaderEmpNo: ptr("12345")}, []web.FieldError{{Field: "leaderEmpNo", Code: web.CodeInvalidFormat}}},
		{"date only", api.ListIdeasParams{CreatedFrom: ptr("2026-01-01")}, []web.FieldError{{Field: "createdFrom", Code: web.CodeInvalidFormat}}},
		{"no offset", api.ListIdeasParams{CreatedTo: ptr("2026-01-01T00:00:00")}, []web.FieldError{{Field: "createdTo", Code: web.CodeInvalidFormat}}},
		{"all, in parameter order", api.ListIdeasParams{Size: ptr(500), Page: ptr(-1), Org: ptr("x"), CreatedTo: ptr("nope")},
			[]web.FieldError{{Field: "page", Code: web.CodeOutOfRange}, {Field: "size", Code: web.CodeOutOfRange},
				{Field: "org", Code: web.CodeInvalidEnum}, {Field: "createdTo", Code: web.CodeInvalidFormat}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, got := parseList(tc.p)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] got %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseListValid(t *testing.T) {
	q, errs := parseList(api.ListIdeasParams{
		Page: ptr(3), Size: ptr(100), Org: ptr("BSID"), Category: ptr("COOL_IDEA"), Direction: ptr("TALENT_DEVELOPMENT"),
		LeaderEmpNo: ptr("000123"), CreatedFrom: ptr("2026-01-01T08:00:00+08:00"), CreatedTo: ptr("2026-02-01T00:00:00.5Z"),
	})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if q.page != 3 || q.size != 100 || q.filter.Org != "BSID" || q.filter.LeaderEmpNo != "000123" {
		t.Errorf("q = %+v", q)
	}
	if !q.filter.CreatedFrom.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("from = %v", q.filter.CreatedFrom)
	}
}

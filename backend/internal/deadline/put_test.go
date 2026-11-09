package deadline_test

import (
	"errors"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/deadline"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func TestParseDeadline(t *testing.T) {
	ok := []struct{ in, want string }{
		{"2030-05-01T10:00:00Z", "2030-05-01T10:00:00Z"},
		{"2030-05-01T18:00:00+08:00", "2030-05-01T10:00:00Z"},
		{"2030-05-01T10:00:00.987Z", "2030-05-01T10:00:00Z"}, // fraction truncated
		{"2000-01-01T00:00:00Z", "2000-01-01T00:00:00Z"},
		{"2099-12-31T23:59:59Z", "2099-12-31T23:59:59Z"},
		{"2020-01-01T00:00:00Z", "2020-01-01T00:00:00Z"}, // the past is allowed
	}
	for _, tc := range ok {
		got, err := deadline.ParseDeadline(tc.in)
		if err != nil || got.Format(time.RFC3339) != tc.want {
			t.Errorf("%q = %v, %v; want %s", tc.in, got, err, tc.want)
		}
	}
	bad := []struct {
		in   string
		code web.ErrorCode
	}{
		{"2030-05-01T10:00:00", web.CodeInvalidFormat}, // no offset
		{"2030-05-01", web.CodeInvalidFormat},
		{"", web.CodeInvalidFormat},
		{"tomorrow", web.CodeInvalidFormat},
		{"1999-12-31T23:59:59Z", web.CodeOutOfRange},
		{"2100-01-01T00:00:00Z", web.CodeOutOfRange},
	}
	for _, tc := range bad {
		var e *web.Error
		_, err := deadline.ParseDeadline(tc.in)
		if !errors.As(err, &e) || e.Status != 422 || len(e.Fields) != 1 ||
			e.Fields[0].Field != "deadline" || e.Fields[0].Code != tc.code {
			t.Errorf("%q: %v", tc.in, err)
		}
	}
	// The year is judged on the UTC instant: this is 2099-12-31T16:00Z.
	if _, err := deadline.ParseDeadline("2100-01-01T00:00:00+08:00"); err != nil {
		t.Errorf("UTC-in-range value rejected: %v", err)
	}
}

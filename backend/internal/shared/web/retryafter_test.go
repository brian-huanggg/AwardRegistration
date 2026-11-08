package web_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func TestRetryAfterWholeSecondsRoundedUpMinimumOne(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, ""},
		{time.Millisecond, "1"},
		{time.Second, "1"},
		{1001 * time.Millisecond, "2"},
		{15 * time.Minute, "900"},
	}
	for _, tc := range tests {
		e := web.NewError(web.CodeLoginThrottled, "")
		e.RetryAfter = tc.d
		rec := httptest.NewRecorder()
		web.WriteProblem(rec, httptest.NewRequest("POST", "/", nil), e)
		if got := rec.Header().Get("Retry-After"); got != tc.want {
			t.Errorf("%v: Retry-After = %q, want %q", tc.d, got, tc.want)
		}
	}
}

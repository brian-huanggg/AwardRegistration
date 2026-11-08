package web

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestFormatTimeTruncatesNotRounds(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*3600)
	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{"exact", time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC), "2026-11-08T12:00:00.000Z"},
		{"999.9 ms stays 999", time.Date(2026, 11, 8, 12, 0, 0, 999_900_000, time.UTC), "2026-11-08T12:00:00.999Z"},
		{"microseconds dropped", time.Date(2026, 11, 8, 12, 0, 0, 123_999_000, time.UTC), "2026-11-08T12:00:00.123Z"},
		{"other zone converted to UTC", time.Date(2026, 11, 8, 20, 0, 0, 5_000_000, zone), "2026-11-08T12:00:00.005Z"},
	}
	for _, tc := range tests {
		if got := FormatTime(tc.in); got != tc.want {
			t.Errorf("%s: FormatTime = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, 200, map[string]int{"a": 1})
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json" ||
		rec.Header().Get("Cache-Control") != "no-store" || rec.Body.String() != "{\"a\":1}\n" {
		t.Fatalf("unexpected response: %d %v %q", rec.Code, rec.Header(), rec.Body)
	}
}

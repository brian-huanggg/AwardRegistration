package web

import (
	"encoding/json"
	"net/http"
	"time"
)

// TimeLayout is the one wire format for timestamps: UTC, fixed three fraction digits.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// FormatTime renders t as UTC truncated (never rounded) to milliseconds
// (D55, Backend.md §10): 12:00:00.9996 is written 12:00:00.999, so a client
// never sees a time later than the database's.
func FormatTime(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format(TimeLayout)
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

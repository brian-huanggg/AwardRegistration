package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"plain", "E12345", 64, "E12345"},
		{"control chars and newlines dropped", "a\x00b\r\nc\td", 64, "abcd"},
		{"line separators dropped", "a b c", 64, "abc"},
		{"invalid utf-8 dropped", "ab\xffcd", 64, "abcd"},
		{"trimmed", "  x  ", 64, "x"},
		{"cut by code point, not byte", strings.Repeat("創", 10), 4, "創創創創"},
		{"emoji kept whole", "😀😀😀", 2, "😀😀"},
		{"empty", "", 8, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clean(tc.in, tc.max); got != tc.want {
				t.Errorf("clean(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // "" = NULL
		wantErr bool
	}{
		{"empty is NULL", ``, "", false},
		{"blank is NULL", "  \n", "", false},
		{"null is NULL", `null`, "", false},
		{"compacted", "{ \"a\": 1 }", `{"a":1}`, false},
		{"big number kept exactly", `{"n":12345678901234567890}`, `{"n":12345678901234567890}`, false},
		{"password redacted", `{"password":"hunter2","name":"x"}`, `{"name":"x","password":"[REDACTED]"}`, false},
		{"key spelling ignored", `{"Session_ID":"s","x-csrf-token":"c","Set-Cookie":"k"}`,
			`{"Session_ID":"[REDACTED]","Set-Cookie":"[REDACTED]","x-csrf-token":"[REDACTED]"}`, false},
		{"nested and arrays", `{"a":[{"token":"t","ok":true}]}`, `{"a":[{"ok":true,"token":"[REDACTED]"}]}`, false},
		{"file content redacted", `{"filename":"a.pptx","content":"AAAA"}`, `{"content":"[REDACTED]","filename":"a.pptx"}`, false},
		{"invalid", `{"a":`, "", true},
		{"trailing data", `{"a":1} {"b":2}`, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cleanJSON(json.RawMessage(tc.in))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
			if tc.want == "" && got != nil {
				t.Errorf("want nil slice (NULL), got %q", got)
			}
		})
	}
}

func TestCleanJSONOversizeIsMarker(t *testing.T) {
	big := `{"text":"` + strings.Repeat("x", maxJSONBytes) + `"}`
	got, err := cleanJSON(json.RawMessage(big))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("marker is not valid JSON: %v", err)
	}
	if m["truncated"] != true || len(got) > 100 {
		t.Errorf("marker = %s", got)
	}
}

func TestInsertRejectsBadEventsBeforeTouchingTheDatabase(t *testing.T) {
	ok := Event{Action: AuthLogin, Outcome: Success}
	tests := []struct {
		name string
		mod  func(*Event)
		ctx  context.Context
	}{
		{"unknown action", func(e *Event) { e.Action = "NOPE" }, nil},
		{"empty action", func(e *Event) { e.Action = "" }, nil},
		{"unknown outcome", func(e *Event) { e.Outcome = "MAYBE" }, nil},
		{"unknown target type", func(e *Event) { e.TargetType = "TEAM" }, nil},
		{"target id without type", func(e *Event) { e.TargetID = uuid.New() }, nil},
		{"unknown actor role", func(e *Event) { e.ActorRole = "ROOT" }, nil},
		{"bad before", func(e *Event) { e.Before = json.RawMessage(`{`) }, nil},
		{"bad after", func(e *Event) { e.After = json.RawMessage(`[`) }, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := ok
			tc.mod(&e)
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			// A nil execer would panic if validation did not stop first.
			if err := insert(ctx, nil, e); err == nil {
				t.Error("want error")
			}
		})
	}
}

func TestRecordNeedsTransaction(t *testing.T) {
	var tx *sql.Tx
	if err := New(nil, nil).Record(context.Background(), tx, Event{Action: AuthLogin, Outcome: Success}); err == nil {
		t.Error("want error for nil tx")
	}
}

func TestRecordRejectedRefusesSuccess(t *testing.T) {
	s := New(nil, nil) // a nil DB would panic if it got that far
	s.RecordRejected(context.Background(), Event{Action: AuthLogin, Outcome: Success})
	if s.RejectedFailures() != 1 {
		t.Errorf("failures = %d, want 1", s.RejectedFailures())
	}
}

// Package binding has one test per row of the JSON binding rules in
// docs/Architecture/Backend.md §3 (D46, ADR O1).
package binding

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

type inner struct {
	Kind *string `json:"kind"`
}

type dto struct {
	EmpNo *string `json:"empNo"`
	Count *int    `json:"count"`
	Note  *string `json:"note,omitempty"`
	In    *inner  `json:"in,omitempty"`
}

func decode(t *testing.T, contentType, body string) (dto, error) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	var d dto
	return d, web.DecodeJSON(req, &d)
}

func wantCode(t *testing.T, err error, code web.ErrorCode) {
	t.Helper()
	var e *web.Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestBindingRows(t *testing.T) {
	const ct = "application/json"
	cases := []struct {
		name, ct, body string
		code           web.ErrorCode // "" = success
	}{
		{"valid", ct, `{"empNo":"89290","count":2}`, ""},
		{"content type with charset", "application/json; charset=utf-8", `{"empNo":"1","count":1}`, ""},
		{"missing content type → 415", "", `{}`, web.CodeUnsupportedMedia},
		{"wrong content type → 415", "text/plain", `{}`, web.CodeUnsupportedMedia},
		{"duplicate top-level key", ct, `{"empNo":"1","empNo":"2","count":1}`, web.CodeMalformedRequest},
		{"duplicate nested key", ct, `{"empNo":"1","count":1,"in":{"kind":"a","kind":"b"}}`, web.CodeMalformedRequest},
		{"duplicate key differing only by case is not a duplicate", ct, `{"empNo":"1","EMPNO":"2","count":1}`, ""},
		{"trailing data", ct, `{"empNo":"1","count":1} {}`, web.CodeMalformedRequest},
		{"trailing garbage", ct, `{"empNo":"1","count":1}x`, web.CodeMalformedRequest},
		{"broken JSON", ct, `{"empNo":`, web.CodeMalformedRequest},
		{"empty body", ct, ``, web.CodeMalformedRequest},
		{"top level not an object", ct, `[1]`, web.CodeMalformedRequest},
		{"top level null", ct, `null`, web.CodeMalformedRequest},
		{"absent required field", ct, `{"empNo":"1"}`, web.CodeMalformedRequest},
		{"null required field", ct, `{"empNo":null,"count":1}`, web.CodeMalformedRequest},
		{"wrong type: number for string", ct, `{"empNo":89290,"count":1}`, web.CodeMalformedRequest},
		{"wrong type: string for number", ct, `{"empNo":"1","count":"1"}`, web.CodeMalformedRequest},
		{"wrong type: fractional for int", ct, `{"empNo":"1","count":1.5}`, web.CodeMalformedRequest},
		{"wrong type in nested object", ct, `{"empNo":"1","count":1,"in":{"kind":7}}`, web.CodeMalformedRequest},
		{"missing field in nested object", ct, `{"empNo":"1","count":1,"in":{}}`, web.CodeMalformedRequest},
		{"unknown fields ignored", ct, `{"empNo":"1","count":1,"extra":{"a":1}}`, ""},
		{"optional absent", ct, `{"empNo":"1","count":1}`, ""},
		{"optional null treated as absent", ct, `{"empNo":"1","count":1,"note":null}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := decode(t, c.ct, c.body)
			if c.code == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			wantCode(t, err, c.code)
		})
	}
}

func TestNamesAreCaseSensitive(t *testing.T) {
	_, err := decode(t, "application/json", `{"EMPNO":"1","count":1}`)
	wantCode(t, err, web.CodeMalformedRequest) // wrong-case name is "unknown", so empNo is absent

	d, err := decode(t, "application/json", `{"empNo":"1","count":1,"COUNT":9,"Note":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if *d.Count != 1 || d.Note != nil {
		t.Fatalf("wrong-case keys leaked into fields: count=%d note=%v", *d.Count, d.Note)
	}
}

func TestValuesDecoded(t *testing.T) {
	d, err := decode(t, "application/json", `{"empNo":"89290","count":3,"note":"n","in":{"kind":"k"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if *d.EmpNo != "89290" || *d.Count != 3 || *d.Note != "n" || *d.In.Kind != "k" {
		t.Fatalf("bad decode: %+v", d)
	}
}

func TestDeepNestingRejected(t *testing.T) {
	body := `{"empNo":"1","count":1,"x":` + strings.Repeat("[", 100) + strings.Repeat("]", 100) + `}`
	_, err := decode(t, "application/json", body)
	wantCode(t, err, web.CodeMalformedRequest)
}

func TestOversizedBody413(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"empNo":"`+strings.Repeat("a", 100)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 16)
	var d dto
	wantCode(t, web.DecodeJSON(req, &d), web.CodeRequestTooLarge)
}

func TestMalformedUsesCatalogueStatus(t *testing.T) {
	_, err := decode(t, "application/json", `{`)
	var e *web.Error
	if !errors.As(err, &e) || e.Status != http.StatusBadRequest {
		t.Fatalf("want 400, got %v", err)
	}
}

package web_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func TestCatalogueEqualsJSON(t *testing.T) {
	raw, err := os.ReadFile("../../../../testdata/error-codes.json")
	if err != nil {
		t.Fatal(err)
	}
	var file []struct {
		Code   string `json:"code"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, e := range file {
		got[e.Code] = e.Status
	}
	if len(got) != len(file) {
		t.Fatal("duplicate code in testdata/error-codes.json")
	}
	want := map[string]int{}
	for c, s := range web.Catalogue {
		want[string(c)] = s
	}
	var diff []string
	for c, s := range want {
		if g, ok := got[c]; !ok || g != s {
			diff = append(diff, c)
		}
	}
	for c := range got {
		if _, ok := want[c]; !ok {
			diff = append(diff, c)
		}
	}
	sort.Strings(diff)
	if len(diff) > 0 {
		t.Fatalf("catalogue and testdata/error-codes.json differ: %v", diff)
	}
}

func handler(log *slog.Logger, h http.HandlerFunc) http.Handler {
	return web.RequestID(web.AccessLog(log)(web.Recover(log)(h)))
}

func TestRequestID(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	h := handler(log, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(web.RequestIDFrom(r.Context())))
	})
	valid := uuid.NewString()
	for name, tc := range map[string]struct {
		in   string
		same bool
	}{"valid reused": {valid, true}, "non-uuid replaced": {"abc", false}, "empty replaced": {"", false},
		"braced uuid replaced": {"{" + valid + "}", false}} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.in != "" {
				req.Header.Set(web.RequestIDHeader, tc.in)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			got := rec.Header().Get(web.RequestIDHeader)
			if _, err := uuid.Parse(got); err != nil {
				t.Fatalf("response id %q not a uuid", got)
			}
			if (got == tc.in) != tc.same {
				t.Fatalf("got %q, in %q, same=%v", got, tc.in, tc.same)
			}
		})
	}
}

func TestPanicBecomes500WithoutLeak(t *testing.T) {
	var buf bytes.Buffer
	log := web.NewLogger(&buf, slog.LevelInfo)
	h := handler(log, func(http.ResponseWriter, *http.Request) { panic("secret-sql SELECT *") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != 500 || rec.Header().Get("Content-Type") != web.ProblemContentType {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if strings.Contains(rec.Body.String(), "secret-sql") {
		t.Fatal("panic value leaked to client")
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p["code"] != "INTERNAL_ERROR" || p["requestId"] != rec.Header().Get(web.RequestIDHeader) {
		t.Fatalf("body %v", p)
	}
}

func TestEveryLogLineHasRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := web.NewLogger(&buf, slog.LevelInfo)
	h := handler(log, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/boom" {
			panic("x")
		}
	})
	for _, p := range []string{"/ok", "/boom"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, p+"?token=s3cret", nil))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("want >=3 log lines, got %d", len(lines))
	}
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		if id, _ := m["requestId"].(string); id == "" {
			t.Fatalf("line without requestId: %s", l)
		}
		if m["@timestamp"] == nil || m["log.level"] == nil {
			t.Fatalf("not ECS-shaped: %s", l)
		}
	}
	if strings.Contains(buf.String(), "s3cret") {
		t.Fatal("query string logged")
	}
}

func TestWriteErrorWrapped(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	web.WriteError(rec, req, errors.Join(errors.New("x"), web.NewError(web.CodeForbidden, "no")))
	if rec.Code != 403 {
		t.Fatalf("got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	web.WriteError(rec, req, errors.New("db password=hunter2"))
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestUnknownCodeDefaultsTo500(t *testing.T) {
	if e := web.NewError("NOPE", ""); e.Status != 500 {
		t.Fatalf("got %d", e.Status)
	}
	if got := web.NewError(web.CodeNotFound, "").Error(); got != "NOT_FOUND" {
		t.Fatal(got)
	}
	cause := errors.New("c")
	e := &web.Error{Code: web.CodeInternalError, Cause: cause}
	if !errors.Is(e, cause) || !strings.Contains(e.Error(), "c") {
		t.Fatal("cause not wrapped")
	}
}

func TestFieldErrorsRendered(t *testing.T) {
	e := web.NewError(web.CodeValidationFailed, "bad")
	e.Fields = []web.FieldError{{Field: "name", Code: web.CodeRequired}}
	rec := httptest.NewRecorder()
	web.WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/", nil), e)
	if !strings.Contains(rec.Body.String(), `"errors":[{"field":"name","code":"REQUIRED"}]`) || rec.Code != 422 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestClientIPTrustedProxies(t *testing.T) {
	for name, tc := range map[string]struct {
		hops int
		xff  []string
		want string
	}{
		"hops 0 ignores header":     {0, []string{"198.51.100.1"}, "192.0.2.9"},
		"one hop takes last entry":  {1, []string{"203.0.113.7, 198.51.100.1"}, "198.51.100.1"},
		"spoofed prefix is ignored": {1, []string{"10.0.0.1, 198.51.100.1"}, "198.51.100.1"},
		"two hops":                  {2, []string{"203.0.113.7, 198.51.100.1, 10.1.1.1"}, "198.51.100.1"},
		"several header lines":      {1, []string{"203.0.113.7", "198.51.100.1"}, "198.51.100.1"},
		"too few entries":           {2, []string{"198.51.100.1"}, "192.0.2.9"},
		"no header":                 {1, nil, "192.0.2.9"},
		"garbage entry":             {1, []string{"198.51.100.1, nope"}, "192.0.2.9"},
		"ipv4-mapped and zone":      {1, []string{"::ffff:198.51.100.1"}, "198.51.100.1"},
		"ipv6":                      {1, []string{"2001:db8::1"}, "2001:db8::1"},
		"empty entry":               {1, []string{"198.51.100.1,"}, "192.0.2.9"},
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "192.0.2.9:4000"
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			var got netip.Addr
			web.TrustedProxies(tc.hops)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = web.ClientIP(r)
			})).ServeHTTP(httptest.NewRecorder(), r)
			if got.String() != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

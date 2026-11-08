package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/server"
)

// The public list in code must equal the operations the spec marks
// `security: []`; otherwise the contract and the middleware disagree.
func TestPublicRoutesMatchOpenAPI(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.Security != nil && len(*op.Security) == 0 {
				want = append(want, method+" /api/v1"+path)
			}
		}
	}
	var got []string
	for _, p := range server.PublicRoutes {
		got = append(got, p.Method+" "+p.Path)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Errorf("PublicRoutes\n got  %v\n want %v (spec, security: [])", got, want)
	}
}

func TestIsPublicNeedsExactMethodAndPath(t *testing.T) {
	tests := []struct {
		method, path string
		want         bool
	}{
		{"GET", "/api/v1/auth/csrf", true},
		{"POST", "/api/v1/auth/logout", true},
		{"POST", "/api/v1/auth/csrf", false},
		{"HEAD", "/api/v1/deadline", false},
		{"GET", "/api/v1/auth/csrf/", false},
		{"GET", "/api/v1/auth/me", false},
		{"GET", "/api/v1/auth/../ideas", false},
		{"GET", "/auth/csrf", false},
	}
	for _, tc := range tests {
		req := httptest.NewRequestWithContext(context.Background(), tc.method, "http://x"+tc.path, nil)
		if got := server.IsPublic(req); got != tc.want {
			t.Errorf("%s %s = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestActionForCoversPublicWrites(t *testing.T) {
	for _, p := range server.PublicRoutes {
		if p.Method == http.MethodGet {
			continue
		}
		r := httptest.NewRequest(p.Method, p.Path, nil)
		if server.ActionFor(r) == "" {
			t.Errorf("%s %s has no audit action for a CSRF denial", p.Method, p.Path)
		}
	}
}

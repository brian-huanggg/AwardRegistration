package web

import (
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"
)

// RoutePattern is the matched chi pattern (/api/v1/ideas/{id}), never the raw
// path, so IDs do not leak into the audit log and rows group by route. It is
// empty when no route matched.
func RoutePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		return rc.RoutePattern()
	}
	return ""
}

// ClientIP is the TCP peer. Trusted-proxy handling is not decided yet.
func ClientIP(r *http.Request) netip.Addr {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr().Unmap()
}

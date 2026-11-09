package web

import (
	"context"
	"net/http"
	"net/netip"
	"strings"

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

type clientIPKey struct{}

// TrustedProxies resolves the client address from X-Forwarded-For when hops > 0
// and stores it for ClientIP. hops is the number of proxies in front of the
// service that append to the header; the client is the hops-th entry from the
// right (earlier entries are client-supplied and ignored). With hops <= 0, or
// when the header has fewer than hops valid entries, the TCP peer is used.
func TrustedProxies(hops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if hops <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip, ok := forwardedClient(r.Header.Values("X-Forwarded-For"), hops); ok {
				r = r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func forwardedClient(values []string, hops int) (netip.Addr, bool) {
	var parts []string
	for _, v := range values {
		parts = append(parts, strings.Split(v, ",")...)
	}
	if len(parts) < hops {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(parts[len(parts)-hops]))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.WithZone("").Unmap(), true
}

// ClientIP is the address TrustedProxies resolved, else the TCP peer.
func ClientIP(r *http.Request) netip.Addr {
	if ip, ok := r.Context().Value(clientIPKey{}).(netip.Addr); ok {
		return ip
	}
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return ap.Addr().Unmap()
}

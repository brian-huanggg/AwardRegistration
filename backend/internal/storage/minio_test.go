package storage

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
)

func newStore(t *testing.T, endpoint string) SlideStore {
	t.Helper()
	s, err := NewMinIO(MinIOConfig{Endpoint: endpoint, AccessKey: "a", SecretKey: "bbbbbbbb", Bucket: "slides"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func assertUnavailable(t *testing.T, err error) {
	t.Helper()
	var we *web.Error
	if !errors.As(err, &we) || we.Code != web.CodeStorageUnavailable || we.Status != 503 {
		t.Fatalf("want 503 STORAGE_UNAVAILABLE, got %v", err)
	}
}

// refusing accepts a connection and drops it at once, counting attempts.
func refusing(t *testing.T) (addr string, attempts *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	attempts = new(atomic.Int32)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			attempts.Add(1)
			_ = c.Close()
		}
	}()
	return ln.Addr().String(), attempts
}

// OI-52 / A3c: retries are off, so a failing server sees exactly one attempt.
func TestSingleAttemptPerCall(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(SlideStore) error{
		"put":    func(s SlideStore) error { return s.Put(ctx, "k", strings.NewReader("abc"), 3, "application/pdf") },
		"get":    func(s SlideStore) error { _, _, err := s.Get(ctx, "k"); return err },
		"delete": func(s SlideStore) error { return s.Delete(ctx, "k") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			addr, attempts := refusing(t)
			assertUnavailable(t, call(newStore(t, addr)))
			if n := attempts.Load(); n != 1 {
				t.Fatalf("connection attempts = %d, want 1", n)
			}
		})
	}
}

func TestUnreachableIsUnavailable(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	_ = ln.Close() // nothing listens any more
	err := newStore(t, addr).Put(context.Background(), "k", strings.NewReader("x"), 1, "application/pdf")
	assertUnavailable(t, err)
}

func TestNewMinIORejectsBadEndpoint(t *testing.T) {
	if _, err := NewMinIO(MinIOConfig{Endpoint: "http://host/path"}); err == nil {
		t.Fatal("want error")
	}
}

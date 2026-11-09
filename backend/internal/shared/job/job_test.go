package job

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func noRand(int64) int64 { return 0 }

func logger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestRunRepeatsUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(ctx, slog.New(slog.DiscardHandler), Spec{Name: "t", Interval: time.Millisecond}, func(context.Context) error {
			if calls.Add(1) == 3 {
				cancel()
			}
			return nil
		}, noRand)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("calls = %d, want 3 (no run after cancel)", n)
	}
}

func TestRunLogsFailureAndContinues(t *testing.T) {
	buf := &syncBuf{}
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	run(ctx, slog.New(slog.NewTextHandler(buf, nil)), Spec{Name: "purge", Interval: time.Millisecond}, func(context.Context) error {
		if calls.Add(1) == 2 {
			cancel()
		}
		return errors.New("boom")
	}, noRand)
	out := buf.String()
	// The first failure is logged; the second, returned after cancel, is not.
	if strings.Count(out, "job run failed") != 1 || !strings.Contains(out, "job=purge") || !strings.Contains(out, "error=boom") {
		t.Errorf("log = %q", out)
	}
}

func TestRunFirstCallWaitsForJitter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var buf bytes.Buffer
	called := false
	run(ctx, logger(&buf), Spec{Name: "t", Interval: time.Millisecond, Jitter: time.Hour}, func(context.Context) error {
		called = true
		return nil
	}, func(n int64) int64 { return n - 1 })
	if called {
		t.Error("first run must wait for its jitter")
	}
	if !strings.Contains(buf.String(), "job stopped") {
		t.Errorf("log = %q", buf.String())
	}
}

func TestJitter(t *testing.T) {
	var asked int64
	got := jitter(time.Minute, func(n int64) int64 { asked = n; return n / 2 })
	if asked != int64(time.Minute) || got != 30*time.Second {
		t.Errorf("jitter = %v (asked %d)", got, asked)
	}
	for _, d := range []time.Duration{0, -time.Second} {
		if got := jitter(d, func(int64) int64 { panic("must not be called") }); got != 0 {
			t.Errorf("jitter(%v) = %v", d, got)
		}
	}
}

func TestRunUsesRealRandom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	Run(ctx, slog.New(slog.DiscardHandler), Spec{Name: "t", Interval: time.Hour, Jitter: time.Nanosecond}, func(context.Context) error {
		cancel()
		return nil
	})
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestDrainRepeatsUntilShortBatch(t *testing.T) {
	batches := []int64{3, 3, 1, 99}
	var calls int
	n, err := drain(context.Background(), nil, 3, func(_ context.Context, _ *sql.DB, limit int) (int64, error) {
		if limit != 3 {
			t.Fatalf("limit = %d", limit)
		}
		calls++
		return batches[calls-1], nil
	})
	if err != nil || n != 7 || calls != 3 {
		t.Fatalf("drain = %d, %v after %d calls; want 7 after 3", n, err, calls)
	}
}

func TestDrainStopsOnError(t *testing.T) {
	boom := errors.New("boom")
	var calls int
	n, err := drain(context.Background(), nil, 2, func(context.Context, *sql.DB, int) (int64, error) {
		calls++
		if calls == 2 {
			return 0, boom
		}
		return 2, nil
	})
	if !errors.Is(err, boom) || n != 2 || calls != 2 {
		t.Fatalf("drain = %d, %v after %d calls", n, err, calls)
	}
}

func TestDrainStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int
	n, err := drain(ctx, nil, 1, func(context.Context, *sql.DB, int) (int64, error) {
		calls++
		cancel()
		return 1, nil
	})
	if !errors.Is(err, context.Canceled) || n != 1 || calls != 1 {
		t.Fatalf("drain = %d, %v after %d calls", n, err, calls)
	}
}

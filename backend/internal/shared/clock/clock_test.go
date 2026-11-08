package clock

import (
	"context"
	"testing"
	"time"
)

func TestFixed(t *testing.T) {
	want := time.Date(2026, 1, 2, 3, 4, 5, 6000, time.UTC)
	got, err := Fixed(want).Now(context.Background())
	if err != nil || !got.Equal(want) {
		t.Fatalf("got %v, %v", got, err)
	}
}

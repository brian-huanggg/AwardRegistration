//go:build integration

package migration_test

import (
	"strings"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
)

func TestPendingFileDeletionRules(t *testing.T) {
	d := dbtest.New(t)
	ins := func(key, reason string) error {
		_, err := d.Exec(`INSERT INTO pending_file_deletion (object_key, reason) VALUES (?, ?)`, key, reason)
		return err
	}
	for _, r := range []string{"REPLACED", "DELETED", "UPLOAD_ABORTED"} {
		if err := ins("slides/a/b.pdf", r); err != nil {
			t.Errorf("reason %s (same key again): want accepted, got %v", r, err)
		}
	}
	if err := ins(strings.Repeat("k", 128), "DELETED"); err != nil {
		t.Errorf("128-char key: %v", err)
	}
	for name, c := range map[string][2]string{
		"unknown reason":    {"slides/x.pdf", "EXPIRED"},
		"lower-case reason": {"slides/x.pdf", "replaced"},
		"empty key":         {"", "DELETED"},
		"129-char key":      {strings.Repeat("k", 129), "DELETED"},
	} {
		if err := ins(c[0], c[1]); err == nil {
			t.Errorf("%s: want rejected", name)
		}
	}

	var attempts int
	var due bool
	if err := d.QueryRow(`SELECT attempts, next_attempt_at <= NOW(6) AND created_at <= NOW(6)
	                        FROM pending_file_deletion WHERE reason = 'DELETED' LIMIT 1`).Scan(&attempts, &due); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || !due {
		t.Errorf("defaults: attempts=%d due=%v, want 0 and due now", attempts, due)
	}
}

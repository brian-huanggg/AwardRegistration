package matrix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The data file itself is checked without Docker: ids are unique, actors are
// known, and every expected code exists in the shared error catalogue with the
// status the catalogue gives it.
func TestMatrixFileIsConsistent(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(testdata, "error-codes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cat []struct {
		Code   string
		Status int
	}
	if err := json.Unmarshal(raw, &cat); err != nil {
		t.Fatal(err)
	}
	status := map[string]int{}
	for _, c := range cat {
		status[c.Code] = c.Status
	}

	seen := map[string]bool{}
	for _, r := range load(t) {
		if seen[r.ID] {
			t.Errorf("duplicate row id %q", r.ID)
		}
		seen[r.ID] = true
		for actor, c := range r.Expect {
			if !slices.Contains(Actors, actor) {
				t.Errorf("%s: unknown actor %q", r.ID, actor)
			}
			if c.Code == "" {
				if c.Status >= 400 {
					t.Errorf("%s/%s: %d needs a code", r.ID, actor, c.Status)
				}
				continue
			}
			if want, ok := status[c.Code]; !ok || want != c.Status {
				t.Errorf("%s/%s: %s is %d in the catalogue, row says %d", r.ID, actor, c.Code, want, c.Status)
			}
		}
	}
}

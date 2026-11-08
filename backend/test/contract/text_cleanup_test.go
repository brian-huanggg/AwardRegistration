package contract

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/text"
)

type textCase struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Input       string `json:"input"`
	Max         int    `json:"max"`
	Want        string `json:"want"`
	WantCount   int    `json:"wantCount"`
	WantProblem string `json:"wantProblem"`
}

func TestTextCleanup(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/text-cleanup.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Cases []textCase }
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range f.Cases {
		t.Run(c.ID, func(t *testing.T) {
			var got string
			var p text.Problem
			switch c.Kind {
			case "name":
				got, p = text.CheckName(c.Input, c.Max)
			case "summary":
				got, p = text.CheckSummary(c.Input, c.Max)
			default:
				t.Fatalf("unknown kind %q", c.Kind)
			}
			if got != c.Want {
				t.Errorf("cleaned = %q, want %q", got, c.Want)
			}
			if n := text.Count(got); n != c.WantCount {
				t.Errorf("count = %d, want %d", n, c.WantCount)
			}
			if string(p) != c.WantProblem {
				t.Errorf("problem = %q, want %q", p, c.WantProblem)
			}
		})
	}
}

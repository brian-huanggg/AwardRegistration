package contract

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

func TestExpectedFilename(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/expected-filename.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []struct {
			ID          string `json:"id"`
			Category    string `json:"category"`
			Org         string `json:"org"`
			LeaderName  string `json:"leaderName"`
			LeaderEmpNo string `json:"leaderEmpNo"`
			IdeaName    string `json:"ideaName"`
			WantBase    string `json:"wantBase"`
			WantBytes   int    `json:"wantBytes"`
			WantTooLong bool   `json:"wantTooLong"`
		}
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range f.Cases {
		t.Run(c.ID, func(t *testing.T) {
			got, err := storage.ExpectedFilenameBase(c.Category, c.Org, c.LeaderName, c.LeaderEmpNo, c.IdeaName)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.WantBase {
				t.Errorf("base = %q, want %q", got, c.WantBase)
			}
			if n := len(got + ".pptx"); n != c.WantBytes {
				t.Errorf("bytes = %d, want %d", n, c.WantBytes)
			}
			err = storage.CheckFilenameLength(got)
			if c.WantTooLong {
				var pe *web.Error
				if !errors.As(err, &pe) || pe.Code != web.CodeFilenameTooLong {
					t.Errorf("err = %v, want FILENAME_TOO_LONG", err)
				}
			} else if err != nil {
				t.Errorf("unexpected error %v", err)
			}
		})
	}
}

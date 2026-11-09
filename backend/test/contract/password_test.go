package contract

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/account"
)

func TestPasswordPolicy(t *testing.T) {
	raw, err := os.ReadFile("../../../testdata/password.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Cases []struct {
			ID        string   `json:"id"`
			Password  string   `json:"password"`
			EmpNo     string   `json:"empNo"`
			WantRules []string `json:"wantRules"`
		}
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("no cases")
	}
	policy := account.DefaultPolicy()
	for _, c := range f.Cases {
		t.Run(c.ID, func(t *testing.T) {
			got := []string{}
			for _, fe := range policy.Check(c.Password, c.EmpNo) {
				if fe.Field != "password" || fe.Code != "PASSWORD_POLICY" {
					t.Errorf("entry %+v: want field password, code PASSWORD_POLICY", fe)
				}
				got = append(got, fe.Rule)
			}
			if !slices.Equal(got, c.WantRules) {
				t.Errorf("rules = %v, want %v", got, c.WantRules)
			}
		})
	}
}

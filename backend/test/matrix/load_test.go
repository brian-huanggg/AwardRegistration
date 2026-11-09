// Package matrix runs the permission matrix (PRD §7) from
// testdata/permission-matrix.json: every cell is one test case.
package matrix

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testdata = "../../../testdata"

// Cell is the expected outcome of one (row, actor) pair.
type Cell struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
}

// Row is one operation or check-order case.
type Row struct {
	ID          string          `json:"id"`
	Operation   string          `json:"operation"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Body        json.RawMessage `json:"body"`
	Multipart   *Multipart      `json:"multipart"`
	CSRF        string          `json:"csrf"`
	State       string          `json:"state"`
	OutsideSpec bool            `json:"outsideSpec"`
	Expect      map[string]Cell `json:"expect"`
}

// Multipart is a multipart/form-data body: text parts and one file part.
type Multipart struct {
	Fields map[string]string `json:"fields"`
	File   struct {
		Field    string `json:"field"`
		Filename string `json:"filename"`
		Content  string `json:"content"` // "pdf": a minimal valid PDF
	} `json:"file"`
}

// Actors in PRD §7 order. The harness has a fixture for each one it lists in
// fixtures; a row naming any other actor fails loudly instead of passing.
var Actors = []string{"visitor", "user", "member", "leader", "admin"}

func load(tb testing.TB) []Row {
	tb.Helper()
	raw, err := os.ReadFile(filepath.Join(testdata, "permission-matrix.json"))
	if err != nil {
		tb.Fatal(err)
	}
	var m struct{ Rows []Row }
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		tb.Fatalf("permission-matrix.json: %v", err)
	}
	return m.Rows
}

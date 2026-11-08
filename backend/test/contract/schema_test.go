// Package contract validates every shared file in /testdata against its
// JSON Schema in /testdata/schema (Architecture D52).
package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const root = "../../../testdata"

func load(t *testing.T, path string) any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	v, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v
}

func TestEveryTestdataFileMatchesItsSchema(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no testdata files found")
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		t.Run(name, func(t *testing.T) {
			schemaPath := filepath.Join(root, "schema", name+".schema.json")
			if _, err := os.Stat(schemaPath); err != nil {
				t.Fatalf("missing schema %s", schemaPath)
			}
			sch, err := jsonschema.NewCompiler().Compile(schemaPath)
			if err != nil {
				t.Fatalf("compile %s: %v", schemaPath, err)
			}
			if err := sch.Validate(load(t, file)); err != nil {
				t.Fatalf("%s violates schema:\n%v", file, err)
			}
		})
	}
}

func TestEverySchemaHasTestdata(t *testing.T) {
	schemas, _ := filepath.Glob(filepath.Join(root, "schema", "*.schema.json"))
	for _, s := range schemas {
		raw, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(raw) {
			t.Fatalf("%s is not valid JSON", s)
		}
		name := strings.TrimSuffix(filepath.Base(s), ".schema.json")
		if _, err := os.Stat(filepath.Join(root, name+".json")); err != nil {
			t.Errorf("schema %s has no testdata/%s.json", s, name)
		}
	}
}

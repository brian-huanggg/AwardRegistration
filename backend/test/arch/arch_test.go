// Package arch enforces the architecture rules of Backend.md §2.4 that neither the compiler nor
// depguard can express (B0-11):
//
//  1. shared depends on no business module (D32) and not on the generated api or server packages;
//  2. module-to-module imports are exactly those in allowedEdges (acyclicity is the compiler's job);
//  3. internal/api is imported only by module handlers (top-level package) and internal/server;
//  4. no UPDATE / DELETE / REPLACE / TRUNCATE on audit_log in any queries.sql or .go file.
//
// Imports are read with go/parser (ImportsOnly) so files behind build tags (integration) are
// checked as well. Every rule is a pure function over data and is exercised twice: against the
// real tree, and against a deliberate violation to prove the check turns red.
package arch

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Packages directly under internal/ that are not business modules.
const (
	sharedPkg = "shared"
	apiPkg    = "api"
	serverPkg = "server"
)

// allowedEdges is THE edge table (Backend.md §2.1/§2.4 rule 3) [Proposed]: module -> modules it
// may import. A module directory missing from this table fails the test, so adding a module forces
// a conscious decision about its edges. Facts it encodes:
//   - D29: storage has no dependency on idea; slides sits on idea + storage.
//   - idea -> storage is allowed only for Idea delete (Sequence.md §5): idea inserts
//     pending_file_deletion in its tx and deletes the object after commit. idea should depend on
//     a small interface it declares itself, not on storage's concrete types (bd 1ve).
//   - D30: account never imports auth (SessionRevoker is injected from main.go).
//   - D53: audit is reached by business modules; audit itself imports no module.
//   - admin holds HTTP entry points only and may call any service.
var allowedEdges = map[string][]string{
	"audit":    {},
	"deadline": {"audit"},
	"account":  {"audit"},
	"auth":     {"account", "audit"},
	"storage":  {"audit"},
	"idea":     {"audit", "deadline", "storage"},
	"team":     {"audit", "deadline", "idea"},
	"slides":   {"audit", "deadline", "idea", "storage"},
	"admin":    {"account", "audit", "auth", "deadline", "idea", "slides", "storage", "team"},
}

// imp is one import of one file. Pkg is the importing package path relative to internal/
// ("shared/web", "idea/internal/store"); Target is the imported path relative to internal/
// (empty when it is not inside this module's internal/ tree).
type imp struct {
	File   string
	Pkg    string
	Target string
	Test   bool
}

func top(rel string) string {
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return rel
}

func isModule(name string) bool {
	return name != sharedPkg && name != apiPkg && name != serverPkg && name != ""
}

// checkImports applies rules 1–3 and returns one message per violation.
func checkImports(imps []imp) []string {
	var out []string
	for _, i := range imps {
		if i.Target == "" {
			continue
		}
		from, to := top(i.Pkg), top(i.Target)
		switch {
		case from == sharedPkg && (isModule(to) || to == apiPkg || to == serverPkg):
			out = append(out, fmt.Sprintf("%s: shared must not import %q (rule 1)", i.File, i.Target))
		case from == apiPkg && to != apiPkg && to != sharedPkg:
			out = append(out, fmt.Sprintf("%s: api must not import %q", i.File, i.Target))
		case isModule(from) && to == apiPkg && strings.Contains(i.Pkg, "/internal"):
			out = append(out, fmt.Sprintf("%s: %s is module-internal and must not import api (rule 3)", i.File, i.Pkg))
		case isModule(from) && isModule(to) && from != to && !i.Test:
			if !contains(allowedEdges[from], to) {
				out = append(out, fmt.Sprintf("%s: edge %s -> %s is not in allowedEdges (rule 2)", i.File, from, to))
			}
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// checkModules fails for module directories missing from the edge table, and for table rows or
// targets naming a module that does not exist in the table.
func checkModules(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if isModule(d) {
			if _, ok := allowedEdges[d]; !ok {
				out = append(out, fmt.Sprintf("internal/%s is not in allowedEdges: add its edges", d))
			}
		}
	}
	for m, tos := range allowedEdges {
		for _, t := range tos {
			if _, ok := allowedEdges[t]; !ok {
				out = append(out, fmt.Sprintf("allowedEdges[%s] names unknown module %q", m, t))
			}
			if t == m {
				out = append(out, fmt.Sprintf("allowedEdges[%s] lists itself", m))
			}
		}
	}
	return out
}

// Rule 4: a statement that writes audit_log other than INSERT. \b stops at "audit_log_x" tables.
var auditMutation = regexp.MustCompile("(?is)\\b(?:UPDATE|DELETE\\s+FROM|REPLACE\\s+INTO|TRUNCATE(?:\\s+TABLE)?)\\s+`?(?:\\w+`?\\.`?)?audit_log`?\\b")

// checkAuditSQL scans one file's text.
func checkAuditSQL(file, text string) []string {
	var out []string
	for _, m := range auditMutation.FindAllStringIndex(text, -1) {
		line := 1 + strings.Count(text[:m[0]], "\n")
		out = append(out, fmt.Sprintf("%s:%d: audit_log is append-only: %q (rule 4)", file, line, strings.Join(strings.Fields(text[m[0]:m[1]]), " ")))
	}
	return out
}

// --- tree access -----------------------------------------------------------------------------

func backendRoot(t *testing.T) (root, modPath string) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(l), "module "); ok {
					return dir, strings.TrimSpace(rest)
				}
			}
			t.Fatal("go.mod has no module line")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above test directory")
		}
		dir = parent
	}
}

func collectImports(t *testing.T, root, modPath string) (imps []imp, moduleDirs []string) {
	t.Helper()
	internal := filepath.Join(root, "internal")
	entries, err := os.ReadDir(internal)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			moduleDirs = append(moduleDirs, e.Name())
		}
	}
	prefix := modPath + "/internal/"
	fset := token.NewFileSet()
	err = filepath.WalkDir(internal, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(internal, filepath.Dir(p))
		for _, s := range f.Imports {
			path, _ := strconv.Unquote(s.Path.Value)
			target, ok := strings.CutPrefix(path, prefix)
			if !ok {
				continue
			}
			imps = append(imps, imp{File: filepath.ToSlash(filepath.Join("internal", rel, d.Name())), Pkg: filepath.ToSlash(rel), Target: target, Test: strings.HasSuffix(p, "_test.go")})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return imps, moduleDirs
}

func scanAudit(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == "node_modules" || n == ".git" || n == "arch" && filepath.Base(filepath.Dir(p)) == "test" {
				return filepath.SkipDir // this package quotes the forbidden statements in its own tests
			}
			return nil
		}
		if n := d.Name(); n != "queries.sql" && !strings.HasSuffix(n, ".go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, checkAuditSQL(filepath.ToSlash(rel), string(b))...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func report(t *testing.T, violations []string) {
	t.Helper()
	sort.Strings(violations)
	for _, v := range violations {
		t.Error(v)
	}
}

// --- tests on the real tree ------------------------------------------------------------------

func TestImportGraph(t *testing.T) {
	root, mod := backendRoot(t)
	imps, dirs := collectImports(t, root, mod)
	if len(imps) == 0 {
		t.Fatal("no internal imports found: the scan is broken")
	}
	report(t, append(checkImports(imps), checkModules(dirs)...))
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	root, _ := backendRoot(t)
	report(t, scanAudit(t, root))
}

// --- deliberate violations: each check must go red --------------------------------------------

func TestChecksDetectViolations(t *testing.T) {
	imports := []struct {
		name string
		imp  imp
	}{
		{"shared imports a module", imp{File: "internal/shared/web/x.go", Pkg: "shared/web", Target: "idea"}},
		{"shared imports api", imp{File: "internal/shared/web/x.go", Pkg: "shared/web", Target: "api"}},
		{"shared test imports a module", imp{File: "internal/shared/db/x_test.go", Pkg: "shared/db", Target: "audit", Test: true}},
		{"account imports auth (D30)", imp{File: "internal/account/x.go", Pkg: "account", Target: "auth"}},
		{"storage imports idea (D29)", imp{File: "internal/storage/x.go", Pkg: "storage", Target: "idea"}},
		{"module-internal imports api", imp{File: "internal/idea/internal/store/x.go", Pkg: "idea/internal/store", Target: "api"}},
	}
	for _, c := range imports {
		t.Run(c.name, func(t *testing.T) {
			if len(checkImports([]imp{c.imp})) == 0 {
				t.Errorf("violation not detected: %+v", c.imp)
			}
		})
	}

	allowed := []imp{
		{File: "a.go", Pkg: "slides", Target: "storage"},
		{File: "a.go", Pkg: "idea", Target: "storage"},
		{File: "a.go", Pkg: "idea", Target: "shared/web"},
		{File: "a.go", Pkg: "idea", Target: "api"},
		{File: "a.go", Pkg: "server", Target: "idea"},
		{File: "a.go", Pkg: "idea", Target: "idea/internal/store"},
	}
	if v := checkImports(allowed); len(v) != 0 {
		t.Errorf("legal imports rejected: %v", v)
	}

	if len(checkModules([]string{"newmodule"})) == 0 {
		t.Error("module missing from allowedEdges not detected")
	}
	if v := checkModules([]string{"idea", sharedPkg, apiPkg, serverPkg}); len(v) != 0 {
		t.Errorf("valid layout rejected: %v", v)
	}

	bad := []string{
		"UPDATE audit_log SET action = 'x'",
		"update `audit_log` set action = 'x'",
		"DELETE FROM audit_log WHERE id = 1",
		"delete\n  from audit_log",
		"REPLACE INTO audit_log (id) VALUES (1)",
		"TRUNCATE TABLE audit_log",
		"UPDATE mydb.audit_log SET a = 1",
		`db.Exec("DELETE FROM audit_log")`,
	}
	for _, s := range bad {
		if len(checkAuditSQL("q.sql", s)) == 0 {
			t.Errorf("audit mutation not detected: %q", s)
		}
	}
	good := []string{
		"INSERT INTO audit_log (id) VALUES (1)",
		"SELECT * FROM audit_log WHERE id < ?",
		"UPDATE audit_log_archive SET a = 1",
		"UPDATE idea SET title = ?",
	}
	for _, s := range good {
		if v := checkAuditSQL("q.sql", s); len(v) != 0 {
			t.Errorf("legal statement rejected %q: %v", s, v)
		}
	}
}

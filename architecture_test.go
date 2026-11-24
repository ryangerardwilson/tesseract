package tesseract_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const modulePath = "github.com/ryangerardwilson/tesseract"

func TestPackagesImportOnlyWhatTheirRuleAllows(t *testing.T) {
	// allowedImports is the dependency rule for every package in this module:
	// the module packages it may import. tmux and snapshot are the core and import
	// no adapter or UI package; UI and entry packages depend on them, not the
	// reverse. A new package fails this test until it is given a row.
	allowedImports := map[string][]string{
		"":                    nil,
		"cmd/tesseract":       {"internal/cli"},
		"internal/assets":     {""},
		"internal/browser":    {"internal/quickshell", "internal/snapshot", "internal/tmux"},
		"internal/cli":        {"internal/assets", "internal/browser", "internal/keyd", "internal/quickshell", "internal/snapshot", "internal/tmux", "internal/version"},
		"internal/keyd":       nil,
		"internal/quickshell": {"internal/assets", "internal/snapshot", "internal/tmux"},
		"internal/snapshot":   {"internal/tmux"},
		"internal/tmux":       nil,
		"internal/version":    nil,
	}

	// Only the terminal browser may use a UI framework.
	thirdPartyImporters := map[string]bool{"internal/browser": true}

	imports := moduleImports(t)
	for pkg := range allowedImports {
		if _, ok := imports[pkg]; !ok {
			t.Errorf("allowedImports has a row for %q, which has no Go files", pkg)
		}
	}
	for pkg, deps := range imports {
		allowed, known := allowedImports[pkg]
		if !known {
			t.Errorf("package %q has no dependency rule in allowedImports", pkg)
			continue
		}
		for _, dep := range deps {
			if strings.HasPrefix(dep, "third-party:") {
				if !thirdPartyImporters[pkg] {
					t.Errorf("package %q imports %s; only the browser may import a third-party module", pkg, strings.TrimPrefix(dep, "third-party:"))
				}
				continue
			}
			if !contains(allowed, dep) {
				t.Errorf("package %q imports %q, which its rule does not allow", pkg, dep)
			}
		}
	}
}

// moduleImports maps each package directory, relative to the module root, to
// the module packages and third-party modules its non-test files import.
func moduleImports(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (strings.HasPrefix(entry.Name(), ".") && path != "." || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(path))
		if pkg == "." {
			pkg = ""
		}
		for _, spec := range file.Imports {
			if dep, ok := importedDependency(strings.Trim(spec.Path.Value, `"`)); ok && !contains(out[pkg], dep) {
				out[pkg] = append(out[pkg], dep)
			}
		}
		if _, ok := out[pkg]; !ok {
			out[pkg] = nil
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for pkg := range out {
		sort.Strings(out[pkg])
	}
	return out
}

func importedDependency(path string) (string, bool) {
	if path == modulePath {
		return "", true
	}
	if rest, ok := strings.CutPrefix(path, modulePath+"/"); ok {
		return rest, true
	}
	if first, _, _ := strings.Cut(path, "/"); strings.Contains(first, ".") {
		return "third-party:" + path, true
	}
	return "", false
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

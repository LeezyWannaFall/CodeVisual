package golang

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codevisual/internal/graph"
	"codevisual/internal/project"
)

func analyze(t *testing.T, path string, tags []string) *graph.Snapshot {
	t.Helper()
	p, err := project.Discover(context.Background(), path, tags)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	r, err := (Analyzer{}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	s, err := graph.NewSnapshot(p.Project, r)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestAnalyzeDeclarationsAndReadOnly(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "basic")
	before := contents(t, path)
	s := analyze(t, path, nil)
	counts := map[string]int{}
	names := map[string]graph.Node{}
	for _, n := range s.Nodes {
		counts[n.Kind]++
		names[n.Label] = n
	}
	if counts["package"] != 2 || counts["method"] != 2 || counts["struct"] != 1 || counts["interface"] != 1 || counts["type"] != 2 {
		t.Fatalf("wrong counts: %v", counts)
	}
	if _, ok := names["TestOnlySymbol"]; ok {
		t.Fatal("test source analyzed")
	}
	if _, ok := names["Tagged"]; ok {
		t.Fatal("inactive build tag analyzed")
	}
	if names["Read"].Attributes.Receiver != "*Store[T]" {
		t.Fatalf("wrong receiver: %+v", names["Read"])
	}
	if len(names["Reader"].Attributes.Members) != 1 || names["Reader"].Attributes.Members[0] == "" {
		t.Fatalf("missing interface members: %+v", names["Reader"])
	}
	if names["New"].Source.StartLine < 1 || names["Store"].Attributes.Documentation == "" {
		t.Fatal("missing source/docs")
	}
	importCount, methodCount := 0, 0
	for _, e := range s.Edges {
		if e.Kind == "imports" {
			importCount++
			if e.Evidence == nil {
				t.Fatal("missing import evidence")
			}
		}
		if e.Kind == "declares_method" {
			methodCount++
		}
	}
	if importCount != 2 || methodCount != 2 {
		t.Fatalf("wrong edge counts %d %d", importCount, methodCount)
	}
	if s.Status != "complete" {
		t.Fatalf("unexpected diagnostics: %+v", s.Diagnostics)
	}
	if !reflect.DeepEqual(before, contents(t, path)) {
		t.Fatal("analysis changed input files")
	}
	again := analyze(t, path, nil)
	if s.AnalysisID != again.AnalysisID {
		t.Fatal("snapshot is not reproducible")
	}
	tagged := analyze(t, path, []string{"custom"})
	found := false
	for _, n := range tagged.Nodes {
		if n.Label == "Tagged" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom tag not honored")
	}
}
func TestPartialAnalysis(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.test/broken\n\ngo 1.26.0\n")
	put(t, root, "a.go", "package broken\nimport _ \"example.invalid/not-cached\"\ntype Item struct{}\nfunc Available() {}\n")
	before := contents(t, root)
	s := analyze(t, root, nil)
	if s.Status != "partial" || len(s.Diagnostics) == 0 {
		t.Fatal("missing partial diagnostics")
	}
	found := false
	for _, n := range s.Nodes {
		if n.Label == "Available" {
			found = true
		}
	}
	if !found {
		t.Fatal("lost usable AST")
	}
	if !reflect.DeepEqual(before, contents(t, root)) {
		t.Fatal("modified broken module")
	}
	put(t, root, "a.go", "package broken\ntype Item struct{}\nfunc Bad( {\n")
	s = analyze(t, root, nil)
	if s.Status != "partial" {
		t.Fatal("syntax errors not reported")
	}
}
func TestCancellationAndMissingToolchain(t *testing.T) {
	p, err := project.Discover(context.Background(), "../../../testdata/basic", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = p.Load(ctx); err == nil {
		t.Fatal("expected cancellation")
	}
	t.Setenv("PATH", t.TempDir())
	if err = p.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "toolchain") {
		t.Fatalf("expected toolchain error: %v", err)
	}
}
func TestImportCycleAndCgo(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.test/cycle\n\ngo 1.26.0\n")
	put(t, root, "a/a.go", "package a\nimport _ \"example.test/cycle/b\"\nfunc A() {}\n")
	put(t, root, "b/b.go", "package b\nimport _ \"example.test/cycle/a\"\nfunc B() {}\n")
	put(t, root, "a/cgo.go", "package a\nimport \"C\"\nfunc CgoOnly() {}\n")
	s := analyze(t, root, nil)
	if s.Status != "partial" {
		t.Fatal("expected cycle diagnostic")
	}
	for _, n := range s.Nodes {
		if n.Label == "CgoOnly" {
			t.Fatal("cgo file included")
		}
	}
}
func put(t *testing.T, root, path, content string) {
	t.Helper()
	name := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func contents(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			rel, _ := filepath.Rel(root, path)
			out[rel] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRelativeReplacementPreservesModuleMetadata(t *testing.T) {
	root := t.TempDir()
	put(t, root, "dep/go.mod", "module example.test/dep\n\ngo 1.26.0\n")
	put(t, root, "dep/dep.go", "package dep\ntype Value string\n")
	put(t, root, "app/go.mod", "module example.test/app\n\ngo 1.26.0\nrequire example.test/dep v0.0.0\nreplace example.test/dep => ../dep\n")
	put(t, root, "app/app.go", "package app\nimport \"example.test/dep\"\nfunc Value() dep.Value { return \"x\" }\n")
	before := contents(t, root)
	s := analyze(t, filepath.Join(root, "app"), nil)
	if s.Status != "complete" {
		t.Fatalf("relative replace failed: %+v", s.Diagnostics)
	}
	if !reflect.DeepEqual(before, contents(t, root)) {
		t.Fatal("module metadata changed")
	}
}

func TestUnresolvedInternalImport(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.test/internalmissing\n\ngo 1.26.0\n")
	put(t, root, "a.go", "package main\nimport _ \"example.test/internalmissing/absent\"\nfunc main() {}\n")
	s := analyze(t, root, nil)
	found := false
	for _, n := range s.Nodes {
		if n.Attributes.ImportPath == "example.test/internalmissing/absent" {
			found = true
			if n.Kind != "package" || !n.Attributes.Unresolved {
				t.Fatalf("unresolved import misclassified: %+v", n)
			}
		}
	}
	if !found {
		t.Fatal("missing import reference")
	}
}

func TestRequiredModuleSharingRootPrefixIsExternal(t *testing.T) {
	root := t.TempDir()
	put(t, root, "dep/go.mod", "module example.test/dep\ngo 1.26.0\n")
	put(t, root, "dep/dep.go", "package dep\ntype Value string\n")
	put(t, root, "app/go.mod", "module example.test\ngo 1.26.0\nrequire example.test/dep v0.0.0\nreplace example.test/dep => ../dep\n")
	put(t, root, "app/app.go", "package app\nimport \"example.test/dep\"\nfunc Value() dep.Value { return \"x\" }\n")
	before := contents(t, root)
	s := analyze(t, filepath.Join(root, "app"), nil)
	found := false
	for _, n := range s.Nodes {
		if n.Attributes.ImportPath == "example.test/dep" {
			found = true
			if n.Kind != "externalPackage" {
				t.Fatalf("separate module treated as internal: %+v", n)
			}
		}
	}
	if !found {
		t.Fatal("missing import")
	}
	if !reflect.DeepEqual(before, contents(t, root)) {
		t.Fatal("target metadata changed")
	}
}

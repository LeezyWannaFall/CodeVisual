package python

import (
	"codevisual/internal/graph"
	"codevisual/internal/project"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func analyze(t *testing.T, files map[string]string) (*project.Project, graph.AnalysisResult) {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := (Analyzer{}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = graph.NewSnapshot(p.Project, r); err != nil {
		t.Fatal(err)
	}
	return p, r
}
func TestASTImportsClassesCallsAndNoExecution(t *testing.T) {
	p, r := analyze(t, map[string]string{
		"pyproject.toml":  "[project]\nname='sample'\n",
		"pkg/__init__.py": "",
		"pkg/helpers.py":  "def clean(value):\n    return value.strip()\n",
		"pkg/api.py":      "from .helpers import clean as normalize\nfrom pathlib import Path\n\nclass Worker:\n    label: str\n    def run(self, value):\n        return normalize(value)\n\nasync def handler(value):\n    return normalize(value)\n\ndef shadow(normalize):\n    return normalize('x')\n\ndef outer():\n    def inner():\n        return 1\n    return inner()\n\nopen('EXECUTED', 'w').write('bad')\nraise RuntimeError('must never execute')\n",
	})
	if _, err := os.Stat(filepath.Join(p.Root, "EXECUTED")); !os.IsNotExist(err) {
		t.Fatal("project code executed")
	}
	names := map[string]graph.Node{}
	byID := map[string]graph.Node{}
	for _, n := range r.Nodes {
		names[n.Label] = n
		byID[n.ID] = n
	}
	if names["Worker"].Kind != "class" || names["run"].Kind != "method" || names["handler"].Source == nil {
		t.Fatalf("missing declarations: %+v", r.Nodes)
	}
	if names["handler"].Attributes.Signature != "async def handler(value):" {
		t.Fatalf("signature=%s", names["handler"].Attributes.Signature)
	}
	foundImport, handlerCall, shadowDynamic, nestedCall := false, false, false, false
	for _, e := range r.Edges {
		if e.Kind == "imports" && byID[e.SourceID].Label == "pkg.api" && byID[e.TargetID].Label == "pkg.helpers" {
			foundImport = true
		}
		if e.SourceID == names["handler"].ID && e.TargetID == names["clean"].ID {
			handlerCall = true
		}
		if e.SourceID == names["shadow"].ID && e.Kind == "dynamic_call" {
			shadowDynamic = true
		}
		if e.SourceID == names["outer"].ID && e.TargetID == names["inner"].ID {
			nestedCall = true
		}
	}
	if !foundImport || !handlerCall || !shadowDynamic || !nestedCall {
		t.Fatalf("import=%v call=%v shadow=%v nested=%v", foundImport, handlerCall, shadowDynamic, nestedCall)
	}
}
func TestParseErrorsArePartialAndOtherFilesSurvive(t *testing.T) {
	_, r := analyze(t, map[string]string{"good.py": "def good(): return 1\n", "bad.py": "def (\n"})
	if len(r.Diagnostics) == 0 {
		t.Fatal("missing parse diagnosis")
	}
	found := false
	for _, n := range r.Nodes {
		found = found || n.Label == "good"
	}
	if !found {
		t.Fatal("valid file lost")
	}
}

func TestPythonIgnoresProjectImportHooks(t *testing.T) {
	root := t.TempDir()
	for name, value := range map[string]string{"app.py": "def safe(): return 1\n", "ast.py": "open('AST_EXECUTED','w').write('bad')\nraise RuntimeError('project ast imported')\n", "sitecustomize.py": "open('SITE_EXECUTED','w').write('bad')\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("PYTHONPATH", root)
	if _, err = (Analyzer{}).Analyze(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"AST_EXECUTED", "SITE_EXECUTED"} {
		if _, err := os.Stat(filepath.Join(root, marker)); !os.IsNotExist(err) {
			t.Fatalf("project import hook executed: %s", marker)
		}
	}
}

func TestDecoratedBindingsStopAtDynamicBoundary(t *testing.T) {
	_, result := analyze(t, map[string]string{"app.py": "def decorate(fn):\n    return fn\n\n@decorate\ndef wrapped():\n    return 1\n\ndef caller():\n    return wrapped()\n"})
	var caller, wrapped string
	for _, n := range result.Nodes {
		if n.Label == "caller" {
			caller = n.ID
		}
		if n.Label == "wrapped" && n.Kind == "function" {
			wrapped = n.ID
		}
	}
	found := false
	for _, edge := range result.Edges {
		if edge.SourceID == caller {
			if edge.TargetID == wrapped {
				t.Fatal("decorated target presented as direct call")
			}
			found = found || edge.Kind == "dynamic_call"
		}
	}
	if !found {
		t.Fatal("missing dynamic boundary")
	}
}

package cli

import (
	"bytes"
	"codevisual/internal/graph"
	"codevisual/internal/project"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, args := range [][]string{{"analyze", "/project", "--port", "7441", "--no-browser", "--tags", "one,two"}, {"analyze", "--port=7441", "--no-browser", "--tags=one,two", "/project"}} {
		o, err := Parse(args, io.Discard)
		if err != nil || o.Path != "/project" || o.Port != 7441 || !o.NoBrowser || len(o.Tags) != 2 {
			t.Fatalf("%v: %+v %v", args, o, err)
		}
	}
	for _, args := range [][]string{nil, {"analyze"}, {"other", "."}, {"analyze", ".", "extra"}, {"analyze", ".", "--port=0"}, {"analyze", ".", "--tags=a,,b"}} {
		if _, err := Parse(args, io.Discard); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestVersionDoesNotRequireGo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, arg := range []string{"--version", "version"} {
		var out bytes.Buffer
		if err := Run(context.Background(), []string{arg}, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "CodeVisual "+Version) {
			t.Fatalf("version output: %s", out.String())
		}
	}
}

func TestAnalyzePythonOnlyWithoutGo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.py"), []byte("def hello():\n    return 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := graph.NewSnapshot(p.Project, result)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "complete" || len(p.Languages) != 1 || p.Languages[0] != "python" {
		t.Fatalf("project=%+v diagnostics=%+v", p.Project, result.Diagnostics)
	}
}

func TestAnalyzeMixedWorkspaceAndRebasedSources(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"pyproject.toml":     "[project]\nname='mixed'\n",
		"worker.py":          "def run():\n    return 1\n",
		"backend/go.mod":     "module example.test/backend\n\ngo 1.26.0\n",
		"backend/main.go":    "package backend\nfunc Handle() {}\n",
		"compose.yml":        "services:\n  backend:\n    build: ./backend\n  worker:\n    image: python:3.12\n    volumes: [./worker.py:/app/worker.py]\n    depends_on: [backend]\n",
		"backend/Dockerfile": "FROM golang:1.26\nCOPY . /app\n",
	}
	for name, value := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(p.Languages, ",") != "go,python" {
		t.Fatalf("languages=%v", p.Languages)
	}
	result, err := Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := graph.NewSnapshot(p.Project, result)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "complete" {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	foundGo, foundPython, foundContent := false, false, false
	for _, n := range result.Nodes {
		if n.Label == "Handle" {
			foundGo = n.Source != nil && n.Source.FileID == graph.ID("file", "backend/main.go")
		}
		if n.Label == "run" {
			foundPython = n.Attributes.Language == "python"
		}
		if n.Kind == "dockerStage" {
			for _, c := range n.Attributes.Container.Contents {
				for _, id := range c.PackageIDs {
					foundContent = foundContent || id == graph.ID("package", "example.test/backend")
				}
			}
		}
	}
	if !foundGo || !foundPython || !foundContent {
		t.Fatalf("go=%v python=%v container-code=%v", foundGo, foundPython, foundContent)
	}
	for _, f := range result.Files {
		b, err := project.ReadFile(root, f.RelativePath)
		if err != nil || graph.Hash(b) != f.ContentHash {
			t.Fatalf("invalid source %s: %v", f.RelativePath, err)
		}
	}
	if _, err := snapshot.View(graph.ViewOptions{Level: "containers"}); err != nil {
		t.Fatal(err)
	}
}

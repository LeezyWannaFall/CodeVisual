package containers

import (
	"codevisual/internal/graph"
	"codevisual/internal/project"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T, files map[string]string) (*project.Project, graph.AnalysisResult) {
	t.Helper()
	root := t.TempDir()
	for name, value := range files {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	code := graph.AnalysisResult{Nodes: []graph.Node{{ID: graph.ID("module", p.ModulePath), Kind: "module", Label: p.ModulePath}}}
	for _, file := range []string{"app.py", "ignored.py"} {
		if _, ok := files[file]; ok {
			id := graph.ID("package", file)
			fid := graph.ID("file", file)
			code.Nodes = append(code.Nodes, graph.Node{ID: id, Kind: "package", Source: &graph.SourceRange{FileID: fid, StartLine: 1, EndLine: 1}, Attributes: graph.Attributes{Language: "python"}})
			code.Files = append(code.Files, graph.File{ID: fid, RelativePath: file, ContentHash: graph.Hash([]byte(files[file]))})
		}
	}
	r, err := (Analyzer{Code: code}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = graph.NewSnapshot(p.Project, graph.Merge(code, r)); err != nil {
		t.Fatal(err)
	}
	return p, r
}
func TestComposeBuildStagesDependenciesAndContents(t *testing.T) {
	_, r := setup(t, map[string]string{
		"app.py": "def run(): pass\n", "ignored.py": "def ignored(): pass\n", ".dockerignore": "ignored.py\n",
		"Dockerfile":   "FROM python:3.12 AS build\nWORKDIR /app\nCOPY [\".\", \"/app\"]\nRUN echo do-not-execute\nFROM scratch AS runtime\nCOPY --from=build /out/app /app\nENTRYPOINT [\"/app\"]\n",
		"compose.yaml": "services:\n  api:\n    build:\n      context: .\n      target: runtime\n    ports: [\"8080:80\"]\n    depends_on:\n      db:\n        condition: service_healthy\n    volumes:\n      - ./app.py:/app/app.py:ro\n    environment:\n      TOKEN: private\n  db:\n    image: postgres:16\n    volumes: [data:/var/lib/postgresql/data]\nvolumes:\n  data: {}\n",
	})
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", r.Diagnostics)
	}
	services, stages := 0, 0
	var api *graph.Container
	copyLinked := false
	for _, n := range r.Nodes {
		if n.Kind == "container" {
			services++
			if n.Label == "api" {
				api = n.Attributes.Container
			}
		}
		if n.Kind == "dockerStage" {
			stages++
			for _, c := range n.Attributes.Container.Contents {
				for _, id := range c.PackageIDs {
					if id == graph.ID("package", "ignored.py") {
						t.Fatal("dockerignore ignored")
					}
					copyLinked = copyLinked || id == graph.ID("package", "app.py")
				}
			}
		}
	}
	if services != 2 || stages != 2 || api == nil || !copyLinked {
		t.Fatalf("services=%d stages=%d api=%+v links=%v", services, stages, api, copyLinked)
	}
	if len(api.EnvironmentKeys) != 1 || api.EnvironmentKeys[0] != "TOKEN" {
		t.Fatalf("keys=%v", api.EnvironmentKeys)
	}
	bindLinked := false
	for _, c := range api.Contents {
		if c.Kind == "bind" && len(c.PackageIDs) == 1 {
			bindLinked = true
		}
	}
	found := map[string]bool{}
	for _, e := range r.Edges {
		found[e.Kind] = true
	}
	if !bindLinked || !found["depends_on"] || !found["copies_from"] || !found["uses_network"] || !found["mounts"] {
		t.Fatalf("bind=%v edges=%v", bindLinked, found)
	}
}
func TestStandaloneDockerfileAndMalformedCompose(t *testing.T) {
	_, r := setup(t, map[string]string{"Dockerfile": "FROM alpine:3\nCOPY file.txt /data/\nCMD [\"cat\",\"/data/file.txt\"]\n", "compose.yml": "services: [broken]\n"})
	count := 0
	for _, n := range r.Nodes {
		if n.Kind == "container" {
			count++
		}
	}
	if count != 1 || len(r.Diagnostics) != 1 {
		t.Fatalf("nodes=%+v diagnostics=%+v", r.Nodes, r.Diagnostics)
	}
}
func TestUnresolvedConfigurationIsDiagnosed(t *testing.T) {
	_, r := setup(t, map[string]string{"compose.yml": "services:\n  api:\n    image: ${IMAGE}\n    build: ../outside\n    depends_on: [missing]\n    volumes: [/outside:/data]\n"})
	codes := map[string]bool{}
	for _, d := range r.Diagnostics {
		codes[d.Code] = true
	}
	for _, code := range []string{"compose_interpolation", "docker_context", "compose_dependency", "docker_bind"} {
		if !codes[code] {
			t.Fatalf("missing %s: %+v", code, r.Diagnostics)
		}
	}
}

func TestCopyCannotEscapeContext(t *testing.T) {
	_, r := setup(t, map[string]string{"app.py": "pass", "build/Dockerfile": "FROM scratch\nCOPY ../app.py /app/\nADD /app.py /app/\n", "compose.yaml": "services:\n  app:\n    build: ./build\n"})
	for _, n := range r.Nodes {
		if c := n.Attributes.Container; c != nil {
			for _, item := range c.Contents {
				if len(item.PackageIDs) > 0 {
					t.Fatalf("outside context associated: %+v", item)
				}
			}
		}
	}
}
func TestWildcardCopiesMatchedDirectories(t *testing.T) {
	w := &walker{code: graph.AnalysisResult{Files: []graph.File{{ID: "file", RelativePath: "src/pkg/app.py"}}, Nodes: []graph.Node{{ID: "pkg", Kind: "package", Source: &graph.SourceRange{FileID: "file"}}}}}
	if ids := w.packages("src/*", nil); len(ids) != 1 {
		t.Fatalf("missing directory contents: %v", ids)
	}
}

func TestCompletionDependencyDeclaration(t *testing.T) {
	_, r := setup(t, map[string]string{"compose.yaml": "services:\n  app:\n    image: app\n    depends_on:\n      migration:\n        condition: service_completed_successfully\n  migration:\n    image: worker\n"})
	for _, edge := range r.Edges {
		if edge.Kind == "depends_on" && edge.Label == "service_completed_successfully" {
			return
		}
	}
	t.Fatal("missing completion condition")
}

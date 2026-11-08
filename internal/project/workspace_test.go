package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDetectsLanguagesAndInfrastructure(t *testing.T) {
	for _, tt := range []struct {
		name      string
		files     map[string]string
		languages string
	}{
		{"python", map[string]string{"pyproject.toml": "[project]\nname='demo'\n", "app.py": "def hello(): pass\n", ".venv/hidden.py": "bad", "venv/hidden.py": "bad", "test_sample.py": "bad", "tests/helper.py": "bad"}, "python"},
		{"go", map[string]string{"go.mod": "module example.test/demo\ngo 1.26.0\n", "main.go": "package main\n"}, "go"},
		{"mixed", map[string]string{"go.mod": "module example.test/demo\ngo 1.26.0\n", "main.go": "package main\n", "worker/app.py": "def run(): pass\n", "Dockerfile": "FROM python:3.12\n", "compose.yml": "services: {}\n"}, "go,python"},
		{"docker", map[string]string{"Dockerfile": "FROM nginx:alpine\n", "compose.yaml": "services: {}\n"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, value := range tt.files {
				write(t, root, name, value)
			}
			p, err := Open(context.Background(), root, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(p.Languages, ","); got != tt.languages {
				t.Fatalf("languages=%s", got)
			}
			if !sameDirectory(t, p.Root, root) {
				t.Fatalf("root=%s", p.Root)
			}
			if tt.name == "python" && len(p.PythonFiles) != 1 {
				t.Fatalf("files=%v", p.PythonFiles)
			}
		})
	}
}
func TestOpenKeepsModuleAndSymlinkBoundaries(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/demo\ngo 1.26.0\n")
	write(t, root, "main.go", "package main\n")
	write(t, root, "nested/go.mod", "module nested\n")
	write(t, root, "nested/app.py", "raise RuntimeError()\n")
	outside := t.TempDir()
	write(t, outside, "secret.py", "secret")
	if err := os.Symlink(filepath.Join(outside, "secret.py"), filepath.Join(root, "linked.py")); err != nil {
		t.Skip(err)
	}
	p, err := Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.PythonFiles) != 0 || len(p.Diagnostics) != 1 {
		t.Fatalf("unexpected scan: %+v", p)
	}
}

func TestDockerfileIgnoreNotDiscoveredAsDockerfile(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Dockerfile", "FROM scratch\n")
	write(t, root, "Dockerfile.dockerignore", "*.py\n")
	p, err := Open(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Dockerfiles) != 1 {
		t.Fatalf("Dockerfiles: %v", p.Dockerfiles)
	}
}

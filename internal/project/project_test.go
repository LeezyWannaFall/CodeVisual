package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, path, content string) {
	t.Helper()
	name := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestDiscoverBoundaries(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/root\n\ngo 1.26.0\n")
	write(t, root, "main.go", "package root\n")
	write(t, root, "sub/sub.go", "package sub\n")
	for _, dir := range []string{".git", ".hidden", "vendor", "node_modules", "testdata"} {
		write(t, root, dir+"/bad.go", "invalid")
	}
	write(t, root, "nested/go.mod", "module nested\n")
	write(t, root, "nested/n.go", "package nested\n")
	write(t, root, "go.work", "go 1.26.0\nuse .\n")
	if err := os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	p, err := Discover(context.Background(), filepath.Join(root, "sub"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != root || strings.Join(p.Directories, ",") != ".,sub" {
		t.Fatalf("unexpected discovery: %+v", p)
	}
	if len(p.Diagnostics) != 2 {
		t.Fatalf("expected workspace and nested module notes: %+v", p.Diagnostics)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Discover(ctx, root, nil); err == nil {
		t.Fatal("expected cancellation")
	}
}
func TestReadFileRejectsTraversalAndSymlinks(t *testing.T) {
	root := t.TempDir()
	write(t, root, "ok.go", "package ok\n")
	outside := t.TempDir()
	write(t, outside, "secret", "secret")
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../secret", filepath.Join(outside, "secret"), "link.go"} {
		if _, err := ReadFile(root, name); err == nil {
			t.Errorf("accepted unsafe path %s", name)
		}
	}
	if b, err := ReadFile(root, "ok.go"); err != nil || string(b) != "package ok\n" {
		t.Fatalf("read failed: %q %v", b, err)
	}
}
func TestOfflineEnvironmentOverridesHooks(t *testing.T) {
	for _, key := range []string{"GOPROXY", "GOSUMDB", "GOVCS", "GOTOOLCHAIN", "GOWORK", "GOPACKAGESDRIVER", "GOFLAGS", "CGO_ENABLED", "GONOPROXY", "GOPRIVATE", "GO_TELEMETRY_CHILD"} {
		t.Setenv(key, "untrusted")
	}
	env := map[string]string{}
	for _, entry := range OfflineEnv("/tmp/isolated-telemetry") {
		k, v, _ := strings.Cut(entry, "=")
		env[k] = v
	}
	for k, want := range map[string]string{"GOPROXY": "off", "GONOPROXY": "none", "TEST_TELEMETRY_DIR": "/tmp/isolated-telemetry", "GO_TELEMETRY_CHILD": "", "GOSUMDB": "off", "GOVCS": "off", "GOTOOLCHAIN": "local", "GOWORK": "off", "GOPACKAGESDRIVER": "off", "GOFLAGS": "-mod=readonly", "CGO_ENABLED": "0"} {
		if env[k] != want {
			t.Errorf("%s=%s", k, env[k])
		}
	}
}
func TestMissingAndEmptyModule(t *testing.T) {
	root := t.TempDir()
	if _, err := Discover(context.Background(), root, nil); err == nil {
		t.Fatal("missing module accepted")
	}
	write(t, root, "go.mod", "module empty\n")
	if _, err := Discover(context.Background(), root, nil); err == nil {
		t.Fatal("empty module accepted")
	}
}

func TestToolchainTelemetryIsIsolated(t *testing.T) {
	root := t.TempDir()
	write(t, root, "mode", "off\n")
	cmd := exec.Command("go", "env", "GOTELEMETRY", "GOTELEMETRYDIR")
	cmd.Env = OfflineEnv(root)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go env: %v: %s", err, b)
	}
	if string(b) != "off\n"+root+"\n" {
		t.Fatalf("toolchain telemetry hook changed: %s", b)
	}
}

func TestValidateToolchain(t *testing.T) {
	for _, tt := range []struct{ installed, required, message string }{
		{"go1.26.0", "go1.26.0", ""},
		{"go1.27.1", "go1.26.0", ""},
		{"devel go1.27-abcdef", "go1.26.0", ""},
		{"go1.25.9", "go1.26.0", "too old"},
		{"go1.26.0", "go1.27.0", "go1.27.0"},
		{"unknown", "go1.26.0", "cannot determine"},
	} {
		t.Run(tt.installed+"/"+tt.required, func(t *testing.T) {
			err := validateToolchain(tt.installed, tt.required)
			if tt.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("error=%v, want %q", err, tt.message)
			}
		})
	}
}

func TestLoadExplainsMissingToolchain(t *testing.T) {
	p, err := Discover(context.Background(), "../../testdata/basic", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	if err = p.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "install Go 1.26") {
		t.Fatalf("error=%v", err)
	}
}

func TestLoadExplainsModuleToolchainRequirement(t *testing.T) {
	root := t.TempDir()
	metadata := "module example.test/future\n\ngo 99.0.0\n"
	write(t, root, "go.mod", metadata)
	write(t, root, "main.go", "package future\n")
	p, err := Discover(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "requires go99.0.0") {
		t.Fatalf("error=%v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(b) != metadata {
		t.Fatalf("module metadata changed: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); !os.IsNotExist(err) {
		t.Fatalf("unexpected go.sum: %v", err)
	}
}

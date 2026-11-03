package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codevisual/internal/graph"
)

func handler(t *testing.T) (http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	content := []byte("package demo\nfunc Hello() {}\n")
	if err := os.WriteFile(filepath.Join(root, "demo.go"), content, 0644); err != nil {
		t.Fatal(err)
	}
	f := graph.File{ID: "file_demo", RelativePath: "demo.go", ContentHash: graph.Hash(content)}
	source := &graph.SourceRange{FileID: f.ID, StartLine: 2, StartColumn: 1, EndLine: 2, EndColumn: 16}
	r := graph.AnalysisResult{Nodes: []graph.Node{{ID: graph.ID("module", "example.test/demo"), Kind: "module", Label: "demo"}, {ID: "pkg", Kind: "package", Label: "demo"}, {ID: "fn", Kind: "function", Label: "Hello", PackageID: "pkg", ParentID: "pkg", Source: source, Attributes: graph.Attributes{Exported: true}}}, Files: []graph.File{f}}
	s, err := graph.NewSnapshot(graph.Project{Root: root, ModulePath: "example.test/demo"}, r)
	if err != nil {
		t.Fatal(err)
	}
	return New(s, "127.0.0.1:7331"), root
}
func request(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://127.0.0.1:7331"+path, nil)
	h.ServeHTTP(w, r)
	return w
}
func TestAPIAndSource(t *testing.T) {
	h, root := handler(t)
	for _, path := range []string{"/api/v1/project", "/api/v1/tree", "/api/v1/graph", "/api/v1/graph?level=http", "/api/v1/graph?level=flow&rootId=fn", "/api/v1/graph?level=database", "/api/v1/graph?level=symbols&packageId=pkg", "/api/v1/nodes/fn", "/api/v1/files/file_demo", "/api/v1/diagnostics", "/api/v1/search?q=hello"} {
		w := request(h, path)
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), root) {
			t.Fatal("absolute root leaked")
		}
	}
	for _, path := range []string{"/api/v1/graph?level=invalid", "/api/v1/graph?level=flow", "/api/v1/graph?level=flow&rootId=fn&depth=0", "/api/v1/graph?level=flow&rootId=fn&includePossible=invalid", "/api/v1/graph?level=symbols", "/api/v1/graph?offset=-1", "/api/v1/graph?includeExternal=maybe"} {
		if w := request(h, path); w.Code != 400 {
			t.Fatalf("expected 400: %s %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/missing", "/api/v2/project", "/api/v1/files/missing", "/api/v1/nodes/missing"} {
		w := request(h, path)
		if w.Code != 404 || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("expected JSON 404: %s %d", path, w.Code)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "demo.go"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "/api/v1/files/file_demo"); w.Code != 409 {
		t.Fatalf("changed source: %d", w.Code)
	}
	if err := os.Remove(filepath.Join(root, "demo.go")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "demo.go")); err != nil {
		t.Fatal(err)
	}
	if w := request(h, "/api/v1/files/file_demo"); w.Code != 409 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal("symlink source exposed")
	}
}
func TestLocalRequestGuardAndSPA(t *testing.T) {
	h, _ := handler(t)
	for _, tc := range []struct{ host, origin, site string }{{"evil.test:7331", "", ""}, {"127.0.0.1:7331", "http://evil.test", ""}, {"127.0.0.1:7331", "null", ""}, {"127.0.0.1:7331", "", "cross-site"}} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:7331/api/v1/project", nil)
		r.Host = tc.host
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Sec-Fetch-Site", tc.site)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("guard accepted %+v", tc)
		}
	}
	for _, path := range []string{"/", "/explore/packages"} {
		w := request(h, path)
		if w.Code != 200 || !strings.Contains(strings.ToLower(w.Body.String()), "<!doctype html>") {
			t.Fatalf("SPA unavailable %s: %d", path, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:7331/api/v1/project", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 405 {
		t.Fatal("mutation route accepted")
	}
}

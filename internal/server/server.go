// Package server serves the read-only local API and embedded web application.
package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"codevisual/internal/graph"
	"codevisual/internal/project"
	"github.com/go-chi/chi/v5"
)

//go:embed all:webdist
var assets embed.FS

type api struct{ snapshot *graph.Snapshot }
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func fail(w http.ResponseWriter, status int, code, message string) {
	v := apiError{}
	v.Error.Code = code
	v.Error.Message = message
	writeJSON(w, status, v)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// New pins Host and Origin to the listener authority to prevent DNS rebinding.
func New(snapshot *graph.Snapshot, authority string) http.Handler {
	a := &api{snapshot: snapshot}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
			if r.Host != authority {
				fail(w, http.StatusForbidden, "invalid_host", "Only the local server address is allowed")
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Scheme != "http" || u.Host != authority || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
					fail(w, http.StatusForbidden, "invalid_origin", "Cross-origin requests are not allowed")
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				fail(w, http.StatusForbidden, "invalid_origin", "Cross-site requests are not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/project", a.project)
		r.Get("/tree", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.snapshot.Tree()) })
		r.Get("/graph", a.graph)
		r.Get("/search", a.search)
		r.Get("/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
			n, ok := a.snapshot.Node(chi.URLParam(r, "id"))
			if !ok {
				fail(w, 404, "node_not_found", "Node not found")
				return
			}
			writeJSON(w, 200, n)
		})
		r.Get("/files/{id}", a.file)
		r.Get("/diagnostics", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, a.snapshot.Diagnostics) })
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			fail(w, 404, "not_found", "API endpoint not found")
			return
		}
		serveSPA(w, r)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		fail(w, 405, "method_not_allowed", "Only GET requests are supported")
	})
	return r
}
func (a *api) project(w http.ResponseWriter, r *http.Request) {
	counts := map[string]int{}
	for _, n := range a.snapshot.Nodes {
		counts[n.Kind]++
	}
	writeJSON(w, 200, struct {
		SchemaVersion   int            `json:"schemaVersion"`
		AnalysisID      string         `json:"analysisId"`
		Status          string         `json:"status"`
		Project         graph.Project  `json:"project"`
		Statistics      map[string]int `json:"statistics"`
		DiagnosticCount int            `json:"diagnosticCount"`
	}{a.snapshot.SchemaVersion, a.snapshot.AnalysisID, a.snapshot.Status, a.snapshot.Project, counts, len(a.snapshot.Diagnostics)})
}
func (a *api) graph(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset := 0
	var err error
	if q.Get("offset") != "" {
		offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || offset < 0 {
			fail(w, 400, "invalid_query", "offset must be non-negative")
			return
		}
	}
	for _, key := range []string{"includeExternal", "exportedOnly", "includePossible"} {
		if v := q.Get(key); v != "" && v != "true" && v != "false" {
			fail(w, 400, "invalid_query", key+" must be true or false")
			return
		}
	}
	depth := 0
	if value := q.Get("depth"); value != "" {
		depth, err = strconv.Atoi(value)
		if err != nil || depth < 1 || depth > 10 {
			fail(w, 400, "invalid_query", "depth must be between 1 and 10")
			return
		}
	}
	view, err := a.snapshot.View(graph.ViewOptions{Level: q.Get("level"), RootID: q.Get("rootId"), Depth: depth, IncludePossible: q.Get("includePossible") == "true", PackageID: q.Get("packageId"), GroupID: q.Get("groupId"), Query: q.Get("q"), Kind: q.Get("kind"), Method: q.Get("method"), IncludeExternal: q.Get("includeExternal") == "true", ExportedOnly: q.Get("exportedOnly") == "true", Offset: offset})
	if err != nil {
		fail(w, 400, "invalid_query", err.Error())
		return
	}
	writeJSON(w, 200, view)
}
func (a *api) search(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	out := []graph.Node{}
	total := 0
	if q != "" {
		for _, n := range a.snapshot.Nodes {
			if n.Kind == "module" || n.Kind == "directory" || n.Kind == "externalPackage" {
				continue
			}
			if strings.Contains(strings.ToLower(n.Label+" "+n.Attributes.ImportPath), q) {
				total++
				if len(out) < graph.MaxViewNodes {
					out = append(out, n)
				}
			}
		}
	}
	writeJSON(w, 200, struct {
		Nodes []graph.Node `json:"nodes"`
		Total int          `json:"total"`
	}{out, total})
}
func (a *api) file(w http.ResponseWriter, r *http.Request) {
	f, ok := a.snapshot.File(chi.URLParam(r, "id"))
	if !ok {
		fail(w, 404, "file_not_found", "Source file not found")
		return
	}
	b, err := project.ReadFile(a.snapshot.Project.Root, f.RelativePath)
	if err != nil || graph.Hash(b) != f.ContentHash {
		fail(w, 409, "source_changed", "Source changed or is unavailable. Restart codevisual analyze.")
		return
	}
	writeJSON(w, 200, struct {
		ID           string `json:"id"`
		RelativePath string `json:"relativePath"`
		Content      string `json:"content"`
	}{f.ID, f.RelativePath, string(b)})
}
func serveSPA(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		fail(w, 405, "method_not_allowed", "Only GET requests are supported")
		return
	}
	sub, _ := fs.Sub(assets, "webdist")
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." || name == "" {
		name = "index.html"
	}
	if _, err := fs.Stat(sub, name); err != nil {
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/"
		r = clone
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}

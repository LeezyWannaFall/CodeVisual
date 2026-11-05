package httproutes

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"codevisual/internal/graph"
	"codevisual/internal/project"
)

func analyzeSource(t *testing.T, source string) graph.AnalysisResult {
	t.Helper()
	root := t.TempDir()
	name := filepath.Join(root, "routes.go")
	if err := os.WriteFile(name, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	p := &project.Project{Project: graph.Project{Root: root, ModulePath: "example.test/app"}, Packages: []project.Package{{ImportPath: "example.test/app", Name: "app", Fset: fset, Syntax: []*ast.File{file}}}, SourceHashes: map[string]string{name: graph.Hash([]byte(source))}}
	result, err := (Analyzer{}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestRoutesMountsAndMiddleware(t *testing.T) {
	r := analyzeSource(t, `package app
import router "github.com/go-chi/chi/v5"
func Routes() router.Router {
 r := router.NewRouter()
 r.Use(Logging)
 r.Get("/health", Health)
 r.Route("/v1", func(api router.Router) {
  api.Use(Auth)
  api.Get("/users/{id}", Show)
  api.With(Admin).Post("/users", Create)
  api.Group(func(g router.Router) { g.Delete("/users/{id}", Delete) })
 })
 r.Mount("/admin", AdminRoutes())
 Register(r)
 r.Put(dynamicPath, Update)
 return r
}
func AdminRoutes() router.Router { r:=router.NewRouter();r.Get("/stats", Stats);return r }
func Register(r router.Router) { r.Options("/options", Options) }
func Fake() { other.Get("/not-a-route", Handle) }
`)
	routes := map[string]*graph.HTTPRoute{}
	for _, n := range r.Nodes {
		if n.Source == nil || n.Source.StartLine < 1 {
			t.Fatal("missing source")
		}
		routes[n.Label] = n.Attributes.HTTP
	}
	if len(routes) != 6 {
		t.Fatalf("wrong routes: %+v", routes)
	}
	for _, label := range []string{"GET /health", "GET /v1/users/{id}", "POST /v1/users", "DELETE /v1/users/{id}", "GET /admin/stats", "OPTIONS /options"} {
		if routes[label] == nil {
			t.Fatalf("missing %s", label)
		}
	}
	if got := strings.Join(routes["POST /v1/users"].Middleware, ","); got != "Logging,Auth,Admin" {
		t.Fatalf("wrong middleware: %s", got)
	}
	if got := strings.Join(routes["GET /admin/stats"].Middleware, ","); got != "Logging" {
		t.Fatalf("mount inherited wrong middleware: %s", got)
	}
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "http_dynamic_path" {
		t.Fatalf("missing dynamic diagnostic: %+v", r.Diagnostics)
	}
}
func TestUnknownMountBranchesAndUnrelatedMethods(t *testing.T) {
	r := analyzeSource(t, `package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router {
 r:=chi.NewRouter()
 r.Mount("/unknown", external())
 if enabled { r.Get("/conditional", H) }
 for _,p:=range paths {r.Get(p,H)}
 r.Get("/known", H)
 return r
}
func unrelated(){ if flag { foo() }; other.Get("/fake",H) }
`)
	if len(r.Nodes) != 1 || r.Nodes[0].Label != "GET /known" {
		t.Fatalf("invented routes: %+v", r.Nodes)
	}
	if len(r.Diagnostics) != 3 {
		t.Fatalf("expected three limitations: %+v", r.Diagnostics)
	}
}

func TestTypedProjectAndMountedBuilderVariable(t *testing.T) {
	p, err := project.Discover(context.Background(), "../../../testdata/explorers", nil)
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
	if len(r.Nodes) != 3 || len(r.Diagnostics) != 0 {
		t.Fatalf("wrong typed registrations: %+v %+v", r.Nodes, r.Diagnostics)
	}
	found := false
	for _, n := range r.Nodes {
		if n.Label == "GET /api/users/{id}" {
			found = true
			if n.Attributes.HTTP.Handler != "ShowUser" || strings.Join(n.Attributes.HTTP.Middleware, ",") != "Logging" {
				t.Fatalf("wrong registration: %+v", n)
			}
		}
	}
	if !found {
		t.Fatal("variable-based Mount not resolved")
	}
}

func TestLocalRouterMountedTwice(t *testing.T) {
	r := analyzeSource(t, `package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router {
 r := chi.NewRouter()
 r.Use(Logging)
 sub := chi.NewRouter()
 sub.Use(Auth)
 sub.Get("/users", H)
 r.Mount("/api", sub)
 r.Mount("/v2", sub)
 return r
}`)
	if len(r.Diagnostics) != 0 || len(r.Nodes) != 2 {
		t.Fatalf("routes=%+v diagnostics=%+v", r.Nodes, r.Diagnostics)
	}
	paths := map[string]bool{}
	for _, n := range r.Nodes {
		paths[n.Attributes.HTTP.Path] = true
		if got := strings.Join(n.Attributes.HTTP.Middleware, ","); got != "Logging,Auth" {
			t.Fatalf("middleware=%s", got)
		}
	}
	if !paths["/api/users"] || !paths["/v2/users"] {
		t.Fatalf("paths=%v", paths)
	}
}

func TestReturnedRouterExtendedWithoutMount(t *testing.T) {
	for _, declaration := range []string{"r := Base()", "var r = Base()"} {
		r := analyzeSource(t, strings.Replace(`package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router {
 DECLARATION
 r.Get("/extra", H)
 return r
}
func Base() chi.Router {
 r := chi.NewRouter()
 r.Use(Auth)
 r.Get("/base", H)
 return r
}`, "DECLARATION", declaration, 1))
		if len(r.Diagnostics) != 0 || len(r.Nodes) != 2 {
			t.Fatalf("routes=%+v diagnostics=%+v", r.Nodes, r.Diagnostics)
		}
		paths := map[string]bool{}
		for _, n := range r.Nodes {
			paths[n.Attributes.HTTP.Path] = true
			if got := strings.Join(n.Attributes.HTTP.Middleware, ","); got != "Auth" {
				t.Fatalf("middleware=%s", got)
			}
		}
		if !paths["/base"] || !paths["/extra"] {
			t.Fatalf("paths=%v", paths)
		}
	}
}

func TestNestedMountsAndBuilderExtensions(t *testing.T) {
	r := analyzeSource(t, `package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router {
 r := chi.NewRouter()
 r.Use(Logging)
 sub := Base()
 sub.Get("/extra", H)
 middle := chi.NewRouter()
 middle.Use(Auth)
 middle.Mount("/v1", sub)
 r.Mount("/api", middle)
 return r
}
func Base() chi.Router { r := chi.NewRouter(); r.Get("/base", H); return r }
`)
	if len(r.Nodes) != 2 || len(r.Diagnostics) != 0 {
		t.Fatalf("routes=%+v diagnostics=%+v", r.Nodes, r.Diagnostics)
	}
	for _, n := range r.Nodes {
		if n.Label != "GET /api/v1/base" && n.Label != "GET /api/v1/extra" {
			t.Fatalf("wrong path: %s", n.Label)
		}
		if got := strings.Join(n.Attributes.HTTP.Middleware, ","); got != "Logging,Auth" {
			t.Fatalf("middleware=%s", got)
		}
	}
}

func TestUnresolvedBuilderExplainsPartialAnalysis(t *testing.T) {
	r := analyzeSource(t, `package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router { r := chi.NewRouter(); r.Mount("/api", Dynamic()); return r }
func Dynamic() chi.Router { if enabled { return chi.NewRouter() }; return unknown }
`)
	codes := map[string]bool{}
	for _, d := range r.Diagnostics {
		codes[d.Code] = true
	}
	if !codes["http_builder_return"] || !codes["http_unresolved_mount"] {
		t.Fatalf("missing diagnostics: %+v", r.Diagnostics)
	}
	if len(r.Nodes) != 0 {
		t.Fatalf("invented routes: %+v", r.Nodes)
	}
}

func TestCyclicMountIsDiagnosed(t *testing.T) {
	r := analyzeSource(t, `package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router { r := chi.NewRouter(); r.Get("/ok", H); r.Mount("/loop", r); return r }
`)
	if len(r.Nodes) != 1 || r.Nodes[0].Label != "GET /ok" || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "http_recursive_mount" {
		t.Fatalf("routes=%+v diagnostics=%+v", r.Nodes, r.Diagnostics)
	}
}

func TestTypedReturnedRouterAndLocalMount(t *testing.T) {
	root := t.TempDir()
	stub, err := filepath.Abs("../../../testdata/explorers/chistub")
	if err != nil {
		t.Fatal(err)
	}
	metadata := "module example.test/app\n\ngo 1.26.0\n\nrequire github.com/go-chi/chi/v5 v5.0.0\nreplace github.com/go-chi/chi/v5 => " + strconv.Quote(filepath.ToSlash(stub)) + "\n"
	source := `package app
import (
 "net/http"
 "github.com/go-chi/chi/v5"
)
func Routes() chi.Router {
 r := chi.NewRouter()
 r.Use(Logging)
 sub := Base()
 sub.Get("/extra", H)
 r.Mount("/api", sub)
 return r
}
func Base() chi.Router { r := chi.NewRouter(); r.Use(Auth); r.Get("/base", H); return r }
func H(http.ResponseWriter, *http.Request) {}
func Auth(next http.Handler) http.Handler { return next }
func Logging(next http.Handler) http.Handler { return next }
`
	for name, content := range map[string]string{"go.mod": metadata, "routes.go": source} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Discover(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(p.Diagnostics) != 0 {
		t.Fatalf("load diagnostics: %+v", p.Diagnostics)
	}
	result, err := (Analyzer{}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 2 || len(result.Diagnostics) != 0 {
		t.Fatalf("routes=%+v diagnostics=%+v", result.Nodes, result.Diagnostics)
	}
	for _, n := range result.Nodes {
		route := n.Attributes.HTTP
		if route.Path != "/api/base" && route.Path != "/api/extra" {
			t.Fatalf("wrong path: %s", route.Path)
		}
		if route.HandlerID == "" || route.HandlerResolution != "static" || route.HandlerSource == nil {
			t.Fatalf("handler not resolved: %+v", route)
		}
		if got := strings.Join(route.Middleware, ","); got != "Logging,Auth" {
			t.Fatalf("middleware=%s", got)
		}
		if len(route.MiddlewareTargets) != 2 {
			t.Fatalf("middleware targets=%+v", route.MiddlewareTargets)
		}
		for _, target := range route.MiddlewareTargets {
			if target.NodeID == "" || target.Source == nil {
				t.Fatalf("middleware target not resolved: %+v", target)
			}
		}
	}
}

func TestReturnedRouteCallbackKeepsRegistrations(t *testing.T) {
	r := analyzeSource(t, `package app
import "github.com/go-chi/chi/v5"
func Routes() chi.Router {
 r := chi.NewRouter()
 return r.Route("/api", func(sub chi.Router) { sub.Get("/users", H) })
}`)
	if len(r.Nodes) != 1 || r.Nodes[0].Label != "GET /api/users" {
		t.Fatalf("routes=%+v diagnostics=%+v", r.Nodes, r.Diagnostics)
	}
}

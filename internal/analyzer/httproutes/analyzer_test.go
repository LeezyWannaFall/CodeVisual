package httproutes

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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

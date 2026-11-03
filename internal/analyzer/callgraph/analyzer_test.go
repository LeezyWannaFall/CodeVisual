package callgraph_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codevisual/internal/analyzer/callgraph"
	goanalyzer "codevisual/internal/analyzer/golang"
	"codevisual/internal/analyzer/httproutes"
	"codevisual/internal/analyzer/symbols"
	"codevisual/internal/graph"
	"codevisual/internal/project"
)

func analyze(t *testing.T, source string) *graph.Snapshot {
	t.Helper()
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		name := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	stub, err := os.ReadFile("../../../testdata/explorers/chistub/chi.go")
	if err != nil {
		t.Fatal(err)
	}
	write("go.mod", "module example.test/app\n\ngo 1.26.0\nrequire github.com/go-chi/chi/v5 v5.0.0\nreplace github.com/go-chi/chi/v5 => ./chi\n")
	write("chi/go.mod", "module github.com/go-chi/chi/v5\n\ngo 1.26.0\n")
	write("chi/chi.go", string(stub))
	write("app.go", source)
	p, err := project.Discover(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	index := symbols.New(p)
	goResult, err := (goanalyzer.Analyzer{}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	routes, err := (httproutes.Analyzer{Index: index}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := (callgraph.Analyzer{Index: index}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := graph.NewSnapshot(p.Project, graph.Merge(goResult, routes, calls, callgraph.ConnectRoutes(routes, index)))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
func route(t *testing.T, s *graph.Snapshot) graph.Node {
	t.Helper()
	for _, n := range s.Nodes {
		if n.Kind == "httpRoute" {
			return n
		}
	}
	t.Fatal("missing route")
	return graph.Node{}
}
func TestStaticCallsInterfaceBoundaryAndCandidates(t *testing.T) {
	s := analyze(t, `package app
import("context";"net/http";"github.com/go-chi/chi/v5")
type Repo interface{ Save(context.Context) error }
type Store struct{}
func(*Store)Save(context.Context)error{return nil}
type WrongStore struct{}
func(*WrongStore)Save(string)error{return nil}
type Service struct{repo Repo}
func(s *Service)Save(ctx context.Context){_ = s.repo.Save(ctx)}
func Handler(w http.ResponseWriter,r *http.Request){s:=&Service{};s.Save(r.Context());alias:=leaf;alias();go leaf();defer leaf();unused:=func(){hidden()};_ = unused}
func leaf(){}
func hidden(){}
func Routes()chi.Router{r:=chi.NewRouter();r.Use(Logging);r.Get("/save",Handler);return r}
func Logging(next http.Handler)http.Handler{return next}
`)
	r := route(t, s)
	if r.Attributes.HTTP.HandlerResolution != "static" || r.Attributes.HTTP.HandlerID == "" {
		t.Fatalf("handler unresolved: %+v", r)
	}
	handler, ok := s.Node(r.Attributes.HTTP.HandlerID)
	if !ok || handler.Label != "Handler" || handler.Source == nil {
		t.Fatal("handler source link missing")
	}
	if len(r.Attributes.HTTP.MiddlewareTargets) != 1 || r.Attributes.HTTP.MiddlewareTargets[0].NodeID == "" {
		t.Fatal("missing middleware source")
	}
	view, err := s.View(graph.ViewOptions{Level: "flow", RootID: r.ID, Depth: 8})
	if err != nil {
		t.Fatal(err)
	}
	hasInterface, hasConcrete := false, false
	for _, n := range view.Nodes {
		if n.Kind == "interfaceMethod" {
			hasInterface = true
		}
		if strings.Contains(n.Label, "Store.Save") {
			hasConcrete = true
		}
		if n.Label == "hidden" {
			t.Fatal("uninvoked closure body attributed to handler")
		}
	}
	if !hasInterface || hasConcrete || view.Flow.OmittedPossible != 1 {
		t.Fatalf("interface should stop by default: %+v", view)
	}
	possible, err := s.View(graph.ViewOptions{Level: "flow", RootID: r.ID, Depth: 8, IncludePossible: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range possible.Nodes {
		if strings.Contains(n.Label, "WrongStore") {
			t.Fatal("incompatible implementation included")
		}
		if n.Kind == "method" && strings.Contains(n.Label, "Store.Save") {
			found = true
			if !n.Attributes.Possible {
				t.Fatal("candidate not marked uncertain")
			}
		}
	}
	if !found {
		t.Fatal("compatible implementation missing")
	}
	for _, n := range s.Nodes {
		if n.Kind == "method" && n.Label == "Save" && n.Attributes.Possible {
			t.Fatal("projection mutated snapshot")
		}
	}
	invocation := map[string]bool{}
	for _, e := range view.Edges {
		if e.SourceID == handler.ID {
			invocation[e.Invocation] = true
		}
		if e.Evidence == nil {
			t.Fatalf("call has no source evidence: %+v", e)
		}
	}
	if !invocation["go"] || !invocation["defer"] || !invocation["call"] {
		t.Fatalf("missing invocation modes: %v", invocation)
	}
}
func TestHandlerFactoriesClosuresAndDynamicValues(t *testing.T) {
	s := analyze(t, `package app
import("net/http";"github.com/go-chi/chi/v5")
func Factory()http.HandlerFunc{return func(w http.ResponseWriter,r *http.Request){leaf()}}
func leaf(){}
func Routes()chi.Router{r:=chi.NewRouter();r.Get("/factory",Factory());return r}
`)
	r := route(t, s)
	h := r.Attributes.HTTP
	if h.HandlerResolution != "factory_return" || h.HandlerFactoryID == "" {
		t.Fatalf("factory confused with handler: %+v", h)
	}
	handler, _ := s.Node(h.HandlerID)
	if handler.Kind != "closure" {
		t.Fatal("factory must target returned closure")
	}
	view, err := s.View(graph.ViewOptions{Level: "flow", RootID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range view.Nodes {
		if n.ID == h.HandlerFactoryID {
			t.Fatal("registration factory included as request-time call")
		}
	}
	s = analyze(t, `package app
import("net/http";"github.com/go-chi/chi/v5")
var callback func()
func Handler(http.ResponseWriter,*http.Request){callback()}
func Routes()chi.Router{r:=chi.NewRouter();r.Get("/dynamic",Handler);return r}
`)
	view, err = s.View(graph.ViewOptions{Level: "flow", RootID: route(t, s).ID})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range view.Nodes {
		if n.Kind == "dynamicCall" {
			found = true
		}
	}
	if !found {
		t.Fatal("dynamic call should be an explicit terminal boundary")
	}
}
func TestNamedCallbacksMiddlewareHelperAndDelegatedRouter(t *testing.T) {
	s := analyze(t, `package app
import("net/http";"github.com/go-chi/chi/v5")
type R = chi.Router
func Outer()chi.Router{return Routes()}
func Routes()chi.Router{r:=chi.NewRouter();middleware(r);r.Route("/v1",Register);return r}
func middleware(r chi.Router){alias:=r;alias.Use(Logging)}
func Register(r R){r.Get("/items",Handler)}
func Handler(http.ResponseWriter,*http.Request){}
func Logging(next http.Handler)http.Handler{return next}
`)
	routes := []graph.Node{}
	for _, n := range s.Nodes {
		if n.Kind == "httpRoute" {
			routes = append(routes, n)
		}
	}
	if len(routes) != 1 || routes[0].Label != "GET /v1/items" {
		t.Fatalf("named callback or delegation duplicated routes: %+v", routes)
	}
	h := routes[0].Attributes.HTTP
	if len(h.Middleware) != 1 || h.Middleware[0] != "Logging" || h.HandlerID == "" {
		t.Fatalf("middleware helper lost: %+v", h)
	}
}

func TestGlobalClosuresAdaptersAndFunctionValues(t *testing.T) {
	s := analyze(t, `package app
import("net/http";"github.com/go-chi/chi/v5")
var Global=http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){leaf()})
func leaf(){}
func Routes()chi.Router{r:=chi.NewRouter();r.Get("/global",Global);return r}
`)
	r := route(t, s)
	handler, _ := s.Node(r.Attributes.HTTP.HandlerID)
	if handler.Kind != "closure" || handler.Source == nil {
		t.Fatalf("global closure unresolved: %+v", handler)
	}
	s = analyze(t, `package app
import("net/http";"github.com/go-chi/chi/v5")
type Adapter func()
func(a Adapter)ServeHTTP(http.ResponseWriter,*http.Request){a()}
func leaf(){}
func Routes()chi.Router{r:=chi.NewRouter();r.Handle("/adapter",Adapter(leaf));return r}
`)
	r = route(t, s)
	handler, _ = s.Node(r.Attributes.HTTP.HandlerID)
	if handler.Kind != "method" || handler.Label != "ServeHTTP" {
		t.Fatalf("custom adapter bypassed: %+v", handler)
	}
	v, err := s.View(graph.ViewOptions{Level: "flow", RootID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range v.Edges {
		if e.SourceID == handler.ID && e.TargetID == handler.ID {
			t.Fatal("function value invocation confused with ServeHTTP")
		}
	}
	found := false
	for _, n := range v.Nodes {
		if n.Kind == "dynamicCall" {
			found = true
		}
	}
	if !found {
		t.Fatal("adapter's function receiver should remain a dynamic function value")
	}
}

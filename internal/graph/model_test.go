package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSnapshotValidation(t *testing.T) {
	n := Node{ID: "a", Kind: "package", Label: "a"}
	for _, r := range []AnalysisResult{{Nodes: []Node{n, n}}, {Nodes: []Node{n}, Edges: []Edge{Link("imports", "a", "missing", nil)}}, {Nodes: []Node{{ID: "a", ParentID: "missing"}}}} {
		if _, err := NewSnapshot(Project{}, r); err == nil {
			t.Fatal("invalid graph accepted")
		}
	}
}
func TestProjectionAggregationAndBounds(t *testing.T) {
	module := ID("module", "m")
	nodes := []Node{{ID: module, Kind: "module"}, {ID: "left", Kind: "directory", ParentID: module}, {ID: "right", Kind: "directory", ParentID: module}}
	edges := []Edge{}
	for i := 0; i < 205; i++ {
		id := fmt.Sprintf("p%03d", i)
		parent := "left"
		if i%2 == 1 {
			parent = "right"
		}
		nodes = append(nodes, Node{ID: id, Kind: "package", Label: id, ParentID: parent})
		if i > 0 {
			edges = append(edges, Link("imports", "p000", id, nil))
		}
	}
	s, err := NewSnapshot(Project{ModulePath: "m"}, AnalysisResult{Nodes: nodes, Edges: edges})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.View(ViewOptions{Level: "packages"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Nodes) != 200 || v.TotalNodes != 205 || v.HiddenNodes != 5 {
		t.Fatalf("unbounded view: %+v", v)
	}
	visible := map[string]bool{}
	for _, n := range v.Nodes {
		visible[n.ID] = true
	}
	for _, e := range v.Edges {
		if !visible[e.SourceID] || !visible[e.TargetID] {
			t.Fatal("dangling visible edge")
		}
	}
	v, _ = s.View(ViewOptions{Level: "packages", Offset: 200})
	if len(v.Nodes) != 5 {
		t.Fatal("pagination failed")
	}
	v, _ = s.View(ViewOptions{Level: "directories"})
	if len(v.Nodes) != 2 || len(v.Edges) != 1 || v.Edges[0].Count != 102 || !v.Edges[0].Derived {
		t.Fatalf("wrong grouping: %+v", v)
	}
	v, _ = s.View(ViewOptions{Level: "packages", GroupID: "right"})
	if v.TotalNodes != 102 {
		t.Fatal("group filter failed")
	}
	if _, err := s.View(ViewOptions{Level: "symbols", PackageID: "missing"}); err == nil {
		t.Fatal("invalid package accepted")
	}
}
func TestGraphJSONContract(t *testing.T) {
	v := View{AnalysisID: "fixture", Nodes: []Node{{ID: "package_demo", Kind: "package", Label: "demo", Attributes: Attributes{ImportPath: "example.test/demo"}}}, Edges: []Edge{}, TotalNodes: 1}
	actual, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("../../testdata/contracts/graph.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != strings.TrimSpace(string(expected)) {
		t.Fatalf("contract changed:\n%s", actual)
	}
}

func TestExplorerProjections(t *testing.T) {
	r := AnalysisResult{Nodes: []Node{
		{ID: "p", Kind: "package", Label: "p"},
		{ID: "f", Kind: "function", PackageID: "p", Label: "Handler"},
		{ID: "r", Kind: "httpRoute", ParentID: "p", Label: "GET /users", Attributes: Attributes{HTTP: &HTTPRoute{Method: "GET", Path: "/users", Handler: "Handler"}}},
		{ID: "t", Kind: "table", Label: "users", Attributes: Attributes{Table: &SQLTable{Name: "users"}}},
		{ID: "i", Kind: "index", Label: "users_idx", Attributes: Attributes{Index: &SQLIndex{Name: "users_idx", Table: "users"}}},
	}}
	s, err := NewSnapshot(Project{}, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		options ViewOptions
		count   int
	}{{ViewOptions{Level: "http"}, 1}, {ViewOptions{Level: "http", Method: "POST"}, 0}, {ViewOptions{Level: "database"}, 2}, {ViewOptions{Level: "database", Kind: "index"}, 1}, {ViewOptions{Level: "symbols", PackageID: "p"}, 1}, {ViewOptions{Level: "packages"}, 1}} {
		v, err := s.View(tc.options)
		if err != nil || len(v.Nodes) != tc.count {
			t.Fatalf("projection %+v: %+v %v", tc.options, v, err)
		}
	}
}

func TestMixedLanguageAndContainerContract(t *testing.T) {
	content, err := os.ReadFile("../../testdata/contracts/mixed.json")
	if err != nil {
		t.Fatal(err)
	}
	var view View
	if err = json.Unmarshal(content, &view); err != nil {
		t.Fatal(err)
	}
	if view.Nodes[0].Attributes.Language != "python" || view.Nodes[1].Kind != "class" || view.Nodes[2].Attributes.Container.Contents[0].PackageIDs[0] != "python_module" {
		t.Fatalf("contract lost fields: %+v", view)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var original, roundTrip any
	if err = json.Unmarshal(content, &original); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, roundTrip) {
		t.Fatalf("contract changed: %s", encoded)
	}
}

func TestContainerContentMustReferenceAnExistingPackage(t *testing.T) {
	_, err := NewSnapshot(Project{}, AnalysisResult{Nodes: []Node{{ID: "c", Kind: "container", Attributes: Attributes{Container: &Container{Contents: []ContainerContent{{PackageIDs: []string{"missing"}}}}}}}})
	if err == nil {
		t.Fatal("accepted dangling container contents")
	}
}

func TestContainerViewHidesDefaultNetwork(t *testing.T) {
	s, err := NewSnapshot(Project{}, AnalysisResult{Nodes: []Node{
		{ID: "service", Kind: "container", Label: "app"},
		{ID: "default", Kind: "containerNetwork", Label: "default"},
		{ID: "custom", Kind: "containerNetwork", Label: "custom"},
	}, Edges: []Edge{Link("uses_network", "service", "default", nil), Link("uses_network", "service", "custom", nil)}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.View(ViewOptions{Level: "containers"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Nodes) != 2 || len(v.Edges) != 1 {
		t.Fatalf("unexpected view: %+v", v)
	}
}

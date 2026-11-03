package graph

import (
	"fmt"
	"testing"
)

func TestFlowLimitsCyclesAndUncertainty(t *testing.T) {
	nodes := []Node{{ID: "route", Kind: "httpRoute", Label: "GET /"}, {ID: "handler", Kind: "function"}, {ID: "iface", Kind: "interfaceMethod"}, {ID: "impl", Kind: "method"}, {ID: "external", Kind: "externalCall", Attributes: Attributes{External: true}}}
	edges := []Edge{Link("handles", "route", "handler", nil), Link("interface_call", "handler", "iface", nil), Link("may_dispatch", "iface", "impl", nil), Link("calls", "impl", "handler", nil), Link("calls", "handler", "external", nil)}
	s, err := NewSnapshot(Project{}, AnalysisResult{Nodes: nodes, Edges: edges})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.View(ViewOptions{Level: "flow", RootID: "route", Depth: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Nodes) != 3 || v.Flow.OmittedExternal != 1 || v.Flow.OmittedPossible != 1 {
		t.Fatalf("default boundaries: %+v", v)
	}
	v, err = s.View(ViewOptions{Level: "flow", RootID: "route", Depth: 6, IncludePossible: true, IncludeExternal: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Nodes) != 5 || len(v.Edges) != 5 {
		t.Fatalf("cycle or candidate lost: %+v", v)
	}
	for _, n := range v.Nodes {
		if n.ID == "handler" && n.Attributes.Possible {
			t.Fatal("confirmed node downgraded by a cycle")
		}
		if n.ID == "impl" && !n.Attributes.Possible {
			t.Fatal("candidate confidence missing")
		}
	}
	v, _ = s.View(ViewOptions{Level: "flow", RootID: "route", Depth: 1})
	if !v.Flow.Truncated || len(v.Nodes) != 2 {
		t.Fatal("depth bound not enforced")
	}
	for _, o := range []ViewOptions{{Level: "flow"}, {Level: "flow", RootID: "missing"}, {Level: "flow", RootID: "route", Depth: 11}} {
		if _, err = s.View(o); err == nil {
			t.Errorf("accepted invalid query %+v", o)
		}
	}
}
func TestFlowNodeCap(t *testing.T) {
	nodes := []Node{{ID: "root", Kind: "function"}}
	edges := []Edge{}
	for i := 0; i < 250; i++ {
		id := fmt.Sprint(i)
		nodes = append(nodes, Node{ID: id, Kind: "function"})
		edges = append(edges, Link("calls", "root", id, nil))
	}
	s, err := NewSnapshot(Project{}, AnalysisResult{Nodes: nodes, Edges: edges})
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.View(ViewOptions{Level: "flow", RootID: "root", Depth: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Nodes) != MaxViewNodes || !v.Flow.Truncated {
		t.Fatal("unbounded flow")
	}
	ids := map[string]bool{}
	for _, n := range v.Nodes {
		ids[n.ID] = true
	}
	for _, e := range v.Edges {
		if !ids[e.SourceID] || !ids[e.TargetID] {
			t.Fatal("dangling edge in bounded flow")
		}
	}
}

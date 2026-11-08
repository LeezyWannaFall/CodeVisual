package graph

import "testing"

func TestRebasePreservesClosureAndSourceReferences(t *testing.T) {
	source := &SourceRange{FileID: "file", StartLine: 1, EndLine: 1}
	original := AnalysisResult{Files: []File{{ID: "file", RelativePath: "main.go", ContentHash: "hash"}}, Nodes: []Node{
		{ID: "module", Kind: "module"},
		{ID: "pkg", Kind: "package", ParentID: "module", Attributes: Attributes{RelativePath: "."}},
		{ID: "closure", Kind: "closure", ParentID: "pkg", PackageID: "pkg", Source: source},
		{ID: "route", Kind: "httpRoute", ParentID: "pkg", Source: source, Attributes: Attributes{HTTP: &HTTPRoute{HandlerID: "closure", HandlerSource: source, MiddlewareTargets: []TargetRef{{NodeID: "closure", Source: source}}}}},
	}, Edges: []Edge{Link("handles", "route", "closure", source)}}
	result := Rebase(original, "services/backend", "root")
	closureID := ID("closure", "services/backend:closure")
	for _, n := range result.Nodes {
		if n.Kind == "httpRoute" {
			if n.Attributes.HTTP.HandlerID != closureID || n.Attributes.HTTP.MiddlewareTargets[0].NodeID != closureID || n.Source.FileID != ID("file", "services/backend/main.go") {
				t.Fatalf("lost refs: %+v", n)
			}
		}
	}
	result.Nodes = append(result.Nodes, Node{ID: "root", Kind: "module"})
	if _, err := NewSnapshot(Project{}, result); err != nil {
		t.Fatal(err)
	}
}

// Package callgraph records statically selected calls and explicit uncertainty
// boundaries. It does not claim runtime reachability or execution order.
package callgraph

import (
	"context"
	"go/ast"

	"codevisual/internal/analyzer/symbols"
	"codevisual/internal/graph"
	"codevisual/internal/project"
)

type Analyzer struct{ Index *symbols.Index }

func (a Analyzer) Analyze(ctx context.Context, p *project.Project) (graph.AnalysisResult, error) {
	index := a.Index
	if index == nil {
		index = symbols.New(p)
	}
	result := graph.AnalysisResult{Nodes: []graph.Node{}, Edges: []graph.Edge{}, Files: []graph.File{}, Diagnostics: []graph.Diagnostic{}}
	edges := map[string]graph.Edge{}
	for _, function := range index.Functions {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if function.Body == nil {
			continue
		}
		invocations := map[*ast.CallExpr]string{}
		ast.Inspect(function.Body, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			switch n := n.(type) {
			case *ast.GoStmt:
				invocations[n.Call] = "go"
			case *ast.DeferStmt:
				invocations[n.Call] = "defer"
			}
			return true
		})
		ast.Inspect(function.Body, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			target := index.Resolve(function.Package, call.Fun)
			if target.Resolution == "not_callable" {
				return true
			}
			if target.ID == "" {
				target = index.Boundary(function.Package, call.Fun)
			}
			kind := "calls"
			if target.Resolution == "interface" {
				kind = "interface_call"
			}
			if target.Resolution == "dynamic" {
				kind = "dynamic_call"
			}
			edge := graph.Link(kind, function.ID, target.ID, index.Source(function.Package, call))
			edge.Resolution = target.Resolution
			edge.Invocation = invocations[call]
			if edge.Invocation == "" {
				edge.Invocation = "call"
			}
			edge.ID = graph.ID("edge", edge.ID+":"+edge.Invocation)
			if old, ok := edges[edge.ID]; ok {
				old.Count++
				edges[edge.ID] = old
			} else {
				edge.Count = 1
				edges[edge.ID] = edge
			}
			return true
		})
	}
	for id, node := range index.Extra {
		if node.Kind != "interfaceMethod" {
			continue
		}
		for _, candidate := range index.Candidates(id) {
			edge := graph.Link("may_dispatch", id, candidate, node.Source)
			edge.Resolution = "possible"
			edges[edge.ID] = edge
		}
	}
	for _, node := range index.Extra {
		result.Nodes = append(result.Nodes, node)
	}
	for _, edge := range edges {
		result.Edges = append(result.Edges, edge)
	}
	for _, file := range index.Files {
		result.Files = append(result.Files, file)
	}
	return result, nil
}

// ConnectRoutes links registrations to handler/middleware targets after all
// analyzers have populated the shared symbol index. Factories are not handlers.
func ConnectRoutes(routes graph.AnalysisResult, index *symbols.Index) graph.AnalysisResult {
	result := graph.AnalysisResult{}
	for _, node := range routes.Nodes {
		route := node.Attributes.HTTP
		if route == nil {
			continue
		}
		if route.HandlerID != "" {
			edge := graph.Link("handles", node.ID, route.HandlerID, node.Source)
			edge.Resolution = route.HandlerResolution
			result.Edges = append(result.Edges, edge)
		}
		seen := map[string]bool{}
		for _, target := range route.MiddlewareTargets {
			if target.NodeID == "" || seen[target.NodeID] {
				continue
			}
			seen[target.NodeID] = true
			edge := graph.Link("uses_middleware", node.ID, target.NodeID, node.Source)
			edge.Resolution = target.Resolution
			result.Edges = append(result.Edges, edge)
		}
	}
	return result
}

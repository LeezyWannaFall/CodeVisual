package graph

import (
	"fmt"
	"sort"
)

type FlowInfo struct {
	RootID          string `json:"rootId"`
	Depth           int    `json:"depth"`
	Truncated       bool   `json:"truncated"`
	OmittedExternal int    `json:"omittedExternal"`
	OmittedPossible int    `json:"omittedPossible"`
}

func flowEdge(kind string) bool {
	switch kind {
	case "handles", "calls", "interface_call", "dynamic_call", "may_dispatch":
		return true
	}
	return false
}

// Flow is bounded breadth-first reachability over source call sites, not an
// ordered execution trace. Candidate interface dispatch is opt-in.
func (s *Snapshot) Flow(o ViewOptions) (View, error) {
	root, ok := s.Node(o.RootID)
	if !ok {
		return View{}, fmt.Errorf("rootId must identify a route or callable symbol")
	}
	switch root.Kind {
	case "httpRoute", "function", "method", "closure", "interfaceMethod":
	default:
		return View{}, fmt.Errorf("rootId must identify a route or callable symbol")
	}
	depth := o.Depth
	if depth == 0 {
		depth = 6
	}
	if depth < 1 || depth > 10 {
		return View{}, fmt.Errorf("depth must be between 1 and 10")
	}
	info := &FlowInfo{RootID: root.ID, Depth: depth}
	view := View{AnalysisID: s.AnalysisID, Nodes: []Node{}, Edges: []Edge{}, Flow: info}
	type visit struct {
		id       string
		distance int
		possible bool
	}
	queue := []visit{{root.ID, 0, false}}
	seen := map[string]visit{root.ID: queue[0]}
	edgeSet := map[string]Edge{}
	omittedExternal := map[string]bool{}
	omittedPossible := map[string]bool{}
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		if current.distance >= depth {
			for _, e := range s.outgoing[current.id] {
				if flowEdge(e.Kind) {
					info.Truncated = true
					break
				}
			}
			continue
		}
		for _, edge := range s.outgoing[current.id] {
			if !flowEdge(edge.Kind) {
				continue
			}
			if edge.Kind == "may_dispatch" && !o.IncludePossible {
				omittedPossible[edge.ID] = true
				continue
			}
			target, ok := s.Node(edge.TargetID)
			if !ok {
				continue
			}
			if target.Attributes.External && !o.IncludeExternal && edge.Kind != "handles" {
				omittedExternal[target.ID] = true
				continue
			}
			possible := current.possible || edge.Kind == "may_dispatch"
			previous, exists := seen[target.ID]
			if !exists {
				if len(seen) >= MaxViewNodes {
					info.Truncated = true
					continue
				}
				next := visit{target.ID, current.distance + 1, possible}
				seen[target.ID] = next
				queue = append(queue, next)
			} else if previous.possible && !possible {
				next := visit{target.ID, current.distance + 1, false}
				seen[target.ID] = next
				queue = append(queue, next)
			}
			edgeSet[edge.ID] = edge
		}
	}
	for id, state := range seen {
		node, _ := s.Node(id)
		node.Attributes.Possible = state.possible
		if node.Kind == "method" && node.Attributes.Receiver != "" {
			node.Label = node.Attributes.Receiver + "." + node.Label
		}
		view.Nodes = append(view.Nodes, node)
	}
	for _, edge := range edgeSet {
		view.Edges = append(view.Edges, edge)
	}
	sort.Slice(view.Nodes, func(i, j int) bool {
		a, b := seen[view.Nodes[i].ID], seen[view.Nodes[j].ID]
		if a.distance != b.distance {
			return a.distance < b.distance
		}
		return view.Nodes[i].ID < view.Nodes[j].ID
	})
	sort.Slice(view.Edges, func(i, j int) bool { return view.Edges[i].ID < view.Edges[j].ID })
	info.OmittedExternal = len(omittedExternal)
	info.OmittedPossible = len(omittedPossible)
	view.TotalNodes = len(view.Nodes)
	return view, nil
}

package graph

import (
	"fmt"
	"sort"
	"strings"
)

// View computes a bounded projection. Pagination is deterministic and never invents imports.
func (s *Snapshot) View(o ViewOptions) (View, error) {
	if o.Level == "flow" {
		return s.Flow(o)
	}
	if o.Level == "" {
		o.Level = "packages"
	}
	if o.Offset < 0 {
		return View{}, fmt.Errorf("offset must be non-negative")
	}
	if o.Level != "packages" && o.Level != "directories" && o.Level != "symbols" && o.Level != "http" && o.Level != "database" {
		return View{}, fmt.Errorf("unknown graph level")
	}
	if o.Level == "symbols" {
		n, ok := s.Node(o.PackageID)
		if !ok || n.Kind != "package" {
			return View{}, fmt.Errorf("packageId must identify a package")
		}
	}
	if o.GroupID != "" {
		n, ok := s.Node(o.GroupID)
		if !ok || (n.Kind != "directory" && n.Kind != "module") {
			return View{}, fmt.Errorf("groupId must identify a directory or module")
		}
	}
	candidates := []Node{}
	for _, n := range s.Nodes {
		match := false
		switch o.Level {
		case "packages":
			match = n.Kind == "package" || (o.IncludeExternal && n.Kind == "externalPackage")
			if o.GroupID != "" && n.Kind == "package" {
				match = match && s.descends(n.ID, o.GroupID)
			}
		case "symbols":
			match = n.PackageID == o.PackageID && n.Kind != "package" && n.Kind != "httpRoute"
		case "http":
			match = n.Kind == "httpRoute"
		case "database":
			match = n.Kind == "table" || n.Kind == "index"
		case "directories":
			parent := o.GroupID
			if parent == "" {
				parent = ID("module", s.Project.ModulePath)
			}
			match = n.ParentID == parent && (n.Kind == "directory" || n.Kind == "package")
		}
		if !match {
			continue
		}
		if o.Method != "" && (n.Attributes.HTTP == nil || n.Attributes.HTTP.Method != o.Method) {
			continue
		}
		if o.Kind != "" && n.Kind != o.Kind {
			continue
		}
		if o.ExportedOnly && !n.Attributes.Exported {
			continue
		}
		if o.Query != "" && !strings.Contains(strings.ToLower(n.Label+" "+n.Attributes.ImportPath+" "+n.Attributes.RelativePath+" "+n.Attributes.Signature), strings.ToLower(o.Query)) {
			continue
		}
		candidates = append(candidates, n)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		return a.Attributes.RelativePath+a.Label+a.ID < b.Attributes.RelativePath+b.Label+b.ID
	})
	v := View{AnalysisID: s.AnalysisID, Nodes: []Node{}, Edges: []Edge{}, TotalNodes: len(candidates)}
	start := min(o.Offset, len(candidates))
	end := min(start+MaxViewNodes, len(candidates))
	v.Nodes = append(v.Nodes, candidates[start:end]...)
	v.HiddenNodes = len(candidates) - len(v.Nodes)
	visible := map[string]bool{}
	for _, n := range v.Nodes {
		visible[n.ID] = true
	}
	if o.Level == "directories" {
		// Map each package to its visible ancestor. Internal imports disappear.
		owners := map[string]string{}
		for _, n := range s.Nodes {
			if n.Kind != "package" {
				continue
			}
			for cur := n.ID; cur != ""; {
				if visible[cur] {
					owners[n.ID] = cur
					break
				}
				p, ok := s.Node(cur)
				if !ok {
					break
				}
				cur = p.ParentID
			}
		}
		aggregated := map[string]Edge{}
		for _, e := range s.Edges {
			if e.Kind != "imports" {
				continue
			}
			a, b := owners[e.SourceID], owners[e.TargetID]
			if a == "" || b == "" || a == b {
				continue
			}
			key := a + ":" + b
			item, ok := aggregated[key]
			if !ok {
				item = Link("imports", a, b, nil)
				item.Derived = true
			}
			item.Count++
			aggregated[key] = item
		}
		for _, e := range aggregated {
			v.Edges = append(v.Edges, e)
		}
	} else {
		for _, e := range s.Edges {
			if !flowEdge(e.Kind) && e.Kind != "uses_middleware" && visible[e.SourceID] && visible[e.TargetID] {
				v.Edges = append(v.Edges, e)
			}
		}
	}
	sort.Slice(v.Edges, func(i, j int) bool { return v.Edges[i].ID < v.Edges[j].ID })
	return v, nil
}
func (s *Snapshot) descends(id, parent string) bool {
	for id != "" {
		if id == parent {
			return true
		}
		n, ok := s.Node(id)
		if !ok {
			return false
		}
		id = n.ParentID
	}
	return false
}

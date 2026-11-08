package graph

import (
	"path"
)

// Rebase places a separately loaded Go module under a selected workspace root.
// Package/symbol IDs stay semantic; file/directory IDs use workspace-relative paths.
func Rebase(r AnalysisResult, prefix, rootID string) AnalysisResult {
	ids := map[string]string{}
	for i := range r.Files {
		f := &r.Files[i]
		old := f.ID
		f.RelativePath = path.Join(prefix, f.RelativePath)
		f.ID = ID("file", f.RelativePath)
		ids[old] = f.ID
	}
	for _, n := range r.Nodes {
		if n.Kind == "closure" || n.Kind == "dynamicCall" {
			ids[n.ID] = ID(n.Kind, prefix+":"+n.ID)
		}
		if n.Kind == "module" {
			ids[n.ID] = ID("directory", prefix)
		}
		if n.Kind == "directory" {
			ids[n.ID] = ID("directory", path.Join(prefix, n.Attributes.RelativePath))
		}
	}
	remap := func(id string) string {
		if value, ok := ids[id]; ok {
			return value
		}
		return id
	}
	source := func(s *SourceRange) *SourceRange {
		if s == nil {
			return nil
		}
		v := *s
		v.FileID = remap(v.FileID)
		return &v
	}
	for i := range r.Nodes {
		n := &r.Nodes[i]
		n.ID = remap(n.ID)
		n.ParentID = remap(n.ParentID)
		n.Source = source(n.Source)
		if n.Kind == "module" {
			n.Kind = "directory"
			n.Label = path.Base(prefix)
			n.ParentID = ID("directory", path.Dir(prefix))
			if path.Dir(prefix) == "." {
				n.ParentID = rootID
			}
			n.Attributes.RelativePath = prefix
		} else if n.Attributes.RelativePath != "" {
			n.Attributes.RelativePath = path.Join(prefix, n.Attributes.RelativePath)
		}
		if n.Attributes.HTTP != nil {
			value := *n.Attributes.HTTP
			value.HandlerSource = source(value.HandlerSource)
			value.HandlerID = remap(value.HandlerID)
			value.HandlerFactoryID = remap(value.HandlerFactoryID)
			value.MiddlewareTargets = append([]TargetRef{}, value.MiddlewareTargets...)
			for j := range value.MiddlewareTargets {
				value.MiddlewareTargets[j].Source = source(value.MiddlewareTargets[j].Source)
				value.MiddlewareTargets[j].NodeID = remap(value.MiddlewareTargets[j].NodeID)
				value.MiddlewareTargets[j].FactoryID = remap(value.MiddlewareTargets[j].FactoryID)
			}
			n.Attributes.HTTP = &value
		}
	}
	for i := range r.Edges {
		e := &r.Edges[i]
		e.SourceID = remap(e.SourceID)
		e.TargetID = remap(e.TargetID)
		e.Evidence = source(e.Evidence)
		e.ID = Link(e.Kind, e.SourceID, e.TargetID, e.Evidence).ID
		if e.Invocation != "" {
			e.ID = ID("edge", e.ID+":"+e.Invocation)
		}
	}
	for dir := prefix; dir != "." && dir != ""; dir = path.Dir(dir) {
		parent := ID("directory", path.Dir(dir))
		if path.Dir(dir) == "." {
			parent = rootID
		}
		if dir != prefix {
			r.Nodes = append(r.Nodes, Node{ID: ID("directory", dir), Kind: "directory", Label: path.Base(dir), ParentID: parent, Attributes: Attributes{RelativePath: dir}})
		}
		r.Edges = append(r.Edges, Link("contains", parent, ID("directory", dir), nil))
	}
	for i := range r.Diagnostics {
		r.Diagnostics[i].Source = source(r.Diagnostics[i].Source)
		r.Diagnostics[i].NodeID = remap(r.Diagnostics[i].NodeID)
	}
	return r
}

package graph

import "reflect"

// HTTPRoute records a source registration, not an observed runtime endpoint.
type TargetRef struct {
	Source     *SourceRange `json:"source,omitempty"`
	Name       string       `json:"name"`
	NodeID     string       `json:"nodeId,omitempty"`
	Resolution string       `json:"resolution"`
	FactoryID  string       `json:"factoryId,omitempty"`
}

type HTTPRoute struct {
	HandlerSource     *SourceRange `json:"handlerSource,omitempty"`
	HandlerID         string       `json:"handlerId,omitempty"`
	HandlerResolution string       `json:"handlerResolution,omitempty"`
	HandlerFactoryID  string       `json:"handlerFactoryId,omitempty"`
	MiddlewareTargets []TargetRef  `json:"middlewareTargets"`
	Method            string       `json:"method"`
	Path              string       `json:"path"`
	Handler           string       `json:"handler"`
	Middleware        []string     `json:"middleware"`
	Scope             string       `json:"scope"`
}
type SQLColumn struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Constraints string `json:"constraints,omitempty"`
}
type SQLReference struct {
	Columns       string `json:"columns"`
	Table         string `json:"table"`
	TargetColumns string `json:"targetColumns"`
}

// SQLTable is one CREATE TABLE declaration, not a simulated migrated schema.
type SQLTable struct {
	Name        string         `json:"name"`
	Columns     []SQLColumn    `json:"columns"`
	Constraints []string       `json:"constraints"`
	References  []SQLReference `json:"references"`
}
type SQLIndex struct {
	Name       string `json:"name"`
	Table      string `json:"table"`
	Unique     bool   `json:"unique"`
	Definition string `json:"definition"`
}

// Merge combines independent analyzer output, accepting shared source files only
// when their content hashes agree. NewSnapshot validates the resulting graph.
func Merge(results ...AnalysisResult) AnalysisResult {
	out := AnalysisResult{Nodes: []Node{}, Edges: []Edge{}, Files: []File{}, Diagnostics: []Diagnostic{}}
	files := map[string]File{}
	nodes := map[string]Node{}
	edges := map[string]Edge{}
	for _, r := range results {
		for _, n := range r.Nodes {
			if old, ok := nodes[n.ID]; ok {
				if !reflect.DeepEqual(old, n) {
					out.Diagnostics = append(out.Diagnostics, Diagnostic{Severity: "warning", Code: "node_conflict", Message: "Conflicting analyzer declarations: " + n.ID})
				}
				continue
			}
			nodes[n.ID] = n
			out.Nodes = append(out.Nodes, n)
		}
		for _, e := range r.Edges {
			if _, ok := edges[e.ID]; !ok {
				edges[e.ID] = e
				out.Edges = append(out.Edges, e)
			}
		}
		out.Diagnostics = append(out.Diagnostics, r.Diagnostics...)
		for _, f := range r.Files {
			if previous, ok := files[f.ID]; ok {
				if previous.ContentHash != f.ContentHash {
					out.Diagnostics = append(out.Diagnostics, Diagnostic{Severity: "warning", Code: "source_changed", Message: "Source changed between analyzers: " + f.RelativePath})
				}
				continue
			}
			files[f.ID] = f
			out.Files = append(out.Files, f)
		}
	}
	return out
}

// Container describes declarations, not a running container or an inspected image.
type Container struct {
	Type            string             `json:"type"`
	Image           string             `json:"image,omitempty"`
	BuildContext    string             `json:"buildContext,omitempty"`
	Dockerfile      string             `json:"dockerfile,omitempty"`
	Stage           string             `json:"stage,omitempty"`
	Workdir         string             `json:"workdir,omitempty"`
	Command         string             `json:"command,omitempty"`
	Entrypoint      string             `json:"entrypoint,omitempty"`
	Ports           []string           `json:"ports"`
	Volumes         []string           `json:"volumes"`
	Networks        []string           `json:"networks"`
	EnvironmentKeys []string           `json:"environmentKeys"`
	Contents        []ContainerContent `json:"contents"`
}
type ContainerContent struct {
	Kind       string   `json:"kind"`
	Source     string   `json:"source"`
	Target     string   `json:"target"`
	From       string   `json:"from,omitempty"`
	PackageIDs []string `json:"packageIds"`
}

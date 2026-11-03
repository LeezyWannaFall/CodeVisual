package graph

// HTTPRoute records a source registration, not an observed runtime endpoint.
type HTTPRoute struct {
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Handler    string   `json:"handler"`
	Middleware []string `json:"middleware"`
	Scope      string   `json:"scope"`
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
	for _, r := range results {
		out.Nodes = append(out.Nodes, r.Nodes...)
		out.Edges = append(out.Edges, r.Edges...)
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

// Package graph defines the language-independent analysis snapshot and its API views.
package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const MaxViewNodes = 200

type BuildContext struct {
	GOOS               string   `json:"goos"`
	GOARCH             string   `json:"goarch"`
	Tags               []string `json:"tags"`
	CGOEnabled         bool     `json:"cgoEnabled"`
	ConfigurationKnown bool     `json:"configurationKnown"`
}
type Project struct {
	Root         string       `json:"-"`
	ModulePath   string       `json:"modulePath"`
	RootName     string       `json:"rootName"`
	BuildContext BuildContext `json:"buildContext"`
}
type SourceRange struct {
	FileID      string `json:"fileId"`
	StartLine   int    `json:"startLine"`
	StartColumn int    `json:"startColumn"`
	EndLine     int    `json:"endLine"`
	EndColumn   int    `json:"endColumn"`
}
type Attributes struct {
	HTTP          *HTTPRoute `json:"http,omitempty"`
	Table         *SQLTable  `json:"table,omitempty"`
	Index         *SQLIndex  `json:"index,omitempty"`
	Unresolved    bool       `json:"unresolved,omitempty"`
	ImportPath    string     `json:"importPath,omitempty"`
	RelativePath  string     `json:"relativePath,omitempty"`
	Exported      bool       `json:"exported"`
	Standard      bool       `json:"standard,omitempty"`
	Signature     string     `json:"signature,omitempty"`
	Documentation string     `json:"documentation,omitempty"`
	Receiver      string     `json:"receiver,omitempty"`
	Members       []string   `json:"members,omitempty"`
}
type Node struct {
	ID         string       `json:"id"`
	Kind       string       `json:"kind"`
	Label      string       `json:"label"`
	ParentID   string       `json:"parentId,omitempty"`
	PackageID  string       `json:"packageId,omitempty"`
	Source     *SourceRange `json:"source,omitempty"`
	Attributes Attributes   `json:"attributes"`
}
type Edge struct {
	ID       string       `json:"id"`
	Kind     string       `json:"kind"`
	SourceID string       `json:"sourceId"`
	TargetID string       `json:"targetId"`
	Evidence *SourceRange `json:"evidence,omitempty"`
	Count    int          `json:"count,omitempty"`
	Derived  bool         `json:"derived,omitempty"`
}
type Diagnostic struct {
	Severity string       `json:"severity"`
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	NodeID   string       `json:"nodeId,omitempty"`
	Source   *SourceRange `json:"source,omitempty"`
}
type File struct {
	ID           string `json:"id"`
	RelativePath string `json:"relativePath"`
	ContentHash  string `json:"contentHash"`
}
type AnalysisResult struct {
	Nodes       []Node
	Edges       []Edge
	Diagnostics []Diagnostic
	Files       []File
}
type Snapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	AnalysisID    string       `json:"analysisId"`
	Status        string       `json:"status"`
	Project       Project      `json:"project"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Files         []File       `json:"files"`
	nodes         map[string]Node
	files         map[string]File
}

// ID uses semantic keys rather than checkout paths or source line numbers.
func ID(kind, key string) string { return kind + "_" + Hash([]byte(key))[:24] }
func Hash(b []byte) string       { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Link(kind, from, to string, evidence *SourceRange) Edge {
	return Edge{ID: ID("edge", kind+":"+from+":"+to), Kind: kind, SourceID: from, TargetID: to, Evidence: evidence}
}
func NewSnapshot(p Project, r AnalysisResult) (*Snapshot, error) {
	s := &Snapshot{SchemaVersion: 1, Status: "complete", Project: p, Nodes: r.Nodes, Edges: r.Edges, Diagnostics: r.Diagnostics, Files: r.Files, nodes: map[string]Node{}, files: map[string]File{}}
	if s.Nodes == nil {
		s.Nodes = []Node{}
	}
	if s.Edges == nil {
		s.Edges = []Edge{}
	}
	if s.Diagnostics == nil {
		s.Diagnostics = []Diagnostic{}
	}
	if s.Files == nil {
		s.Files = []File{}
	}
	for _, n := range s.Nodes {
		if _, ok := s.nodes[n.ID]; ok {
			return nil, fmt.Errorf("duplicate node %s", n.ID)
		}
		s.nodes[n.ID] = n
	}
	edgeIDs := map[string]bool{}
	for _, e := range s.Edges {
		if edgeIDs[e.ID] {
			return nil, fmt.Errorf("duplicate edge %s", e.ID)
		}
		edgeIDs[e.ID] = true
		if _, ok := s.nodes[e.SourceID]; !ok {
			return nil, fmt.Errorf("missing edge source %s", e.SourceID)
		}
		if _, ok := s.nodes[e.TargetID]; !ok {
			return nil, fmt.Errorf("missing edge target %s", e.TargetID)
		}
	}
	for _, f := range s.Files {
		if _, ok := s.files[f.ID]; ok {
			return nil, fmt.Errorf("duplicate file %s", f.ID)
		}
		s.files[f.ID] = f
	}
	for _, n := range s.Nodes {
		if n.ParentID != "" {
			if _, ok := s.nodes[n.ParentID]; !ok {
				return nil, fmt.Errorf("missing parent %s", n.ParentID)
			}
		}
		if n.Source != nil {
			if _, ok := s.files[n.Source.FileID]; !ok {
				return nil, fmt.Errorf("missing source file %s", n.Source.FileID)
			}
		}
	}
	sort.Slice(s.Nodes, func(i, j int) bool { return s.Nodes[i].ID < s.Nodes[j].ID })
	sort.Slice(s.Edges, func(i, j int) bool { return s.Edges[i].ID < s.Edges[j].ID })
	sort.Slice(s.Files, func(i, j int) bool { return s.Files[i].RelativePath < s.Files[j].RelativePath })
	sort.Slice(s.Diagnostics, func(i, j int) bool {
		a, b := s.Diagnostics[i], s.Diagnostics[j]
		return a.Code+a.NodeID+a.Message < b.Code+b.NodeID+b.Message
	})
	for _, d := range s.Diagnostics {
		if d.Severity == "error" || d.Severity == "warning" {
			s.Status = "partial"
		}
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	s.AnalysisID = Hash(encoded)[:24]
	return s, nil
}
func (s *Snapshot) Node(id string) (Node, bool) { n, ok := s.nodes[id]; return n, ok }
func (s *Snapshot) File(id string) (File, bool) { f, ok := s.files[id]; return f, ok }

type View struct {
	AnalysisID  string `json:"analysisId"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
	TotalNodes  int    `json:"totalNodes"`
	HiddenNodes int    `json:"hiddenNodes"`
}
type ViewOptions struct {
	Level, PackageID, GroupID, Query, Kind, Method string
	IncludeExternal, ExportedOnly                  bool
	Offset                                         int
}

func (s *Snapshot) Tree() []Node {
	out := []Node{}
	for _, n := range s.Nodes {
		if n.Kind == "module" || n.Kind == "directory" || n.Kind == "package" {
			out = append(out, n)
		}
	}
	return out
}

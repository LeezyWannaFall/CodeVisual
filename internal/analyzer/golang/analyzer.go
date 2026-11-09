// Package golang extracts only source-backed declarations and import relationships.
package golang

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"codevisual/internal/analyzer"
	"codevisual/internal/graph"
	"codevisual/internal/project"
)

type Analyzer struct{}

var _ analyzer.Analyzer = Analyzer{}

func (Analyzer) Analyze(ctx context.Context, p *project.Project) (graph.AnalysisResult, error) {
	r := graph.AnalysisResult{Nodes: []graph.Node{}, Edges: []graph.Edge{}, Files: []graph.File{}, Diagnostics: append([]graph.Diagnostic{}, p.Diagnostics...)}
	nodes := map[string]bool{}
	edges := map[string]bool{}
	files := map[string]graph.File{}
	addNode := func(n graph.Node) {
		if !nodes[n.ID] {
			r.Nodes = append(r.Nodes, n)
			nodes[n.ID] = true
		}
	}
	addEdge := func(e graph.Edge) {
		if !edges[e.ID] {
			r.Edges = append(r.Edges, e)
			edges[e.ID] = true
		}
	}
	moduleID := graph.ID("module", p.ModulePath)
	addNode(graph.Node{ID: moduleID, Kind: "module", Label: p.ModulePath})
	var directory func(string) string
	directory = func(rel string) string {
		if rel == "." || rel == "" {
			return moduleID
		}
		rel = filepath.ToSlash(rel)
		id := graph.ID("directory", rel)
		if nodes[id] {
			return id
		}
		parent := directory(filepath.Dir(rel))
		addNode(graph.Node{ID: id, Kind: "directory", Label: filepath.Base(rel), ParentID: parent, Attributes: graph.Attributes{RelativePath: rel}})
		addEdge(graph.Link("contains", parent, id, nil))
		return id
	}
	internal := map[string]string{}
	for _, pkg := range p.Packages {
		id := graph.ID("package", pkg.ImportPath)
		internal[pkg.ImportPath] = id
		parent := directory(pkg.Directory)
		addNode(graph.Node{ID: id, Kind: "package", Label: pkg.Name, ParentID: parent, Attributes: graph.Attributes{ImportPath: pkg.ImportPath, RelativePath: filepath.ToSlash(pkg.Directory)}})
		addEdge(graph.Link("contains", parent, id, nil))
	}
	for _, pkg := range p.Packages {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		packageID := internal[pkg.ImportPath]
		location := func(start, end token.Pos) *graph.SourceRange {
			a, b := pkg.Fset.Position(start), pkg.Fset.Position(end)
			rel, err := filepath.Rel(p.Root, a.Filename)
			if err != nil || !filepath.IsLocal(rel) {
				return nil
			}
			rel = filepath.ToSlash(rel)
			id := graph.ID("file", rel)
			if _, ok := files[id]; !ok {
				content, e := project.ReadFile(p.Root, rel)
				if e != nil {
					r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "source_unreadable", Message: e.Error()})
					return nil
				}
				hash := p.SourceHashes[a.Filename]
				if hash == "" {
					hash = graph.Hash(content)
				}
				if hash != graph.Hash(content) {
					r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "source_changed", Message: "Source changed during analysis: " + rel})
				}
				files[id] = graph.File{ID: id, RelativePath: rel, ContentHash: hash}
			}
			return &graph.SourceRange{FileID: id, StartLine: a.Line, StartColumn: a.Column, EndLine: b.Line, EndColumn: b.Column}
		}
		typeIDs := map[string]string{}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
					for _, spec := range gen.Specs {
						t := spec.(*ast.TypeSpec)
						kind := typeKind(t)
						typeIDs[t.Name.Name] = graph.ID(kind, pkg.ImportPath+":"+t.Name.Name)
					}
				}
			}
		}
		seen := map[string]int{}
		for _, file := range pkg.Syntax {
			if file.Name.Name != pkg.Name {
				r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "mixed_packages", Message: "Different package name: " + file.Name.Name, NodeID: packageID})
			}
			for _, im := range file.Imports {
				path, e := strconv.Unquote(im.Path.Value)
				if e != nil || path == "C" {
					continue
				}
				target, ok := internal[path]
				if !ok && p.InternalImport(path) {
					target = graph.ID("package", path)
					addNode(graph.Node{ID: target, Kind: "package", Label: filepath.Base(path), ParentID: moduleID, Attributes: graph.Attributes{ImportPath: path, Unresolved: true}})
					addEdge(graph.Link("contains", moduleID, target, nil))
					r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "unresolved_package", Message: "No source available in the current build for imported package " + path, NodeID: target})
				} else if !ok {
					target = graph.ID("externalPackage", path)
					standard := false
					if bp, err := build.Default.Import(path, p.Root, build.FindOnly); err == nil {
						standard = bp.Goroot
					}
					addNode(graph.Node{ID: target, Kind: "externalPackage", Label: path, Attributes: graph.Attributes{ImportPath: path, Standard: standard}})
				}
				addEdge(graph.Link("imports", packageID, target, location(im.Pos(), im.End())))
			}
			for _, decl := range file.Decls {
				if ctx.Err() != nil {
					return r, ctx.Err()
				}
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Name.Name == "_" {
						continue
					}
					kind := "function"
					receiver := ""
					receiverType := ""
					if d.Recv != nil && len(d.Recv.List) > 0 {
						kind = "method"
						receiver = printed(pkg.Fset, d.Recv.List[0].Type)
						receiverType = baseType(d.Recv.List[0].Type)
					}
					key := pkg.ImportPath + ":" + receiverType + ":" + d.Name.Name
					if d.Name.Name == "init" {
						rel, _ := filepath.Rel(p.Root, pkg.Fset.Position(d.Pos()).Filename)
						key += ":" + filepath.ToSlash(rel)
					}
					seen[key]++
					if d.Name.Name == "init" || seen[key] > 1 {
						key += fmt.Sprintf(":%d", seen[key])
					}
					signatureDecl := *d
					signatureDecl.Body = nil
					signatureDecl.Doc = nil
					attrs := graph.Attributes{ImportPath: pkg.ImportPath, RelativePath: filepath.ToSlash(pkg.Directory), Exported: ast.IsExported(d.Name.Name), Receiver: receiver, Signature: printed(pkg.Fset, &signatureDecl), Documentation: d.Doc.Text()}
					if pkg.TypesInfo != nil {
						if obj := pkg.TypesInfo.Defs[d.Name]; obj != nil {
							if fn, ok := obj.(*types.Func); ok { // Access resolved types without requiring successful whole-project typing.
								if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
									attrs.Receiver = types.TypeString(sig.Recv().Type(), func(p *types.Package) string {
										if p.Path() == pkg.ImportPath {
											return ""
										}
										return p.Name()
									})
								}
							}
						}
					}
					n := graph.Node{ID: graph.ID(kind, key), Kind: kind, Label: d.Name.Name, ParentID: packageID, PackageID: packageID, Source: location(d.Pos(), d.End()), Attributes: attrs}
					addNode(n)
					addEdge(graph.Link("contains", packageID, n.ID, n.Source))
					if target := typeIDs[receiverType]; target != "" {
						addEdge(graph.Link("declares_method", target, n.ID, n.Source))
					}
				case *ast.GenDecl:
					if d.Tok != token.TYPE {
						continue
					}
					for _, spec := range d.Specs {
						t := spec.(*ast.TypeSpec)
						if t.Name.Name == "_" {
							continue
						}
						kind := typeKind(t)
						id := typeIDs[t.Name.Name]
						key := "type:" + t.Name.Name
						seen[key]++
						if seen[key] > 1 {
							id = graph.ID(kind, pkg.ImportPath+":"+t.Name.Name+fmt.Sprintf(":%d", seen[key]))
							r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "duplicate_declaration", Message: "Duplicate type " + t.Name.Name, NodeID: packageID})
						}
						doc := t.Doc.Text()
						if doc == "" {
							doc = d.Doc.Text()
						}
						attrs := graph.Attributes{Exported: ast.IsExported(t.Name.Name), Signature: "type " + printed(pkg.Fset, t), Documentation: doc, Members: []string{}}
						var fields *ast.FieldList
						switch x := t.Type.(type) {
						case *ast.StructType:
							fields = x.Fields
						case *ast.InterfaceType:
							fields = x.Methods
						}
						if fields != nil {
							for _, f := range fields.List {
								attrs.Members = append(attrs.Members, fieldString(pkg.Fset, f))
							}
						}
						n := graph.Node{ID: id, Kind: kind, Label: t.Name.Name, ParentID: packageID, PackageID: packageID, Source: location(t.Pos(), t.End()), Attributes: attrs}
						addNode(n)
						addEdge(graph.Link("contains", packageID, n.ID, n.Source))
					}
				}
			}
		}
	}
	for _, f := range files {
		r.Files = append(r.Files, f)
	}
	return r, nil
}
func typeKind(t *ast.TypeSpec) string {
	if t.Assign.IsValid() {
		return "type"
	}
	switch t.Type.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	}
	return "type"
}
func printed(fset *token.FileSet, n any) string {
	var b bytes.Buffer
	if err := format.Node(&b, fset, n); err != nil {
		return ""
	}
	return strings.TrimSpace(b.String())
}
func baseType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return baseType(t.X)
	case *ast.IndexExpr:
		return baseType(t.X)
	case *ast.IndexListExpr:
		return baseType(t.X)
	case *ast.ParenExpr:
		return baseType(t.X)
	}
	return ""
}

func fieldString(fset *token.FileSet, f *ast.Field) string {
	names := []string{}
	for _, name := range f.Names {
		names = append(names, name.Name)
	}
	prefix := ""
	if len(names) > 0 {
		prefix = strings.Join(names, ", ") + " "
	}
	value := prefix + printed(fset, f.Type)
	if f.Tag != nil {
		value += " " + f.Tag.Value
	}
	return value
}

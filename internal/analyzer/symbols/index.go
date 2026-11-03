// Package symbols shares source identities and conservative target resolution
// between HTTP registration and call analysis. It never evaluates project code.
package symbols

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"codevisual/internal/graph"
	"codevisual/internal/project"
)

type Function struct {
	ID      string
	Name    string
	Package *project.Package
	Decl    ast.Node
	Body    *ast.BlockStmt
	Object  *types.Func
	Parent  *Function
}
type Target struct{ ID, Resolution, FactoryID string }
type Index struct {
	Project          *project.Project
	Functions        []*Function
	Objects          map[types.Object]*Function
	Literals         map[*ast.FuncLit]*Function
	Named            map[string]*Function
	Extra            map[string]graph.Node
	Files            map[string]graph.File
	values           map[types.Object]ast.Expr
	valuePackages    map[types.Object]*project.Package
	assignments      map[types.Object]int
	interfaceSources map[types.Object]struct {
		pkg  *project.Package
		node ast.Node
	}
	candidates map[string][]string
}

func New(p *project.Project) *Index {
	x := &Index{Project: p, Objects: map[types.Object]*Function{}, Literals: map[*ast.FuncLit]*Function{}, Named: map[string]*Function{}, Extra: map[string]graph.Node{}, Files: map[string]graph.File{}, values: map[types.Object]ast.Expr{}, valuePackages: map[types.Object]*project.Package{}, assignments: map[types.Object]int{}, interfaceSources: map[types.Object]struct {
		pkg  *project.Package
		node ast.Node
	}{}, candidates: map[string][]string{}}
	for i := range p.Packages {
		pkg := &p.Packages[i]
		seen := map[string]int{}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				d, ok := decl.(*ast.FuncDecl)
				if !ok || d.Name.Name == "_" {
					continue
				}
				kind, receiver := "function", ""
				if d.Recv != nil && len(d.Recv.List) > 0 {
					kind = "method"
					receiver = Base(d.Recv.List[0].Type)
				}
				key := pkg.ImportPath + ":" + receiver + ":" + d.Name.Name
				if d.Name.Name == "init" {
					rel, _ := filepath.Rel(p.Root, pkg.Fset.Position(d.Pos()).Filename)
					key += ":" + filepath.ToSlash(rel)
				}
				seen[key]++
				if d.Name.Name == "init" || seen[key] > 1 {
					key += fmt.Sprintf(":%d", seen[key])
				}
				name := d.Name.Name
				if receiver != "" {
					name = receiver + "." + name
				}
				f := &Function{ID: graph.ID(kind, key), Name: name, Package: pkg, Decl: d, Body: d.Body}
				if pkg.TypesInfo != nil {
					f.Object, _ = pkg.TypesInfo.Defs[d.Name].(*types.Func)
					if f.Object != nil {
						x.Objects[f.Object.Origin()] = f
					}
				}
				x.Functions = append(x.Functions, f)
				if d.Recv == nil {
					x.Named[pkg.ImportPath+":"+d.Name.Name] = f
				}
			}
		}
	}
	// Each closure gets a source-backed boundary; its body belongs to that closure,
	// not to the enclosing function which merely creates it.
	declared := append([]*Function{}, x.Functions...)
	for i := range p.Packages {
		pkg := &p.Packages[i]
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if gen, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range gen.Specs {
						if value, ok := spec.(*ast.ValueSpec); ok {
							for _, expr := range value.Values {
								x.closures(&Function{ID: graph.ID("package", pkg.ImportPath), Package: pkg}, expr)
							}
						}
					}
				}
			}
		}
	}
	for _, f := range declared {
		if f.Body != nil {
			x.closures(f, f.Body)
		}
	}
	for i := range p.Packages {
		pkg := &p.Packages[i]
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.AssignStmt:
					for i, lhs := range n.Lhs {
						if id, ok := lhs.(*ast.Ident); ok {
							obj := pkg.TypesInfo.ObjectOf(id)
							if obj == nil {
								continue
							}
							x.assignments[obj]++
							if i < len(n.Rhs) {
								x.values[obj] = n.Rhs[i]
								x.valuePackages[obj] = pkg
							}
						}
					}
				case *ast.ValueSpec:
					for i, name := range n.Names {
						obj := pkg.TypesInfo.Defs[name]
						if obj != nil && i < len(n.Values) {
							x.assignments[obj]++
							x.values[obj] = n.Values[i]
							x.valuePackages[obj] = pkg
						}
					}
				case *ast.UnaryExpr:
					if n.Op == token.AND {
						if id, ok := n.X.(*ast.Ident); ok {
							if obj := pkg.TypesInfo.ObjectOf(id); obj != nil {
								x.assignments[obj] += 2
							}
						}
					}
				case *ast.IncDecStmt:
					if id, ok := n.X.(*ast.Ident); ok {
						if obj := pkg.TypesInfo.ObjectOf(id); obj != nil {
							x.assignments[obj]++
						}
					}
				case *ast.InterfaceType:
					for _, field := range n.Methods.List {
						for _, name := range field.Names {
							if obj := pkg.TypesInfo.Defs[name]; obj != nil {
								x.interfaceSources[obj] = struct {
									pkg  *project.Package
									node ast.Node
								}{pkg, field}
							}
						}
					}
				}
				return true
			})
		}
	}
	return x
}
func (x *Index) closures(parent *Function, n ast.Node) {
	if n == nil {
		return
	}
	ast.Inspect(n, func(n ast.Node) bool {
		literal, ok := n.(*ast.FuncLit)
		if !ok {
			return true
		}
		source := x.Source(parent.Package, literal)
		if source == nil {
			return false
		}
		id := graph.ID("closure", parent.Package.ImportPath+":"+source.FileID+":"+strconv.Itoa(source.StartLine)+":"+strconv.Itoa(source.StartColumn))
		f := &Function{ID: id, Name: fmt.Sprintf("inline function · L%d", source.StartLine), Package: parent.Package, Decl: literal, Body: literal.Body, Parent: parent}
		x.Literals[literal] = f
		x.Functions = append(x.Functions, f)
		x.Extra[id] = graph.Node{ID: id, Kind: "closure", Label: f.Name, ParentID: parent.ID, Source: source, Attributes: graph.Attributes{ImportPath: parent.Package.ImportPath, Signature: Print(parent.Package, literal.Type), Resolution: "static"}}
		x.closures(f, literal.Body)
		return false
	})
}
func (x *Index) Source(pkg *project.Package, n ast.Node) *graph.SourceRange {
	if n == nil {
		return nil
	}
	a, b := pkg.Fset.Position(n.Pos()), pkg.Fset.Position(n.End())
	rel, err := filepath.Rel(x.Project.Root, a.Filename)
	if err != nil || !filepath.IsLocal(rel) {
		return nil
	}
	rel = filepath.ToSlash(rel)
	id := graph.ID("file", rel)
	if _, ok := x.Files[id]; !ok {
		hash := x.Project.SourceHashes[a.Filename]
		if hash == "" {
			content, e := project.ReadFile(x.Project.Root, rel)
			if e != nil {
				return nil
			}
			hash = graph.Hash(content)
		}
		x.Files[id] = graph.File{ID: id, RelativePath: rel, ContentHash: hash}
	}
	return &graph.SourceRange{FileID: id, StartLine: a.Line, StartColumn: a.Column, EndLine: b.Line, EndColumn: b.Column}
}
func (x *Index) Function(pkg *project.Package, e ast.Expr) *Function {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return x.Function(pkg, e.X)
	case *ast.IndexExpr:
		return x.Function(pkg, e.X)
	case *ast.IndexListExpr:
		return x.Function(pkg, e.X)
	}
	if pkg.TypesInfo != nil {
		var obj types.Object
		switch e := e.(type) {
		case *ast.Ident:
			obj = pkg.TypesInfo.ObjectOf(e)
		case *ast.SelectorExpr:
			obj = pkg.TypesInfo.Uses[e.Sel]
		}
		if fn, ok := obj.(*types.Func); ok {
			return x.Objects[fn.Origin()]
		}
		if obj != nil {
			return nil
		}
	}
	if id, ok := e.(*ast.Ident); ok {
		if id.Obj != nil && id.Obj.Kind != ast.Fun {
			return nil
		}
		return x.Named[pkg.ImportPath+":"+id.Name]
	}
	return nil
}

// Resolve distinguishes a function reference from a factory invocation. A call
// expression in a handler position is NOT a request-time call to that factory.
func (x *Index) Resolve(pkg *project.Package, e ast.Expr) Target {
	return x.resolve(pkg, e, map[types.Object]bool{}, 0)
}
func (x *Index) resolve(pkg *project.Package, e ast.Expr, seen map[types.Object]bool, depth int) Target {
	if e == nil || depth > 12 {
		return Target{Resolution: "dynamic"}
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return x.resolve(pkg, e.X, seen, depth+1)
	case *ast.IndexExpr:
		return x.resolve(pkg, e.X, seen, depth+1)
	case *ast.IndexListExpr:
		return x.resolve(pkg, e.X, seen, depth+1)
	case *ast.FuncLit:
		if f := x.Literals[e]; f != nil {
			return Target{ID: f.ID, Resolution: "static"}
		}
	case *ast.CallExpr:
		if pkg.TypesInfo != nil {
			if tv, ok := pkg.TypesInfo.Types[e.Fun]; ok && tv.IsType() && len(e.Args) == 1 {
				return x.resolve(pkg, e.Args[0], seen, depth+1)
			}
		}
		if f := x.Function(pkg, e.Fun); f != nil {
			if returned := returnedFunction(f); returned != nil {
				t := x.resolve(f.Package, returned, seen, depth+1)
				t.FactoryID = f.ID
				if t.ID != "" {
					t.Resolution = "factory_return"
				}
				return t
			}
			return Target{Resolution: "factory", FactoryID: f.ID}
		}
		return Target{Resolution: "dynamic"}
	}
	if pkg.TypesInfo != nil {
		if sel, ok := e.(*ast.SelectorExpr); ok {
			if selection := pkg.TypesInfo.Selections[sel]; selection != nil {
				if iface, ok := selection.Recv().Underlying().(*types.Interface); ok {
					if obj, ok := selection.Obj().(*types.Func); ok {
						return x.interfaceTarget(pkg, e, obj, selection.Recv(), iface)
					}
				}
			}
		}
		var obj types.Object
		switch e := e.(type) {
		case *ast.Ident:
			obj = pkg.TypesInfo.ObjectOf(e)
		case *ast.SelectorExpr:
			obj = pkg.TypesInfo.Uses[e.Sel]
		}
		switch obj := obj.(type) {
		case *types.Func:
			if f := x.Objects[obj.Origin()]; f != nil {
				return Target{ID: f.ID, Resolution: "static"}
			}
			name := obj.FullName()
			id := graph.ID("externalCall", name)
			importPath := ""
			if obj.Pkg() != nil {
				importPath = obj.Pkg().Path()
			}
			x.Extra[id] = graph.Node{ID: id, Kind: "externalCall", Label: name, Attributes: graph.Attributes{ImportPath: importPath, Signature: types.TypeString(obj.Type(), qualifier), External: true, Resolution: "external"}}
			return Target{ID: id, Resolution: "external"}
		case *types.Var:
			if x.assignments[obj] == 1 && !seen[obj] {
				if value := x.values[obj]; value != nil {
					seen[obj] = true
					return x.resolve(x.valuePackages[obj], value, seen, depth+1)
				}
			}
			return Target{Resolution: "dynamic"}
		case *types.Builtin, *types.TypeName:
			return Target{Resolution: "not_callable"}
		}
	}
	if f := x.Function(pkg, e); f != nil {
		return Target{ID: f.ID, Resolution: "static"}
	}
	return Target{Resolution: "dynamic"}
}
func returnedFunction(f *Function) ast.Expr {
	var result ast.Expr
	count := 0
	if f.Body == nil {
		return nil
	}
	ast.Inspect(f.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if r, ok := n.(*ast.ReturnStmt); ok {
			count++
			if len(r.Results) == 1 {
				result = r.Results[0]
			}
		}
		return true
	})
	if count != 1 {
		return nil
	}
	switch result.(type) {
	case *ast.FuncLit, *ast.Ident, *ast.SelectorExpr, *ast.CallExpr:
		return result
	}
	return nil
}
func (x *Index) interfaceTarget(pkg *project.Package, e ast.Expr, obj *types.Func, receiver types.Type, iface *types.Interface) Target {
	name := types.TypeString(receiver, qualifier) + "." + obj.Name()
	id := graph.ID("interfaceMethod", name)
	path := ""
	if obj.Pkg() != nil {
		path = obj.Pkg().Path()
	}
	external := !x.local(path)
	node := graph.Node{ID: id, Kind: "interfaceMethod", Label: types.TypeString(receiver, func(p *types.Package) string { return p.Name() }) + "." + obj.Name(), Attributes: graph.Attributes{ImportPath: path, Receiver: types.TypeString(receiver, qualifier), Signature: types.TypeString(obj.Type(), qualifier), External: external, Resolution: "interface"}}
	if src, ok := x.interfaceSources[obj.Origin()]; ok {
		node.Source = x.Source(src.pkg, src.node)
	}
	x.Extra[id] = node
	if _, ok := x.candidates[id]; !ok {
		candidates := []string{}
		for _, f := range x.Functions {
			if f.Object == nil || f.Object.Name() != obj.Name() {
				continue
			}
			sig, ok := f.Object.Type().(*types.Signature)
			if !ok || sig.Recv() == nil {
				continue
			}
			recv := sig.Recv().Type()
			implements := types.Implements(recv, iface)
			if _, pointer := recv.(*types.Pointer); !pointer {
				implements = implements || types.Implements(types.NewPointer(recv), iface)
			}
			if implements {
				candidates = append(candidates, f.ID)
			}
		}
		x.candidates[id] = candidates
	}
	return Target{ID: id, Resolution: "interface"}
}
func (x *Index) Candidates(id string) []string { return x.candidates[id] }
func (x *Index) local(path string) bool {
	return path == x.Project.ModulePath || strings.HasPrefix(path, x.Project.ModulePath+"/")
}
func (x *Index) Boundary(pkg *project.Package, e ast.Expr) Target {
	source := x.Source(pkg, e)
	key := pkg.ImportPath + ":" + Print(pkg, e)
	if source != nil {
		key += ":" + source.FileID + ":" + strconv.Itoa(source.StartLine) + ":" + strconv.Itoa(source.StartColumn)
	}
	id := graph.ID("dynamicCall", key)
	x.Extra[id] = graph.Node{ID: id, Kind: "dynamicCall", Label: Print(pkg, e), Source: source, Attributes: graph.Attributes{ImportPath: pkg.ImportPath, Signature: "Runtime target is not statically resolved", Resolution: "dynamic"}}
	return Target{ID: id, Resolution: "dynamic"}
}
func Print(pkg *project.Package, n ast.Node) string {
	var b bytes.Buffer
	_ = format.Node(&b, pkg.Fset, n)
	return b.String()
}
func Base(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return Base(e.X)
	case *ast.IndexExpr:
		return Base(e.X)
	case *ast.IndexListExpr:
		return Base(e.X)
	case *ast.ParenExpr:
		return Base(e.X)
	}
	return ""
}
func qualifier(p *types.Package) string { return p.Path() }

// Handler distinguishes http.Handler objects from HandlerFunc registrations.
// Custom ServeHTTP adapters are not silently replaced by their underlying func.
func (x *Index) Handler(pkg *project.Package, e ast.Expr, object bool) Target {
	if !object {
		return x.Resolve(pkg, e)
	}
	if pkg.TypesInfo == nil {
		return Target{Resolution: "dynamic"}
	}
	typ := pkg.TypesInfo.TypeOf(e)
	if typ == nil {
		return Target{Resolution: "dynamic"}
	}
	typ = types.Unalias(typ)
	if named, ok := typ.(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "net/http" && named.Obj().Name() == "HandlerFunc" {
		return x.Resolve(pkg, e)
	}
	return x.handlerMethod(pkg, e, typ)
}
func (x *Index) handlerMethod(pkg *project.Package, e ast.Expr, typ types.Type) Target {
	method, _, _ := types.LookupFieldOrMethod(typ, true, nil, "ServeHTTP")
	fn, ok := method.(*types.Func)
	if !ok {
		return Target{Resolution: "dynamic"}
	}
	if iface, ok := typ.Underlying().(*types.Interface); ok {
		return x.interfaceTarget(pkg, e, fn, typ, iface)
	}
	if f := x.Objects[fn.Origin()]; f != nil {
		return Target{ID: f.ID, Resolution: "static"}
	}
	id := graph.ID("externalCall", fn.FullName())
	path := ""
	if fn.Pkg() != nil {
		path = fn.Pkg().Path()
	}
	x.Extra[id] = graph.Node{ID: id, Kind: "externalCall", Label: fn.FullName(), Attributes: graph.Attributes{ImportPath: path, External: true, Resolution: "external", Signature: types.TypeString(fn.Type(), qualifier)}}
	return Target{ID: id, Resolution: "external"}
}

func (x *Index) Definition(id string) *graph.SourceRange {
	if node, ok := x.Extra[id]; ok {
		return node.Source
	}
	for _, function := range x.Functions {
		if function.ID == id {
			return x.Source(function.Package, function.Decl)
		}
	}
	return nil
}

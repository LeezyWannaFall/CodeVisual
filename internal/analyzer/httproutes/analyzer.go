// Package httproutes reads chi registrations without executing router builders.
package httproutes

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"codevisual/internal/analyzer/symbols"
	"codevisual/internal/graph"
	"codevisual/internal/project"
)

type Analyzer struct{ Index *symbols.Index }
type function struct {
	decl    *ast.FuncDecl
	pkg     *project.Package
	aliases map[string]bool
}
type router struct {
	identity          string
	group             *routerGroup
	prefix, scope     string
	middleware        []string
	middlewareTargets []graph.TargetRef
}

// Router instances retain declarations until mounts have been resolved.
type routerGroup struct {
	registrations []registration
	mounts        []mount
	mounted       bool
}
type registration struct {
	f     *function
	call  *ast.CallExpr
	state router
}
type mount struct {
	child *routerGroup
	state router
}
type walker struct {
	index         *symbols.Index
	project       *project.Project
	result        graph.AnalysisResult
	functions     map[types.Object]*function
	named         map[string]*function
	all           []*function
	files         map[string]bool
	registrations map[string]bool
	stack         map[*function]bool
	groups        []*routerGroup
	ctx           context.Context
}

func (a Analyzer) Analyze(ctx context.Context, p *project.Project) (graph.AnalysisResult, error) {
	index := a.Index
	if index == nil {
		index = symbols.New(p)
	}
	w := &walker{index: index, project: p, functions: map[types.Object]*function{}, named: map[string]*function{}, files: map[string]bool{}, registrations: map[string]bool{}, stack: map[*function]bool{}, ctx: ctx}
	for i := range p.Packages {
		pkg := &p.Packages[i]
		for _, file := range pkg.Syntax {
			aliases := map[string]bool{}
			for _, im := range file.Imports {
				path, _ := strconv.Unquote(im.Path.Value)
				if path == "github.com/go-chi/chi/v5" || path == "github.com/go-chi/chi" {
					name := "chi"
					if im.Name != nil {
						name = im.Name.Name
					}
					aliases[name] = true
				}
			}
			for _, decl := range file.Decls {
				if d, ok := decl.(*ast.FuncDecl); ok && d.Body != nil {
					f := &function{d, pkg, aliases}
					w.all = append(w.all, f)
					if pkg.TypesInfo != nil {
						if obj := pkg.TypesInfo.Defs[d.Name]; obj != nil {
							w.functions[obj] = f
						}
					}
					if d.Recv == nil {
						w.named[pkg.ImportPath+":"+d.Name.Name] = f
					}
				}
			}
		}
	}
	consumed := map[*function]bool{}
	for _, f := range w.all {
		if !rootFunction(f) {
			continue
		}
		builders := map[string]*function{}
		ast.Inspect(f.decl.Body, func(n ast.Node) bool {
			if spec, ok := n.(*ast.ValueSpec); ok {
				for _, value := range spec.Values {
					if call, ok := value.(*ast.CallExpr); ok {
						if target := w.resolve(f, call.Fun); target != nil && rootFunction(target) {
							consumed[target] = true
						}
					}
				}
			}
			if ret, ok := n.(*ast.ReturnStmt); ok {
				for _, expr := range ret.Results {
					if c, ok := expr.(*ast.CallExpr); ok {
						if target := w.resolve(f, c.Fun); target != nil && rootFunction(target) {
							consumed[target] = true
						}
					}
				}
			}
			if a, ok := n.(*ast.AssignStmt); ok {
				for i, e := range a.Rhs {
					if i >= len(a.Lhs) {
						break
					}
					if id, ok := a.Lhs[i].(*ast.Ident); ok {
						if c, ok := e.(*ast.CallExpr); ok {
							if target := w.resolve(f, c.Fun); target != nil && rootFunction(target) {
								builders[id.Name] = target
								consumed[target] = true
							}
						}
					}
				}
			}
			return true
		})
		ast.Inspect(f.decl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				callback := -1
				if sel.Sel.Name == "Route" {
					callback = 1
				}
				if sel.Sel.Name == "Group" {
					callback = 0
				}
				if callback >= 0 && len(call.Args) > callback {
					if target := w.resolve(f, call.Args[callback]); target != nil {
						consumed[target] = true
					}
				}
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Mount" && len(call.Args) == 2 {
				if id, ok := call.Args[1].(*ast.Ident); ok {
					if target := builders[id.Name]; target != nil {
						consumed[target] = true
					}
				}
				if c, ok := call.Args[1].(*ast.CallExpr); ok {
					if target := w.resolve(f, c.Fun); target != nil {
						consumed[target] = true
					}
				}
			}
			if target := w.resolve(f, call.Fun); target != nil && len(routerParameters(target)) > 0 {
				consumed[target] = true
			}
			return true
		})
	}
	for _, f := range w.all {
		if ctx.Err() != nil {
			return w.result, ctx.Err()
		}
		if consumed[f] || !rootFunction(f) {
			continue
		}
		w.run(f, router{scope: functionScope(f)}, nil)
	}
	for _, group := range w.groups {
		if !group.mounted {
			w.expand(group, router{}, map[*routerGroup]bool{})
		}
	}
	return w.result, ctx.Err()
}
func (w *walker) resolve(f *function, e ast.Expr) *function {
	if target := w.index.Function(f.pkg, e); target != nil {
		if target.Object != nil {
			if fn := w.functions[target.Object]; fn != nil {
				return fn
			}
		}
		for _, fn := range w.all {
			if fn.decl == target.Decl {
				return fn
			}
		}
	}
	if f.pkg.TypesInfo != nil {
		var obj types.Object
		switch e := e.(type) {
		case *ast.Ident:
			obj = f.pkg.TypesInfo.Uses[e]
		case *ast.SelectorExpr:
			obj = f.pkg.TypesInfo.Uses[e.Sel]
		}
		if result := w.functions[obj]; result != nil {
			return result
		}
	}
	if id, ok := e.(*ast.Ident); ok {
		return w.named[f.pkg.ImportPath+":"+id.Name]
	}
	return nil
}
func routerParameters(f *function) []string {
	out := []string{}
	if f.decl.Type.Params == nil {
		return out
	}
	for _, field := range f.decl.Type.Params.List {
		if f.pkg.TypesInfo != nil && chiRouterType(f.pkg.TypesInfo.TypeOf(field.Type)) {
			for _, n := range field.Names {
				out = append(out, n.Name)
			}
			continue
		}
		if s, ok := field.Type.(*ast.SelectorExpr); ok {
			if x, ok := s.X.(*ast.Ident); ok && f.aliases[x.Name] && s.Sel.Name == "Router" {
				for _, n := range field.Names {
					out = append(out, n.Name)
				}
			}
		}
	}
	return out
}
func (w *walker) run(f *function, state router, params map[string]router) map[string]router {
	if w.ctx.Err() != nil {
		return nil
	}
	if w.stack[f] {
		w.note(f, f.decl, "http_recursive_builder", "Recursive router builder was not expanded")
		return nil
	}
	w.stack[f] = true
	defer delete(w.stack, f)
	env := map[string]router{}
	if state.group == nil && len(routerParameters(f)) > 0 {
		state.group = w.newGroup()
	}
	for _, name := range routerParameters(f) {
		value := state
		if value.identity == "" {
			value.identity = state.scope + ":" + name
		}
		env[name] = value
	}
	for k, v := range params {
		env[k] = v
	}
	w.block(f, f.decl.Body, env, state)
	return env
}
func clone(env map[string]router) map[string]router {
	out := map[string]router{}
	for k, v := range env {
		v.middleware = append([]string{}, v.middleware...)
		v.middlewareTargets = append([]graph.TargetRef{}, v.middlewareTargets...)
		out[k] = v
	}
	return out
}
func (w *walker) block(f *function, b *ast.BlockStmt, env map[string]router, state router) {
	if b == nil {
		return
	}
	for _, stmt := range b.List {
		if w.ctx.Err() != nil {
			return
		}
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for i, rhs := range s.Rhs {
				if i >= len(s.Lhs) {
					break
				}
				if id, ok := s.Lhs[i].(*ast.Ident); ok {
					if r, ok := w.routerExpr(f, rhs, env, state); ok {
						env[id.Name] = r
					} else {
						delete(env, id.Name)
					}
				}
			}
		case *ast.DeclStmt:
			if d, ok := s.Decl.(*ast.GenDecl); ok {
				for _, sp := range d.Specs {
					if v, ok := sp.(*ast.ValueSpec); ok {
						for i, value := range v.Values {
							if i < len(v.Names) {
								if r, ok := w.routerExpr(f, value, env, state); ok {
									env[v.Names[i].Name] = r
								}
							}
						}
					}
				}
			}
		case *ast.ExprStmt:
			if c, ok := s.X.(*ast.CallExpr); ok {
				w.call(f, c, env, state)
			}
		case *ast.ReturnStmt:
			for _, e := range s.Results {
				if r, ok := w.routerExpr(f, e, env, state); ok {
					env["$return"] = r
				} else if call, ok := e.(*ast.CallExpr); ok {
					w.call(f, call, env, state)
				}
			}
			return
		case *ast.BlockStmt:
			w.block(f, s, clone(env), state)
		case *ast.IfStmt:
			w.note(f, s, "http_conditional", "Conditional registrations are omitted; runtime branch is unknown")
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			w.note(f, s, "http_dynamic_control", "Loop/switch registrations are omitted; runtime values are unknown")
		}
	}
}
func (w *walker) routerExpr(f *function, e ast.Expr, env map[string]router, state router) (router, bool) {
	if id, ok := e.(*ast.Ident); ok {
		r, found := env[id.Name]
		return r, found
	}
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return router{}, false
	}
	if target := w.resolve(f, c.Fun); target != nil && rootFunction(target) {
		// Evaluate builders once per call site and preserve their returned instance.
		result := w.run(target, router{scope: state.scope}, nil)
		r, ok := result["$return"]
		if !ok {
			w.note(f, c, "http_builder_return", "Router builder return could not be resolved statically")
		}
		return r, ok
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return router{}, false
	}
	if id, ok := s.X.(*ast.Ident); ok && f.aliases[id.Name] && (s.Sel.Name == "NewRouter" || s.Sel.Name == "NewMux") {
		state.identity = f.pkg.ImportPath + ":" + strconv.Itoa(int(c.Pos()))
		state.group = w.newGroup()
		return state, true
	}
	if s.Sel.Name == "With" {
		r, ok := w.routerExpr(f, s.X, env, state)
		if !ok {
			return router{}, false
		}
		r.middleware = append([]string{}, r.middleware...)
		r.identity += ":with:" + strconv.Itoa(int(c.Pos()))
		r.middlewareTargets = append([]graph.TargetRef{}, r.middlewareTargets...)
		for _, arg := range c.Args {
			r.middleware = append(r.middleware, printed(f, arg))
			r.middlewareTargets = append(r.middlewareTargets, w.targetRef(f, arg))
		}
		return r, true
	}
	return router{}, false
}
func (w *walker) call(f *function, c *ast.CallExpr, env map[string]router, state router) {
	s, selector := c.Fun.(*ast.SelectorExpr)
	if !selector {
		w.helper(f, c, env, state)
		return
	}
	r, ok := w.routerExpr(f, s.X, env, state)
	if !ok {
		w.helper(f, c, env, state)
		return
	}
	name := s.Sel.Name
	if name == "Use" {
		for _, arg := range c.Args {
			r.middleware = append(r.middleware, printed(f, arg))
			r.middlewareTargets = append(r.middlewareTargets, w.targetRef(f, arg))
		}
		for name, value := range env {
			if value.identity == r.identity {
				value.middleware = append([]string{}, r.middleware...)
				value.middlewareTargets = append([]graph.TargetRef{}, r.middlewareTargets...)
				env[name] = value
			}
		}
		return
	}
	if name == "Route" || name == "Group" {
		index := 0
		if name == "Route" {
			if len(c.Args) < 2 {
				return
			}
			prefix, ok := literal(f, c.Args[0])
			if !ok {
				w.note(f, c, "http_dynamic_path", "Dynamic Route prefix was omitted")
				return
			}
			r.prefix = join(r.prefix, prefix)
			index = 1
		}
		if len(c.Args) <= index {
			return
		}
		callbackExpr := c.Args[index]
		if literal, ok := callbackExpr.(*ast.ParenExpr); ok {
			callbackExpr = literal.X
		}
		callback, ok := callbackExpr.(*ast.FuncLit)
		if !ok {
			if target := w.resolve(f, c.Args[index]); target != nil {
				w.run(target, r, nil)
				return
			}
			w.note(f, c, "http_callback", "Route callback could not be resolved statically")
			return
		}
		local := clone(env)
		if callback.Type.Params != nil {
			for _, field := range callback.Type.Params.List {
				for _, n := range field.Names {
					local[n.Name] = r
				}
			}
		}
		w.block(f, callback.Body, local, r)
		return
	}
	if name == "Mount" {
		if len(c.Args) != 2 {
			return
		}
		prefix, ok := literal(f, c.Args[0])
		if !ok {
			w.note(f, c, "http_dynamic_mount", "Dynamic mount prefix was omitted")
			return
		}
		r.prefix = join(r.prefix, prefix)
		if child, found := w.routerExpr(f, c.Args[1], env, state); found && child.group != nil && r.group != nil {
			if containsGroup(child.group, r.group, map[*routerGroup]bool{}) {
				w.note(f, c, "http_recursive_mount", "Cyclic router mount was omitted")
				return
			}
			child.group.mounted = true
			r.group.mounts = append(r.group.mounts, mount{child.group, r})
			return
		}
		w.note(f, c, "http_unresolved_mount", "Mounted handler could not be resolved statically: "+printed(f, c.Args[1]))
		return
	}
	if r.group != nil {
		r.group.registrations = append(r.group.registrations, registration{f, c, r})
	}
}

func (w *walker) newGroup() *routerGroup {
	group := &routerGroup{}
	w.groups = append(w.groups, group)
	return group
}

func containsGroup(group, target *routerGroup, seen map[*routerGroup]bool) bool {
	if group == target {
		return true
	}
	if seen[group] {
		return false
	}
	seen[group] = true
	for _, item := range group.mounts {
		if containsGroup(item.child, target, seen) {
			return true
		}
	}
	return false
}

func inherited(parent, child router) router {
	child.prefix = join(parent.prefix, child.prefix)
	child.middleware = append(append([]string{}, parent.middleware...), child.middleware...)
	child.middlewareTargets = append(append([]graph.TargetRef{}, parent.middlewareTargets...), child.middlewareTargets...)
	if parent.scope != "" {
		child.scope = parent.scope
	}
	return child
}

func (w *walker) expand(group *routerGroup, parent router, stack map[*routerGroup]bool) {
	if stack[group] || w.ctx.Err() != nil {
		return
	}
	stack[group] = true
	defer delete(stack, group)
	for _, item := range group.registrations {
		w.emit(item.f, item.call, inherited(parent, item.state))
	}
	for _, item := range group.mounts {
		w.expand(item.child, inherited(parent, item.state), stack)
	}
}

func (w *walker) emit(f *function, c *ast.CallExpr, r router) {
	name := c.Fun.(*ast.SelectorExpr).Sel.Name
	method := ""
	pathIndex, handlerIndex := 0, 1
	switch name {
	case "Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Connect", "Trace":
		method = strings.ToUpper(name)
	case "Handle", "HandleFunc":
		method = "ANY"
	case "Method", "MethodFunc":
		if len(c.Args) < 3 {
			return
		}
		value, ok := literal(f, c.Args[0])
		if !ok {
			w.note(f, c, "http_dynamic_method", "Dynamic HTTP method was omitted")
			return
		}
		method = value
		pathIndex = 1
		handlerIndex = 2
	default:
		return
	}
	if len(c.Args) <= handlerIndex {
		return
	}
	path, ok := literal(f, c.Args[pathIndex])
	if !ok {
		w.note(f, c, "http_dynamic_path", "Dynamic route path was omitted")
		return
	}
	path = join(r.prefix, path)
	handler := printed(f, c.Args[handlerIndex])
	source := w.source(f, c)
	if source == nil {
		return
	}
	id := graph.ID("httpRoute", r.scope+":"+method+":"+path+":"+source.FileID+":"+strconv.Itoa(source.StartLine)+":"+strconv.Itoa(source.StartColumn))
	if w.registrations[id] {
		return
	}
	w.registrations[id] = true
	target := w.index.Handler(f.pkg, c.Args[handlerIndex], name == "Handle" || name == "Method")
	if target.ID == "" {
		boundary := w.index.Boundary(f.pkg, c.Args[handlerIndex])
		boundary.FactoryID = target.FactoryID
		target = boundary
	}
	route := &graph.HTTPRoute{HandlerSource: w.index.Definition(target.ID), HandlerID: target.ID, HandlerResolution: target.Resolution, HandlerFactoryID: target.FactoryID, MiddlewareTargets: append([]graph.TargetRef{}, r.middlewareTargets...), Method: method, Path: path, Handler: handler, Middleware: append([]string{}, r.middleware...), Scope: r.scope}
	w.result.Nodes = append(w.result.Nodes, graph.Node{ID: id, Kind: "httpRoute", Label: method + " " + path, ParentID: graph.ID("package", f.pkg.ImportPath), Source: source, Attributes: graph.Attributes{ImportPath: f.pkg.ImportPath, Signature: method + " " + path + " → " + handler, HTTP: route}})
}
func (w *walker) helper(f *function, c *ast.CallExpr, env map[string]router, state router) {
	target := w.resolve(f, c.Fun)
	if target == nil {
		return
	}
	params := routerParameters(target)
	if len(params) == 0 {
		return
	}
	bindings := map[string]router{}
	// Bind by actual parameter position, including preceding non-router arguments.
	position := 0
	for _, field := range target.decl.Type.Params.List {
		for _, name := range field.Names {
			if position < len(c.Args) {
				if r, ok := w.routerExpr(f, c.Args[position], env, state); ok {
					bindings[name.Name] = r
				}
			}
			position++
		}
	}
	if len(bindings) > 0 {
		changed := w.run(target, state, bindings)
		position = 0
		for _, field := range target.decl.Type.Params.List {
			for _, name := range field.Names {
				if position < len(c.Args) {
					if arg, ok := c.Args[position].(*ast.Ident); ok {
						old, exists := env[arg.Name]
						updated, found := changed[name.Name]
						if exists && found && old.identity == updated.identity {
							for key, value := range env {
								if value.identity == old.identity {
									value.middleware = updated.middleware
									value.middlewareTargets = updated.middlewareTargets
									env[key] = value
								}
							}
						}
					}
				}
				position++
			}
		}
	}
}
func literal(f *function, e ast.Expr) (string, bool) {
	if f.pkg.TypesInfo != nil {
		if v, ok := f.pkg.TypesInfo.Types[e]; ok && v.Value != nil && v.Value.Kind() == constant.String {
			return constant.StringVal(v.Value), true
		}
	}
	if s, ok := e.(*ast.BasicLit); ok && s.Kind == token.STRING {
		value, err := strconv.Unquote(s.Value)
		return value, err == nil
	}
	return "", false
}
func join(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" {
		return prefix
	}
	if path == "/" {
		return strings.TrimSuffix(prefix, "/") + "/"
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(path, "/")
}
func printed(f *function, n ast.Node) string {
	if literal, ok := n.(*ast.FuncLit); ok {
		var signature bytes.Buffer
		_ = format.Node(&signature, f.pkg.Fset, literal.Type)
		return signature.String() + " { … }"
	}
	var b bytes.Buffer
	_ = format.Node(&b, f.pkg.Fset, n)
	return b.String()
}
func (w *walker) source(f *function, n ast.Node) *graph.SourceRange {
	a, b := f.pkg.Fset.Position(n.Pos()), f.pkg.Fset.Position(n.End())
	rel, err := filepath.Rel(w.project.Root, a.Filename)
	if err != nil || !filepath.IsLocal(rel) {
		return nil
	}
	rel = filepath.ToSlash(rel)
	id := graph.ID("file", rel)
	if !w.files[id] {
		hash := w.project.SourceHashes[a.Filename]
		if hash == "" {
			content, err := project.ReadFile(w.project.Root, rel)
			if err != nil {
				return nil
			}
			hash = graph.Hash(content)
		}
		w.files[id] = true
		w.result.Files = append(w.result.Files, graph.File{ID: id, RelativePath: rel, ContentHash: hash})
	}
	return &graph.SourceRange{FileID: id, StartLine: a.Line, StartColumn: a.Column, EndLine: b.Line, EndColumn: b.Column}
}
func (w *walker) note(f *function, n ast.Node, code, message string) {
	if code == "http_conditional" || code == "http_dynamic_control" {
		relevant := false
		ast.Inspect(n, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
					switch sel.Sel.Name {
					case "Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Method", "MethodFunc", "Handle", "HandleFunc", "Use", "Route", "Group", "Mount":
						relevant = true
					}
				}
			}
			return !relevant
		})
		if !relevant {
			return
		}
	}

	// Report limitations only in functions that actually mention chi/router state.
	if len(f.aliases) == 0 && len(routerParameters(f)) == 0 {
		return
	}
	w.result.Diagnostics = append(w.result.Diagnostics, graph.Diagnostic{Severity: "warning", Code: code, Message: fmt.Sprintf("%s: %s", f.decl.Name.Name, message), Source: w.source(f, n)})
}

func rootFunction(f *function) bool {
	if len(routerParameters(f)) > 0 {
		return true
	}
	if f.decl.Type.Results != nil {
		for _, field := range f.decl.Type.Results.List {
			if f.pkg.TypesInfo != nil && chiRouterType(f.pkg.TypesInfo.TypeOf(field.Type)) {
				return true
			}
			if sel, ok := field.Type.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && f.aliases[id.Name] && sel.Sel.Name == "Router" {
					return true
				}
			}
		}
	}
	found := false
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok && f.aliases[id.Name] && (s.Sel.Name == "NewRouter" || s.Sel.Name == "NewMux") {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func functionScope(f *function) string {
	receiver := ""
	if f.decl.Recv != nil && len(f.decl.Recv.List) > 0 {
		receiver = printed(f, f.decl.Recv.List[0].Type) + "."
	}
	return f.pkg.ImportPath + "." + receiver + f.decl.Name.Name
}

func (w *walker) targetRef(f *function, e ast.Expr) graph.TargetRef {
	target := w.index.Resolve(f.pkg, e)
	return graph.TargetRef{Source: w.index.Definition(target.ID), Name: printed(f, e), NodeID: target.ID, Resolution: target.Resolution, FactoryID: target.FactoryID}
}
func chiRouterType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	path := named.Obj().Pkg().Path()
	return (path == "github.com/go-chi/chi/v5" || path == "github.com/go-chi/chi") && (named.Obj().Name() == "Router" || named.Obj().Name() == "Mux")
}

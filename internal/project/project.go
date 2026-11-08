// Package project discovers one module and loads Go source without executing it.
package project

import (
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"codevisual/internal/graph"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

type Package struct {
	ImportPath string
	Directory  string
	Name       string
	Fset       *token.FileSet
	Syntax     []*ast.File
	TypesInfo  *types.Info
}
type Project struct {
	HasGoModule  bool
	GoModuleDirs []string
	PythonFiles  []string
	ComposeFiles []string
	Dockerfiles  []string
	graph.Project
	Directories  []string
	Packages     []Package
	Diagnostics  []graph.Diagnostic
	SourceHashes map[string]string
}

func Discover(ctx context.Context, path string, tags []string) (*Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("project path: %w", err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("project path must be a directory")
	}
	root := abs
	for {
		if _, err = os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return nil, fmt.Errorf("no go.mod found above %s", abs)
		}
		root = parent
	}
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	mf, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}
	if mf.Module == nil {
		return nil, fmt.Errorf("go.mod has no module directive")
	}
	p := &Project{Project: graph.Project{Root: root, ModulePath: mf.Module.Mod.Path, RootName: filepath.Base(root), BuildContext: graph.BuildContext{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Tags: append([]string{}, tags...), ConfigurationKnown: true}}, Directories: []string{}, Packages: []Package{}, Diagnostics: []graph.Diagnostic{}}
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			p.note("info", "workspace_ignored", "go.work is ignored; only the selected module is analyzed")
			break
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if path != root {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
					return filepath.SkipDir
				}
				if _, e := os.Stat(filepath.Join(path, "go.mod")); e == nil {
					p.note("info", "nested_module_skipped", "Skipped nested module: "+filepath.ToSlash(rel))
					return filepath.SkipDir
				}
			}
			entries, e := os.ReadDir(path)
			if e != nil {
				return e
			}
			for _, entry := range entries {
				if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
					p.Directories = append(p.Directories, rel)
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan module: %w", err)
	}
	sort.Strings(p.Directories)
	if len(p.Directories) == 0 {
		return nil, fmt.Errorf("module has no non-test Go source packages")
	}
	return p, nil
}
func (p *Project) note(severity, code, message string) {
	p.Diagnostics = append(p.Diagnostics, graph.Diagnostic{Severity: severity, Code: code, Message: message})
}

// OfflineEnv deliberately overrides user hooks and network-capable Go settings.
func OfflineEnv(telemetryDir string) []string {
	overrides := map[string]string{"GOPROXY": "off", "GONOPROXY": "none", "GONOSUMDB": "*", "GOAUTH": "off", "TEST_TELEMETRY_DIR": telemetryDir, "GO_TELEMETRY_CHILD": "", "GO_TELEMETRY_CHILD_UPLOAD": "", "GOSUMDB": "off", "GOVCS": "off", "GOTOOLCHAIN": "local", "GOWORK": "off", "GOPACKAGESDRIVER": "off", "CGO_ENABLED": "0", "GOFLAGS": "-mod=readonly", "GOTELEMETRY": "local", "GOTOOLDIR": "", "GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH}
	out := []string{}
	for _, item := range os.Environ() {
		k, _, _ := strings.Cut(item, "=")
		if _, ok := overrides[k]; !ok {
			out = append(out, item)
		}
	}
	keys := []string{}
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+overrides[k])
	}
	return out
}
func (p *Project) Load(ctx context.Context) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go toolchain is required: install Go 1.26 or newer and make the go command available in PATH: %w", err)
	}
	patterns := []string{}
	for _, dir := range p.Directories {
		if dir == "." {
			patterns = append(patterns, ".")
		} else {
			patterns = append(patterns, "./"+filepath.ToSlash(dir))
		}
	}
	// -mod=readonly alone is not a write barrier for go.sum. Keep both module
	// metadata files in a temporary location while retaining the real module root.
	scratch, err := os.MkdirTemp("", "codevisual-analysis-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	metadata, err := ReadFile(p.Root, "go.mod")
	if err != nil {
		return err
	}
	alternate := filepath.Join(scratch, "analysis.mod")
	if err = os.WriteFile(alternate, metadata, 0600); err != nil {
		return err
	}
	if sums, e := ReadFile(p.Root, "go.sum"); e == nil {
		if err = os.WriteFile(filepath.Join(scratch, "analysis.sum"), sums, 0600); err != nil {
			return err
		}
	} else if !os.IsNotExist(e) {
		return fmt.Errorf("read go.sum: %w", e)
	}
	// GOTELEMETRY is read-only in the Go command. Its supported test-directory
	// hook isolates telemetry from the user's global mode, including mode=on.
	telemetry := filepath.Join(scratch, "telemetry")
	if err = os.Mkdir(telemetry, 0700); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(telemetry, "mode"), []byte("off\n"), 0600); err != nil {
		return err
	}
	mf, err := modfile.Parse("go.mod", metadata, nil)
	if err != nil {
		return fmt.Errorf("parse go.mod: %w", err)
	}
	required := "go1.26.0"
	if mf.Go != nil && version.Compare("go"+mf.Go.Version, required) > 0 {
		required = "go" + mf.Go.Version
	}
	probe := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	probe.Dir = scratch
	probe.Env = OfflineEnv(telemetry)
	output, err := probe.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("Go toolchain is unavailable: check the go installation in PATH: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err = validateToolchain(strings.TrimSpace(string(output)), required); err != nil {
		return err
	}
	flags := []string{"-mod=readonly", "-modfile=" + alternate}
	if len(p.BuildContext.Tags) > 0 {
		flags = append(flags, "-tags="+strings.Join(p.BuildContext.Tags, ","))
	}
	cfg := &packages.Config{Context: ctx, Dir: p.Root, Env: OfflineEnv(telemetry), Tests: false, Mode: packages.LoadSyntax | packages.NeedModule, BuildFlags: flags}
	p.SourceHashes = map[string]string{}
	var sourceMu sync.Mutex
	cfg.ParseFile = func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		// Only source within the selected module is parsed; no dependency AST is requested.
		rel, err := filepath.Rel(p.Root, filename)
		if err != nil || !filepath.IsLocal(rel) {
			return nil, fmt.Errorf("source outside module")
		}
		b, err := ReadFile(p.Root, filepath.ToSlash(rel))
		if err != nil {
			return nil, err
		}
		sourceMu.Lock()
		p.SourceHashes[filename] = graph.Hash(b)
		sourceMu.Unlock()
		return parser.ParseFile(fset, filename, b, parser.ParseComments|parser.AllErrors)
	}
	loaded, loadErr := packages.Load(cfg, patterns...)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if loadErr != nil {
		p.note("warning", "load_failed", strings.ReplaceAll(loadErr.Error(), scratch, "<analysis>"))
		p.BuildContext.ConfigurationKnown = false
	}
	byDir := map[string]*packages.Package{}
	for _, pkg := range loaded {
		for _, e := range pkg.Errors {
			p.note("warning", "package_load", strings.ReplaceAll(strings.ReplaceAll(e.Error(), p.Root+string(filepath.Separator), ""), scratch, "<analysis>"))
		}
		files := append(append([]string{}, pkg.GoFiles...), pkg.IgnoredFiles...)
		for _, f := range files {
			rel, e := filepath.Rel(p.Root, filepath.Dir(f))
			if e == nil && filepath.IsLocal(rel) {
				byDir[rel] = pkg
				break
			}
		}
	}
	bc := build.Default
	bc.GOOS = p.BuildContext.GOOS
	bc.GOARCH = p.BuildContext.GOARCH
	bc.CgoEnabled = false
	bc.BuildTags = p.BuildContext.Tags
	for _, dir := range p.Directories {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		full := filepath.Join(p.Root, dir)
		pkg := byDir[dir]
		importPath := p.ModulePath
		if dir != "." {
			importPath += "/" + filepath.ToSlash(dir)
		}
		out := Package{ImportPath: importPath, Directory: dir, Fset: token.NewFileSet()}
		parsed := map[string]bool{}
		if pkg != nil {
			out.Name = pkg.Name
			out.Fset = pkg.Fset
			if out.Fset == nil {
				out.Fset = token.NewFileSet()
			}
			out.TypesInfo = pkg.TypesInfo
			for _, f := range pkg.Syntax {
				if f != nil {
					out.Syntax = append(out.Syntax, f)
					parsed[out.Fset.Position(f.Pos()).Filename] = true
				}
			}
		}
		entries, e := os.ReadDir(full)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			active, matchErr := bc.MatchFile(full, name)
			if matchErr != nil {
				p.note("warning", "build_constraints", filepath.ToSlash(filepath.Join(dir, name))+": "+matchErr.Error())
				p.BuildContext.ConfigurationKnown = false
			}
			if !active && matchErr == nil {
				continue
			}
			filename := filepath.Join(full, name)
			if parsed[filename] {
				continue
			}
			b, e := ReadFile(p.Root, filepath.ToSlash(filepath.Join(dir, name)))
			if e != nil {
				p.note("warning", "source_unreadable", e.Error())
				continue
			}
			p.SourceHashes[filename] = graph.Hash(b)
			f, e := parser.ParseFile(out.Fset, filename, b, parser.ParseComments|parser.AllErrors)
			if e != nil {
				p.note("warning", "parse_error", strings.ReplaceAll(e.Error(), p.Root+string(filepath.Separator), ""))
			}
			if f != nil {
				// cgo is disabled even in fallback mode.
				cgo := false
				for _, im := range f.Imports {
					if im.Path.Value == `"C"` {
						cgo = true
					}
				}
				if cgo {
					p.note("warning", "cgo_skipped", "Skipped cgo file: "+filepath.ToSlash(filepath.Join(dir, name)))
					continue
				}
				if pkg == nil {
					p.BuildContext.ConfigurationKnown = false
					p.note("warning", "ast_fallback", "Using AST fallback for "+importPath)
				}
				out.Syntax = append(out.Syntax, f)
			}
		}
		if len(out.Syntax) == 0 {
			continue
		}
		if out.Name == "" {
			out.Name = out.Syntax[0].Name.Name
		}
		sort.Slice(out.Syntax, func(i, j int) bool {
			return out.Fset.Position(out.Syntax[i].Pos()).Filename < out.Fset.Position(out.Syntax[j].Pos()).Filename
		})
		p.Packages = append(p.Packages, out)
	}
	if len(p.Packages) == 0 {
		return fmt.Errorf("no packages could be analyzed for the current build configuration")
	}
	return nil
}

func validateToolchain(installed, required string) error {
	installed = strings.TrimPrefix(installed, "devel ")
	if !version.IsValid(installed) {
		return fmt.Errorf("cannot determine Go toolchain version (%q); install %s or newer and check PATH", installed, required)
	}
	if version.Compare(installed, required) < 0 {
		return fmt.Errorf("Go toolchain %s is too old: this analysis requires %s or newer; install it and check PATH (automatic toolchain downloads are disabled)", installed, required)
	}
	return nil
}

// ReadFile uses an OS-constrained root so a changed symlink cannot escape the module.
func ReadFile(root, relative string) ([]byte, error) {
	native := filepath.FromSlash(relative)
	if !filepath.IsLocal(native) {
		return nil, fmt.Errorf("invalid source path")
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	cur := ""
	for _, part := range strings.Split(native, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		st, err := r.Lstat(cur)
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink source is not allowed")
		}
	}
	return r.ReadFile(native)
}

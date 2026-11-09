package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"codevisual/internal/graph"
	"golang.org/x/mod/modfile"
)

// Open discovers source and infrastructure without requiring a particular language.
// A selected Go module retains its existing build and nested-module boundaries.
func Open(ctx context.Context, input string, tags []string) (*Project, error) {
	root, err := filepath.Abs(input)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("project path must be a directory")
	}
	// Only a Go source subdirectory is promoted to an ancestor module. An
	// explicitly selected Python/container directory remains its own scan root.
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	directGo, ownProject := false, false
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".go") {
			directGo = true
		}
		if name == "go.mod" || name == "pyproject.toml" || name == "setup.cfg" || name == "requirements.txt" || name == "Dockerfile" || strings.HasSuffix(name, ".py") || IsCompose(name) {
			ownProject = true
		}
	}
	if directGo && !ownProject {
		for dir := filepath.Dir(root); ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				root = dir
				break
			}
			if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
				break
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	p := &Project{Project: graph.Project{Root: root, RootName: filepath.Base(root), ModulePath: filepath.Base(root), BuildContext: graph.BuildContext{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Tags: append([]string{}, tags...), ConfigurationKnown: true}}, SourceHashes: map[string]string{}}
	if metadata, err := ReadFile(root, "go.mod"); err == nil {
		mf, err := modfile.Parse("go.mod", metadata, nil)
		if err != nil {
			return nil, fmt.Errorf("parse go.mod: %w", err)
		}
		if mf.Module == nil {
			return nil, fmt.Errorf("go.mod has no module directive")
		}
		p.ModulePath = mf.Module.Mod.Path
		for _, dependency := range mf.Require {
			p.RequiredModules = append(p.RequiredModules, dependency.Mod.Path)
		}
		p.HasGoModule = true
	}
	goDirs := map[string]bool{}
	goFiles := 0
	err = filepath.WalkDir(root, func(name string, d os.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(root, name)
		if d.IsDir() {
			if name != root {
				if SkipDirectory(d.Name()) {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(name, "go.mod")); err == nil {
					insideModule := p.HasGoModule
					for _, moduleDir := range p.GoModuleDirs {
						if strings.HasPrefix(filepath.ToSlash(rel), moduleDir+"/") {
							insideModule = true
						}
					}
					if insideModule {
						p.note("info", "nested_module_skipped", "Skipped nested module: "+filepath.ToSlash(rel))
						return filepath.SkipDir
					}
					p.GoModuleDirs = append(p.GoModuleDirs, filepath.ToSlash(rel))
				}
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base := d.Name()
		switch {
		case strings.HasSuffix(base, ".go") && !strings.HasSuffix(base, "_test.go"):
			goFiles++
			goDirs[filepath.Dir(filepath.FromSlash(rel))] = true
		case PythonSource(rel) && !strings.HasPrefix(base, "test_") && !strings.HasSuffix(base, "_test.py") && base != "conftest.py":
			p.PythonFiles = append(p.PythonFiles, rel)
		case IsCompose(base):
			p.ComposeFiles = append(p.ComposeFiles, rel)
		case !strings.HasSuffix(base, ".dockerignore") && (base == "Dockerfile" || strings.HasPrefix(base, "Dockerfile.") || strings.HasSuffix(base, ".Dockerfile")):
			p.Dockerfiles = append(p.Dockerfiles, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	if !p.HasGoModule && len(p.GoModuleDirs) > 0 {
		for dir := range goDirs {
			covered := false
			for _, moduleDir := range p.GoModuleDirs {
				value := filepath.ToSlash(dir)
				covered = covered || value == moduleDir || strings.HasPrefix(value, moduleDir+"/")
			}
			if !covered {
				p.note("warning", "go_source_outside_modules", "Go source outside discovered modules is omitted: "+filepath.ToSlash(dir))
			}
		}
	}
	if goFiles > 0 {
		p.Languages = append(p.Languages, "go")
		if !p.HasGoModule && len(p.GoModuleDirs) == 0 {
			p.note("warning", "go_module_missing", "Go source found without a root go.mod; add a Go module or analyze its module directory for Go declarations")
		}
	}
	if len(p.PythonFiles) > 0 {
		p.Languages = append(p.Languages, "python")
	}
	for dir := range goDirs {
		p.Directories = append(p.Directories, dir)
	}
	sort.Strings(p.GoModuleDirs)
	sort.Strings(p.Directories)
	sort.Strings(p.PythonFiles)
	sort.Strings(p.ComposeFiles)
	sort.Strings(p.Dockerfiles)
	if len(p.Languages) == 0 && len(p.ComposeFiles) == 0 && len(p.Dockerfiles) == 0 {
		return nil, fmt.Errorf("no Go, Python, Dockerfile, or Docker Compose files found")
	}
	return p, nil
}

func SkipDirectory(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "vendor", "node_modules", "testdata", "venv", "__pycache__", "site-packages":
		return true
	}
	return false
}
func IsCompose(name string) bool {
	lower := strings.ToLower(name)
	return (strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) && (strings.HasPrefix(lower, "compose.") || strings.HasPrefix(lower, "docker-compose."))
}

func PythonSource(relative string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		if part == "tests" || part == "test" {
			return false
		}
	}
	return strings.HasSuffix(relative, ".py")
}

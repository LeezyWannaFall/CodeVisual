// Package containers inventories Docker/Compose declarations without contacting Docker.
package containers

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"codevisual/internal/graph"
	"codevisual/internal/project"
	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
	"gopkg.in/yaml.v3"
)

type Analyzer struct{ Code graph.AnalysisResult }
type walker struct {
	p      *project.Project
	code   graph.AnalysisResult
	result graph.AnalysisResult
	ctx    context.Context
	used   map[string]bool
	files  map[string]bool
	builds map[string][]stage
}
type stage struct {
	id, name string
	data     *graph.Container
	source   *graph.SourceRange
}

func newContainer(kind string) *graph.Container {
	return &graph.Container{Type: kind, Ports: []string{}, Volumes: []string{}, Networks: []string{}, EnvironmentKeys: []string{}, Contents: []graph.ContainerContent{}}
}
func (a Analyzer) Analyze(ctx context.Context, p *project.Project) (graph.AnalysisResult, error) {
	w := &walker{p: p, code: a.Code, ctx: ctx, used: map[string]bool{}, files: map[string]bool{}, builds: map[string][]stage{}}
	for _, file := range p.ComposeFiles {
		if ctx.Err() != nil {
			return w.result, ctx.Err()
		}
		w.compose(file)
	}
	for _, file := range p.Dockerfiles {
		if ctx.Err() != nil {
			return w.result, ctx.Err()
		}
		if w.used[file] {
			continue
		}
		contextDir := path.Dir(file)
		stages := w.dockerfile(file, contextDir)
		if len(stages) == 0 {
			continue
		}
		final := stages[len(stages)-1]
		data := clone(final.data)
		data.Type = "dockerfile"
		data.Dockerfile = file
		data.BuildContext = contextDir
		id := graph.ID("container", "dockerfile:"+file)
		w.result.Nodes = append(w.result.Nodes, graph.Node{ID: id, Kind: "container", Label: file, Source: final.source, Attributes: graph.Attributes{RelativePath: file, Container: data}})
		w.result.Edges = append(w.result.Edges, graph.Link("builds_from", id, final.id, final.source))
	}
	return w.result, ctx.Err()
}
func clone(c *graph.Container) *graph.Container {
	v := *c
	v.Contents = append([]graph.ContainerContent{}, c.Contents...)
	v.Ports = append([]string{}, c.Ports...)
	v.Volumes = append([]string{}, c.Volumes...)
	v.EnvironmentKeys = append([]string{}, c.EnvironmentKeys...)
	return &v
}
func (w *walker) note(code, message string, source *graph.SourceRange) {
	w.result.Diagnostics = append(w.result.Diagnostics, graph.Diagnostic{Severity: "warning", Code: code, Message: message, Source: source})
}
func (w *walker) read(file string) ([]byte, error) {
	b, err := project.ReadFile(w.p.Root, file)
	if err != nil {
		return nil, err
	}
	if !w.files[file] {
		w.files[file] = true
		w.result.Files = append(w.result.Files, graph.File{ID: graph.ID("file", file), RelativePath: file, ContentHash: graph.Hash(b)})
	}
	return b, nil
}
func location(file string, line, end int) *graph.SourceRange {
	return &graph.SourceRange{FileID: graph.ID("file", file), StartLine: line, StartColumn: 1, EndLine: end, EndColumn: 1}
}
func local(base, relative string) (string, bool) {
	if strings.ContainsAny(relative, "$\x00") || strings.Contains(relative, ":") || strings.HasPrefix(relative, "~") || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", false
	}
	value := path.Clean(path.Join(base, relative))
	return value, filepath.IsLocal(filepath.FromSlash(value))
}
func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := []string{}
		for _, x := range v {
			parts = append(parts, fmt.Sprint(x))
		}
		return strings.Join(parts, " ")
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
func mapping(value any) map[string]any { v, _ := value.(map[string]any); return v }
func list(value any) []any             { v, _ := value.([]any); return v }
func keys(value any) []string {
	out := []string{}
	switch v := value.(type) {
	case map[string]any:
		for k := range v {
			out = append(out, k)
		}
	case []any:
		for _, item := range v {
			out = append(out, strings.SplitN(fmt.Sprint(item), "=", 2)[0])
		}
	}
	sort.Strings(out)
	return out
}
func stringsOf(value any) []string {
	out := []string{}
	for _, v := range list(value) {
		out = append(out, stringValue(v))
	}
	return out
}
func unsupportedTags(n *yaml.Node) bool {
	if strings.HasPrefix(n.Tag, "!") && !strings.HasPrefix(n.Tag, "!!") {
		return true
	}
	for _, c := range n.Content {
		if unsupportedTags(c) {
			return true
		}
	}
	return false
}
func (w *walker) compose(file string) {
	b, err := w.read(file)
	if err != nil {
		w.note("compose_read", file+": "+err.Error(), nil)
		return
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(b, &doc); err != nil {
		w.note("compose_parse", file+": "+err.Error(), nil)
		return
	}
	if unsupportedTags(&doc) {
		w.note("compose_tags", file+": custom YAML override/reset tags are not expanded", nil)
		return
	}
	var config map[string]any
	if err = doc.Decode(&config); err != nil {
		w.note("compose_parse", file+": "+err.Error(), nil)
		return
	}
	services := mapping(config["services"])
	if services == nil {
		w.note("compose_services", file+": services must be a mapping", nil)
		return
	}
	if config["include"] != nil {
		w.note("compose_include", file+": included Compose files are not expanded", nil)
	}
	if strings.Contains(string(b), "${") {
		w.note("compose_interpolation", file+": environment interpolation is shown literally; .env files and host variables are not loaded", nil)
	}
	if strings.Contains(path.Base(file), "override") {
		w.note("compose_overlay", file+": analyzed independently; Compose file merging is not simulated", nil)
	}
	serviceSource := map[string]*graph.SourceRange{}
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == "services" {
				n := root.Content[i+1]
				for j := 0; j+1 < len(n.Content); j += 2 {
					serviceSource[n.Content[j].Value] = location(file, n.Content[j].Line, lastLine(n.Content[j+1]))
				}
			}
		}
	}
	ids := map[string]string{}
	for name, raw := range services {
		if mapping(raw) != nil {
			ids[name] = graph.ID("container", file+":"+name)
		}
	}
	for _, name := range keys(services) {
		if w.ctx.Err() != nil {
			return
		}
		service := mapping(services[name])
		if service == nil {
			w.note("compose_service", file+": invalid service "+name, nil)
			continue
		}
		source := serviceSource[name]
		data := newContainer("compose")
		data.Image = stringValue(service["image"])
		data.Command = stringValue(service["command"])
		data.Entrypoint = stringValue(service["entrypoint"])
		data.Workdir = stringValue(service["working_dir"])
		data.EnvironmentKeys = keys(service["environment"])
		data.Ports = stringsOf(service["ports"])
		for i, v := range list(service["ports"]) {
			if port := mapping(v); port != nil {
				data.Ports[i] = stringValue(port["target"])
				if published := stringValue(port["published"]); published != "" {
					data.Ports[i] = published + ":" + data.Ports[i]
				}
				if protocol := stringValue(port["protocol"]); protocol != "" {
					data.Ports[i] += "/" + protocol
				}
			}
		}
		data.Ports = append(data.Ports, stringsOf(service["expose"])...)
		if service["extends"] != nil {
			w.note("compose_extends", file+": service "+name+" extends another definition; inheritance is not expanded", source)
		}
		contextDir, dockerfile, target := "", "Dockerfile", ""
		if build, ok := service["build"].(string); ok {
			contextDir = build
		} else if build := mapping(service["build"]); build != nil {
			contextDir = stringValue(build["context"])
			if contextDir == "" {
				contextDir = "."
			}
			if v := stringValue(build["dockerfile"]); v != "" {
				dockerfile = v
			}
			target = stringValue(build["target"])
			if build["dockerfile_inline"] != nil {
				w.note("compose_inline", file+": inline Dockerfile is not expanded", source)
				contextDir = ""
			}
		}
		if contextDir != "" {
			contextRoot, ok := local(path.Dir(file), contextDir)
			dockerPath, okFile := local(contextRoot, dockerfile)
			if !ok || !okFile {
				w.note("docker_context", file+": external or interpolated build context/Dockerfile omitted for "+name, source)
			} else {
				data.BuildContext = contextRoot
				data.Dockerfile = dockerPath
				data.Stage = target
				w.used[dockerPath] = true
				stages := w.dockerfile(dockerPath, contextRoot)
				selected := -1
				if len(stages) > 0 && target == "" {
					selected = len(stages) - 1
				}
				for i, s := range stages {
					if strings.EqualFold(s.name, target) && target != "" {
						selected = i
					}
				}
				if selected >= 0 {
					s := stages[selected]
					data.Contents = append(data.Contents, s.data.Contents...)
					data.Ports = append(data.Ports, s.data.Ports...)
					if data.Image == "" {
						data.Image = s.data.Image
					}
					if data.Workdir == "" {
						data.Workdir = s.data.Workdir
					}
					if service["command"] == nil {
						data.Command = s.data.Command
					}
					if service["entrypoint"] == nil {
						data.Entrypoint = s.data.Entrypoint
					}
					data.EnvironmentKeys = append(data.EnvironmentKeys, s.data.EnvironmentKeys...)
					w.result.Edges = append(w.result.Edges, graph.Link("builds_from", ids[name], s.id, source))
				} else if target != "" {
					w.note("docker_target", file+": unknown build target "+target, source)
				}
			}
		}
		for _, dependency := range keys(service["depends_on"]) {
			if targetID, ok := ids[dependency]; ok {
				w.result.Edges = append(w.result.Edges, graph.Link("depends_on", ids[name], targetID, source))
			} else {
				w.note("compose_dependency", file+": unknown dependency "+dependency, source)
			}
		}
		networks := keys(service["networks"])
		if service["network_mode"] != nil {
			networks = []string{}
		} else if len(networks) == 0 {
			networks = []string{"default"}
		}
		for _, network := range networks {
			id := graph.ID("containerNetwork", file+":"+network)
			data.Networks = append(data.Networks, network)
			w.result.Nodes = append(w.result.Nodes, graph.Node{ID: id, Kind: "containerNetwork", Label: network, Attributes: graph.Attributes{RelativePath: file}})
			w.result.Edges = append(w.result.Edges, graph.Link("uses_network", ids[name], id, source))
		}
		for _, raw := range list(service["volumes"]) {
			src, dst, kind := "", "", "volume"
			if mount := mapping(raw); mount != nil {
				src = stringValue(mount["source"])
				dst = stringValue(mount["target"])
				if stringValue(mount["type"]) != "" {
					kind = stringValue(mount["type"])
				}
			} else {
				parts := strings.SplitN(stringValue(raw), ":", 3)
				if len(parts) == 1 {
					dst = parts[0]
				} else {
					src = parts[0]
					dst = parts[1]
				}
				if strings.HasPrefix(src, ".") || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "~") {
					kind = "bind"
				}
			}
			data.Volumes = append(data.Volumes, src+":"+dst)
			item := graph.ContainerContent{Kind: kind, Source: src, Target: dst, PackageIDs: []string{}}
			if kind == "bind" {
				if resolved, ok := local(path.Dir(file), src); ok {
					item.PackageIDs = w.packages(resolved, nil)
				} else {
					w.note("docker_bind", file+": bind source outside project or unresolved: "+src, source)
				}
			} else if kind == "volume" && src != "" {
				id := graph.ID("containerVolume", file+":"+src)
				w.result.Nodes = append(w.result.Nodes, graph.Node{ID: id, Kind: "containerVolume", Label: src, Attributes: graph.Attributes{RelativePath: file}})
				w.result.Edges = append(w.result.Edges, graph.Link("mounts", ids[name], id, source))
			}
			data.Contents = append(data.Contents, item)
		}
		w.result.Nodes = append(w.result.Nodes, graph.Node{ID: ids[name], Kind: "container", Label: name, Source: source, Attributes: graph.Attributes{RelativePath: file, Container: data}})
	}
	// Shared networks/volumes within a file are one declared resource.
	w.result = graph.Merge(w.result)
}
func lastLine(n *yaml.Node) int {
	last := n.Line
	for _, c := range n.Content {
		if v := lastLine(c); v > last {
			last = v
		}
	}
	return last
}

func (w *walker) ignore(contextDir, file string) *patternmatcher.PatternMatcher {
	ignore := path.Join(contextDir, ".dockerignore")
	if _, err := project.ReadFile(w.p.Root, file+".dockerignore"); err == nil || !os.IsNotExist(err) {
		ignore = file + ".dockerignore"
	}
	b, err := w.read(ignore)
	if err != nil {
		if !os.IsNotExist(err) {
			w.note("dockerignore", ignore+": "+err.Error(), nil)
			m, _ := patternmatcher.New([]string{"**"})
			return m
		}
		return nil
	}
	patterns, err := ignorefile.ReadAll(strings.NewReader(string(b)))
	if err != nil {
		w.note("dockerignore", ignore+": "+err.Error(), nil)
		m, _ := patternmatcher.New([]string{"**"})
		return m
	}
	matcher, err := patternmatcher.New(patterns)
	if err != nil {
		w.note("dockerignore", ignore+": "+err.Error(), nil)
		m, _ := patternmatcher.New([]string{"**"})
		return m
	}
	return matcher
}
func (w *walker) packages(source string, excluded func(string) bool) []string {
	matched := map[string]bool{}
	files := map[string]string{}
	for _, f := range w.code.Files {
		files[f.ID] = f.RelativePath
	}
	for _, n := range w.code.Nodes {
		if n.Source == nil {
			continue
		}
		file := files[n.Source.FileID]
		if file == "" {
			continue
		}
		match := source == "." || file == source || strings.HasPrefix(file, source+"/")
		if !match && strings.ContainsAny(source, "*?[") {
			match, _ = path.Match(source, file)
		}
		if !match || excluded != nil && excluded(file) {
			continue
		}
		id := n.PackageID
		if n.Kind == "package" {
			id = n.ID
		}
		if id != "" {
			matched[id] = true
		}
	}
	out := []string{}
	for id := range matched {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

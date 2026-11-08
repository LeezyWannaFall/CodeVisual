package containers

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"codevisual/internal/graph"
)

type instruction struct {
	name, value string
	line, end   int
}

func instructions(source string) ([]instruction, error) {
	out := []instruction{}
	var current strings.Builder
	start := 0
	escape := "\\"
	for i, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "# escape=") {
			escape = strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[1])
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "<<") {
			return out, fmt.Errorf("Dockerfile heredocs are not expanded")
		}
		if current.Len() == 0 {
			start = i + 1
		}
		continuation := escape != "" && strings.HasSuffix(trimmed, escape)
		if continuation {
			trimmed = strings.TrimSuffix(trimmed, escape)
		}
		current.WriteString(trimmed)
		if continuation {
			current.WriteByte(' ')
			continue
		}
		text := current.String()
		current.Reset()
		split := strings.IndexFunc(text, unicode.IsSpace)
		if split < 0 {
			out = append(out, instruction{strings.ToUpper(text), "", start, i + 1})
		} else {
			out = append(out, instruction{strings.ToUpper(text[:split]), strings.TrimSpace(text[split:]), start, i + 1})
		}
	}
	if current.Len() != 0 {
		return out, fmt.Errorf("unterminated Dockerfile continuation")
	}
	return out, nil
}

// words tokenizes declarations only; no shell expansion or command execution.
func words(text string) ([]string, error) {
	out := []string{}
	var b strings.Builder
	var quote rune
	escaped := false
	started := false
	for _, c := range text {
		if escaped {
			b.WriteRune(c)
			escaped = false
			started = true
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				b.WriteRune(c)
			}
			started = true
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			started = true
			continue
		}
		if unicode.IsSpace(c) {
			if started {
				out = append(out, b.String())
				b.Reset()
				started = false
			}
			continue
		}
		b.WriteRune(c)
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated quoted declaration")
	}
	if started {
		out = append(out, b.String())
	}
	return out, nil
}
func (w *walker) dockerfile(file, contextDir string) []stage {
	key := file + ":" + contextDir
	if stages, ok := w.builds[key]; ok {
		return stages
	}
	w.builds[key] = nil
	b, err := w.read(file)
	if err != nil {
		w.note("dockerfile_read", file+": "+err.Error(), nil)
		return nil
	}
	items, err := instructions(string(b))
	if err != nil {
		w.note("dockerfile_parse", file+": "+err.Error(), location(file, 1, 1))
		return nil
	}
	stages := []stage{}
	byName := map[string]int{}
	matcher := w.ignore(contextDir, file)
	excluded := func(name string) bool {
		if matcher == nil {
			return false
		}
		relative := strings.TrimPrefix(name, strings.TrimSuffix(contextDir, "/")+"/")
		if contextDir == "." {
			relative = name
		}
		ignored, e := matcher.MatchesOrParentMatches(filepath.FromSlash(relative))
		return e != nil || ignored
	}
	for _, item := range items {
		if w.ctx.Err() != nil {
			return nil
		}
		source := location(file, item.line, item.end)
		args, err := words(item.value)
		if err != nil {
			w.note("dockerfile_parse", file+": "+err.Error(), source)
			continue
		}
		if item.name == "FROM" {
			for len(args) > 0 && strings.HasPrefix(args[0], "--") {
				args = args[1:]
			}
			if len(args) == 0 {
				w.note("dockerfile_from", file+": missing FROM image", source)
				continue
			}
			name := strconv.Itoa(len(stages))
			if len(args) >= 3 && strings.EqualFold(args[1], "AS") {
				name = args[2]
			}
			data := newContainer("stage")
			data.Image = args[0]
			data.Dockerfile = file
			data.BuildContext = contextDir
			data.Stage = name
			if previous, ok := byName[strings.ToLower(args[0])]; ok {
				data = clone(stages[previous].data)
				data.Stage = name
				data.Type = "stage"
			}
			id := graph.ID("dockerStage", key+":"+strconv.Itoa(len(stages)))
			stages = append(stages, stage{id, name, data, source})
			byName[strings.ToLower(name)] = len(stages) - 1
			byName[strconv.Itoa(len(stages)-1)] = len(stages) - 1
			if previous, ok := byName[strings.ToLower(args[0])]; ok && previous < len(stages)-1 {
				w.result.Edges = append(w.result.Edges, graph.Link("builds_from", id, stages[previous].id, source))
			}
			if strings.Contains(args[0], "$") {
				w.note("docker_variables", file+": FROM variable is not expanded", source)
			}
			continue
		}
		if len(stages) == 0 {
			if item.name != "ARG" {
				w.note("dockerfile_stage", file+": instruction before FROM", source)
			}
			continue
		}
		current := &stages[len(stages)-1]
		switch item.name {
		case "WORKDIR":
			if len(args) != 1 {
				w.note("docker_workdir", file+": invalid WORKDIR", source)
				continue
			}
			if path.IsAbs(args[0]) {
				current.data.Workdir = path.Clean(args[0])
			} else {
				current.data.Workdir = path.Join(current.data.Workdir, args[0])
			}
			if strings.Contains(item.value, "$") {
				w.note("docker_variables", file+": WORKDIR variable is not expanded", source)
			}
		case "CMD":
			current.data.Command = item.value
		case "ENTRYPOINT":
			current.data.Entrypoint = item.value
		case "EXPOSE":
			current.data.Ports = append(current.data.Ports, args...)
		case "VOLUME":
			current.data.Volumes = append(current.data.Volumes, item.value)
		case "ENV":
			if len(args) > 0 && !strings.Contains(args[0], "=") {
				current.data.EnvironmentKeys = append(current.data.EnvironmentKeys, args[0])
			} else {
				for _, value := range args {
					current.data.EnvironmentKeys = append(current.data.EnvironmentKeys, strings.SplitN(value, "=", 2)[0])
				}
			}
		case "COPY", "ADD":
			from := ""
			value := item.value
			for strings.HasPrefix(value, "--") {
				end := strings.IndexFunc(value, unicode.IsSpace)
				if end < 0 {
					value = ""
					break
				}
				flag := value[:end]
				value = strings.TrimSpace(value[end:])
				if strings.HasPrefix(flag, "--from=") {
					from = strings.TrimPrefix(flag, "--from=")
				}
				if strings.HasPrefix(flag, "--exclude") || flag == "--parents" {
					w.note("docker_copy_flags", file+": COPY exclude/parents mapping is not expanded", source)
					value = ""
					break
				}
			}
			if value == "" {
				continue
			}
			paths := []string{}
			if strings.HasPrefix(value, "[") {
				err = json.Unmarshal([]byte(value), &paths)
			} else {
				paths, err = words(value)
			}
			if err != nil || len(paths) < 2 {
				w.note("docker_copy", file+": unsupported COPY/ADD declaration", source)
				continue
			}
			destination := paths[len(paths)-1]
			if !path.IsAbs(destination) && current.data.Workdir != "" {
				destination = path.Join(current.data.Workdir, destination)
			}
			for _, src := range paths[:len(paths)-1] {
				content := graph.ContainerContent{Kind: strings.ToLower(item.name), Source: src, Target: destination, From: from, PackageIDs: []string{}}
				if from != "" {
					content.Kind = "stage"
					if index, ok := byName[strings.ToLower(from)]; ok && index < len(stages)-1 {
						w.result.Edges = append(w.result.Edges, graph.Link("copies_from", current.id, stages[index].id, source))
					}
				} else if strings.Contains(src, "://") {
					content.Kind = "remote"
				} else if resolved, ok := local(contextDir, src); ok {
					content.PackageIDs = w.packages(resolved, excluded)
				} else {
					w.note("docker_copy", file+": unresolved or external COPY/ADD source "+src, source)
				}
				current.data.Contents = append(current.data.Contents, content)
			}
		}
	}
	for _, s := range stages {
		w.result.Nodes = append(w.result.Nodes, graph.Node{ID: s.id, Kind: "dockerStage", Label: s.name, Source: s.source, Attributes: graph.Attributes{RelativePath: file, Container: s.data}})
	}
	w.builds[key] = stages
	return stages
}

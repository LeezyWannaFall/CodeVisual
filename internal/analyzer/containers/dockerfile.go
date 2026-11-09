package containers

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
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
	globalArgs := map[string]string{}
	stageVars := []map[string]string{}
	vars := map[string]string{}
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
		if item.name == "ARG" {
			if len(args) == 1 {
				key, value, hasDefault := strings.Cut(args[0], "=")
				if hasDefault {
					expanded, known := expandStatic(value, vars)
					if known {
						vars[key] = expanded
					} else {
						delete(vars, key)
					}
				} else if value, ok := globalArgs[key]; ok {
					vars[key] = value
				} else {
					delete(vars, key)
				}
				if len(stages) == 0 {
					globalArgs = copyVariables(vars)
				}
			}
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
			vars = map[string]string{}
			if previous, ok := byName[strings.ToLower(args[0])]; ok {
				vars = copyVariables(stageVars[previous])
			}
			stageVars = append(stageVars, vars)
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
			expanded, known := expandStatic(args[0], vars)
			if !known {
				current.data.Workdir = ""
				w.note("docker_variables", file+": WORKDIR contains an unknown variable", source)
				continue
			}
			args[0] = expanded
			if path.IsAbs(args[0]) {
				current.data.Workdir = path.Clean(args[0])
			} else {
				current.data.Workdir = path.Join(current.data.Workdir, args[0])
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
			previous := copyVariables(vars)
			if len(args) > 0 && !strings.Contains(args[0], "=") {
				value, known := expandStatic(strings.Join(args[1:], " "), previous)
				if known {
					vars[args[0]] = value
				} else {
					delete(vars, args[0])
				}
			} else {
				for _, arg := range args {
					key, value, _ := strings.Cut(arg, "=")
					value, known := expandStatic(value, previous)
					if known {
						vars[key] = value
					} else {
						delete(vars, key)
					}
				}
			}
			if len(args) > 0 && !strings.Contains(args[0], "=") {
				current.data.EnvironmentKeys = append(current.data.EnvironmentKeys, args[0])
			} else {
				for _, value := range args {
					current.data.EnvironmentKeys = append(current.data.EnvironmentKeys, strings.SplitN(value, "=", 2)[0])
				}
			}
		case "COPY", "ADD":
			from := ""
			parents := false
			excludes := []string{}
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
				if flag == "--parents" {
					parents = true
				}
				if strings.HasPrefix(flag, "--exclude=") {
					excludes = append(excludes, strings.Trim(strings.TrimPrefix(flag, "--exclude="), "\"'"))
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
			known := true
			for i, value := range paths {
				var ok bool
				paths[i], ok = expandStatic(value, vars)
				known = known && ok
			}
			if !known {
				w.note("docker_variables", file+": COPY/ADD contains unknown variables", source)
				continue
			}
			invalidExclude := false
			for i, pattern := range excludes {
				expanded, known := expandStatic(pattern, vars)
				if !known {
					invalidExclude = true
				}
				excludes[i] = expanded
				if _, err := path.Match(expanded, "probe"); err != nil {
					invalidExclude = true
				}
			}
			if invalidExclude {
				w.note("docker_copy_flags", file+": invalid exclude pattern", source)
				continue
			}
			destination := paths[len(paths)-1]
			if !path.IsAbs(destination) && current.data.Workdir != "" {
				destination = path.Join(current.data.Workdir, destination)
			}
			for _, src := range paths[:len(paths)-1] {
				content := graph.ContainerContent{Kind: strings.ToLower(item.name), Source: src, Target: destination, From: from, PackageIDs: []string{}}
				if from != "" {
					if parents || len(excludes) > 0 {
						w.note("docker_copy_flags", file+": stage copy path flags require an unknown stage filesystem", source)
					}
					content.Kind = "stage"
					if index, ok := byName[strings.ToLower(from)]; ok && index < len(stages)-1 {
						w.result.Edges = append(w.result.Edges, graph.Link("copies_from", current.id, stages[index].id, source))
					}
				} else if strings.Contains(src, "://") {
					content.Kind = "remote"
				} else if resolved, ok := local(contextDir, src); ok && !path.IsAbs(src) && (contextDir == "." || resolved == contextDir || strings.HasPrefix(resolved, contextDir+"/")) {
					copyExcluded := func(name string) bool {
						if excluded(name) {
							return true
						}
						relative := strings.TrimPrefix(name, strings.TrimSuffix(contextDir, "/")+"/")
						if contextDir == "." {
							relative = name
						}
						for _, pattern := range excludes {
							for candidate := relative; candidate != "."; candidate = path.Dir(candidate) {
								if match, _ := path.Match(pattern, candidate); match {
									return true
								}
							}
						}
						return false
					}
					if parents {
						pivot := ""
						if index := strings.LastIndex(src, "/./"); index >= 0 {
							pivot = strings.TrimPrefix(path.Clean(src[:index+1]), "./") + "/"
							if pivot == "./" {
								pivot = ""
							}
						}
						mappedFiles := w.packageFiles(resolved, copyExcluded)
						for _, codeFile := range w.code.Files {
							ids := mappedFiles[codeFile.RelativePath]
							if len(ids) == 0 {
								continue
							}
							relative := strings.TrimPrefix(codeFile.RelativePath, strings.TrimSuffix(contextDir, "/")+"/")
							if contextDir == "." {
								relative = codeFile.RelativePath
							}
							target := strings.TrimPrefix(relative, pivot)
							mapped := content
							mapped.Source = relative
							mapped.Target = path.Join(destination, target)
							mapped.PackageIDs = ids
							current.data.Contents = append(current.data.Contents, mapped)
						}
						continue
					}
					content.PackageIDs = w.packages(resolved, copyExcluded)
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

var staticVariable = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

func copyVariables(values map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range values {
		result[key] = value
	}
	return result
}
func expandStatic(value string, values map[string]string) (string, bool) {
	known := true
	expanded := staticVariable.ReplaceAllStringFunc(value, func(token string) string {
		name := strings.Trim(strings.TrimPrefix(token, "$"), "{}")
		replacement, ok := values[name]
		if !ok || strings.Contains(replacement, "$") {
			known = false
		}
		return replacement
	})
	if strings.Contains(expanded, "$") {
		known = false
	}
	return expanded, known
}

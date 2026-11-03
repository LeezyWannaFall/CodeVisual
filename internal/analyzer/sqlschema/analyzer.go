// Package sqlschema inventories PostgreSQL DDL declarations, never executing migrations.
package sqlschema

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codevisual/internal/graph"
	"codevisual/internal/project"
)

type Analyzer struct{}
type token struct {
	text       string
	start, end int
	quoted     bool
}

func (t token) is(word string) bool { return !t.quoted && strings.EqualFold(t.text, word) }
func lex(source string) ([]token, error) {
	out := []token{}
	for i := 0; i < len(source); {
		start := i
		c := source[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if strings.HasPrefix(source[i:], "--") {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(source[i:], "/*") {
			i += 2
			depth := 1
			for i < len(source) && depth > 0 {
				if strings.HasPrefix(source[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(source[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth > 0 {
				return out, fmt.Errorf("unterminated SQL comment")
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote := c
			i++
			closed := false
			for i < len(source) {
				if source[i] == quote {
					if i+1 < len(source) && source[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return out, fmt.Errorf("unterminated SQL quote")
			}
			out = append(out, token{source[start:i], start, i, true})
			continue
		}
		if c == '$' {
			end := i + 1
			for end < len(source) && ((source[end] >= 'a' && source[end] <= 'z') || (source[end] >= 'A' && source[end] <= 'Z') || (source[end] >= '0' && source[end] <= '9') || source[end] == '_') {
				end++
			}
			if end < len(source) && source[end] == '$' {
				tag := source[i : end+1]
				tail := strings.Index(source[end+1:], tag)
				if tail < 0 {
					return out, fmt.Errorf("unterminated dollar-quoted SQL")
				}
				i = end + 1 + tail + len(tag)
				out = append(out, token{source[start:i], start, i, true})
				continue
			}
		}
		if strings.ContainsRune("(),;.[]", rune(c)) {
			i++
			out = append(out, token{string(c), start, i, false})
			continue
		}
		i++
		for i < len(source) && !strings.ContainsRune(" \t\r\n(),;.[]'\"", rune(source[i])) && !strings.HasPrefix(source[i:], "--") && !strings.HasPrefix(source[i:], "/*") {
			i++
		}
		out = append(out, token{source[start:i], start, i, false})
	}
	return out, nil
}
func identifier(tokens []token, i int) (string, int) {
	if i >= len(tokens) || strings.ContainsAny(tokens[i].text, "();,") {
		return "", i
	}
	normalize := func(t token) string {
		if t.quoted && strings.HasPrefix(t.text, `"`) {
			return t.text
		}
		return strings.ToLower(t.text)
	}
	value := normalize(tokens[i])
	i++
	for i+1 < len(tokens) && tokens[i].text == "." {
		value += "." + normalize(tokens[i+1])
		i += 2
	}
	return value, i
}
func text(tokens []token) string {
	parts := []string{}
	for _, t := range tokens {
		parts = append(parts, t.text)
	}
	return strings.Join(parts, " ")
}
func segments(tokens []token) [][]token {
	out := [][]token{}
	start, depth := 0, 0
	for i, t := range tokens {
		if t.text == "(" {
			depth++
		}
		if t.text == ")" {
			depth--
		}
		if t.text == "," && depth == 0 {
			out = append(out, tokens[start:i])
			start = i + 1
		}
	}
	if start < len(tokens) {
		out = append(out, tokens[start:])
	}
	return out
}
func pair(tokens []token, open int) int {
	depth := 0
	for i := open; i < len(tokens); i++ {
		if tokens[i].text == "(" {
			depth++
		}
		if tokens[i].text == ")" {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
func position(source string, offset int) (int, int) {
	line := strings.Count(source[:offset], "\n") + 1
	last := strings.LastIndex(source[:offset], "\n")
	return line, offset - last
}
func sourceRange(source string, from, to int, fileID string) *graph.SourceRange {
	a, b := position(source, from)
	c, d := position(source, to)
	return &graph.SourceRange{FileID: fileID, StartLine: a, StartColumn: b, EndLine: c, EndColumn: d}
}

func (Analyzer) Analyze(ctx context.Context, p *project.Project) (graph.AnalysisResult, error) {
	r := graph.AnalysisResult{Nodes: []graph.Node{}, Edges: []graph.Edge{}, Files: []graph.File{}, Diagnostics: []graph.Diagnostic{}}
	paths := []string{}
	err := filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != p.Root {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
					return filepath.SkipDir
				}
				if _, e := os.Stat(filepath.Join(path, "go.mod")); e == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if d.Type()&os.ModeSymlink == 0 && strings.HasSuffix(name, ".sql") && !strings.HasSuffix(name, ".down.sql") {
			rel, _ := filepath.Rel(p.Root, path)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return r, err
	}
	sort.Strings(paths)
	for _, path := range paths {
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		b, err := project.ReadFile(p.Root, path)
		if err != nil {
			r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "sql_read", Message: path + ": " + err.Error()})
			continue
		}
		fullSource := string(b)
		source := upSection(fullSource)
		fileID := graph.ID("file", path)
		r.Files = append(r.Files, graph.File{ID: fileID, RelativePath: path, ContentHash: graph.Hash(b)})
		tokens, err := lex(source)
		if err != nil {
			r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "sql_parse", Message: path + ": " + err.Error()})
		}
		start := 0
		for i, t := range tokens {
			if t.text != ";" {
				continue
			}
			parseStatement(&r, path, source, fileID, tokens[start:i])
			start = i + 1
		}
		if start < len(tokens) {
			parseStatement(&r, path, source, fileID, tokens[start:])
		}
	}
	// Connect only unambiguous table declarations. No migration ordering is assumed.
	tables := map[string][]string{}
	for _, n := range r.Nodes {
		if n.Attributes.Table != nil {
			tables[n.Attributes.Table.Name] = append(tables[n.Attributes.Table.Name], n.ID)
		}
	}
	for _, n := range r.Nodes {
		if n.Attributes.Index != nil {
			if ids := tables[n.Attributes.Index.Table]; len(ids) == 1 {
				r.Edges = append(r.Edges, graph.Link("indexes", n.ID, ids[0], n.Source))
			}
		}
		if n.Attributes.Table != nil {
			for _, ref := range n.Attributes.Table.References {
				if ids := tables[ref.Table]; len(ids) == 1 {
					e := graph.Link("references", n.ID, ids[0], n.Source)
					duplicate := false
					for _, old := range r.Edges {
						if old.ID == e.ID {
							duplicate = true
						}
					}
					if !duplicate {
						r.Edges = append(r.Edges, e)
					}
				}
			}
		}
	}
	return r, nil
}
func upSection(source string) string {
	offset := 0
	for _, line := range strings.SplitAfter(source, "\n") {
		trim := strings.ToLower(strings.TrimSpace(line))
		if trim == "-- +goose down" || trim == "-- +migrate down" {
			return source[:offset]
		}
		offset += len(line)
	}
	return source
}
func parseStatement(r *graph.AnalysisResult, path, source, fileID string, t []token) {
	if len(t) == 0 {
		return
	}
	loc := sourceRange(source, t[0].start, t[len(t)-1].end, fileID)
	note := func(message string) {
		r.Diagnostics = append(r.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "sql_unsupported", Message: path + ": " + message, Source: loc})
	}
	if t[0].is("alter") || t[0].is("drop") {
		note("ALTER/DROP is not applied; Database lists DDL declarations, not the final migrated schema")
		return
	}
	if !t[0].is("create") {
		return
	}
	i := 1
	unique := false
	if i < len(t) && t[i].is("unique") {
		unique = true
		i++
	}
	if i < len(t) && (t[i].is("unlogged") || t[i].is("temporary") || t[i].is("temp")) {
		i++
	}
	if i >= len(t) {
		return
	}
	kind := t[i]
	i++
	if kind.is("index") && i < len(t) && t[i].is("concurrently") {
		i++
	}
	if i+2 < len(t) && t[i].is("if") && t[i+1].is("not") && t[i+2].is("exists") {
		i += 3
	}
	if kind.is("table") {
		name, next := identifier(t, i)
		i = next
		if name == "" || i >= len(t) || t[i].text != "(" {
			note("Unsupported CREATE TABLE shape")
			return
		}
		end := pair(t, i)
		if end < 0 {
			note("Unbalanced CREATE TABLE parentheses")
			return
		}
		table := &graph.SQLTable{Name: name, Columns: []graph.SQLColumn{}, Constraints: []string{}, References: []graph.SQLReference{}}
		for _, part := range segments(t[i+1 : end]) {
			if len(part) == 0 {
				continue
			}
			constraint := part[0].is("constraint") || part[0].is("primary") || part[0].is("foreign") || part[0].is("unique") || part[0].is("check") || part[0].is("exclude")
			columnName := ""
			if constraint {
				table.Constraints = append(table.Constraints, text(part))
			} else {
				var n int
				columnName, n = identifier(part, 0)
				stop := n
				depth := 0
				for stop < len(part) {
					tok := part[stop]
					if depth == 0 && (tok.is("not") || tok.is("null") || tok.is("default") || tok.is("primary") || tok.is("references") || tok.is("unique") || tok.is("check") || tok.is("constraint") || tok.is("generated") || tok.is("collate")) {
						break
					}
					if tok.text == "(" {
						depth++
					}
					if tok.text == ")" {
						depth--
					}
					stop++
				}
				table.Columns = append(table.Columns, graph.SQLColumn{Name: columnName, Type: text(part[n:stop]), Constraints: text(part[stop:])})
			}
			for j, tok := range part {
				if !tok.is("references") {
					continue
				}
				target, next := identifier(part, j+1)
				targetCols := ""
				if next < len(part) && part[next].text == "(" {
					if end := pair(part, next); end >= 0 {
						targetCols = text(part[next+1 : end])
					}
				}
				columns := columnName
				if constraint {
					for k := 0; k < j; k++ {
						if part[k].text == "(" {
							if end := pair(part, k); end >= 0 {
								columns = text(part[k+1 : end])
							}
							break
						}
					}
				}
				table.References = append(table.References, graph.SQLReference{Columns: columns, Table: target, TargetColumns: targetCols})
			}
		}
		members := []string{}
		for _, c := range table.Columns {
			members = append(members, strings.TrimSpace(c.Name+" "+c.Type+" "+c.Constraints))
		}
		id := graph.ID("table", path+":"+name+":"+fmt.Sprint(t[0].start))
		r.Nodes = append(r.Nodes, graph.Node{ID: id, Kind: "table", Label: name, Source: loc, Attributes: graph.Attributes{RelativePath: path, Signature: source[t[0].start:t[len(t)-1].end], Members: members, Table: table}})
		return
	}
	if kind.is("index") {
		name, next := identifier(t, i)
		i = next
		if i >= len(t) || !t[i].is("on") {
			note("Unsupported CREATE INDEX shape")
			return
		}
		i++
		if i < len(t) && t[i].is("only") {
			i++
		}
		table, _ := identifier(t, i)
		idx := &graph.SQLIndex{Name: name, Table: table, Unique: unique, Definition: source[t[0].start:t[len(t)-1].end]}
		id := graph.ID("index", path+":"+name+":"+fmt.Sprint(t[0].start))
		r.Nodes = append(r.Nodes, graph.Node{ID: id, Kind: "index", Label: name, Source: loc, Attributes: graph.Attributes{RelativePath: path, Signature: idx.Definition, Index: idx}})
		return
	}
	note("CREATE " + kind.text + " is outside the supported TABLE/INDEX inventory")
}

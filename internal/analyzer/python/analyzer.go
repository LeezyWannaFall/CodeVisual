// Package python parses Python source with an isolated standard-library AST helper.
// It never imports or executes analyzed modules.
package python

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"codevisual/internal/graph"
	"codevisual/internal/project"
)

//go:embed parser.py
var parserSource string

type Analyzer struct{}

func (Analyzer) Analyze(ctx context.Context, p *project.Project) (graph.AnalysisResult, error) {
	result := graph.AnalysisResult{}
	if len(p.PythonFiles) == 0 {
		return result, nil
	}
	var executable string
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			probe := exec.CommandContext(ctx, path, "-I", "-S", "-c", "import sys; sys.exit(0 if sys.version_info >= (3, 9) else 1)")
			if probe.Run() == nil {
				executable = path
				break
			}
		}
	}
	if executable == "" {
		return result, fmt.Errorf("Python source detected: install Python 3.9 or newer and make python3 or python available in PATH")
	}
	type inputFile struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Hash    string `json:"hash"`
	}
	files := []inputFile{}
	for _, name := range p.PythonFiles {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		b, err := project.ReadFile(p.Root, name)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, graph.Diagnostic{Severity: "warning", Code: "python_read", Message: name + ": " + err.Error()})
			continue
		}
		files = append(files, inputFile{name, base64.StdEncoding.EncodeToString(b), graph.Hash(b)})
	}
	payload, err := json.Marshal(struct {
		ModulePath string      `json:"modulePath"`
		RootName   string      `json:"rootName"`
		Files      []inputFile `json:"files"`
	}{p.ModulePath, p.RootName, files})
	if err != nil {
		return result, err
	}
	cmd := exec.CommandContext(ctx, executable, "-I", "-S", "-c", parserSource)
	// -I excludes project paths and PYTHON* hooks; -S suppresses site customizations.
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, fmt.Errorf("Python AST parser failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var parsed graph.AnalysisResult
	if err = json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return result, fmt.Errorf("Python parser response: %w", err)
	}
	return graph.Merge(result, parsed), nil
}

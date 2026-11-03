// Package analyzer defines the small contract shared by independent analyzers.
package analyzer

import (
	"codevisual/internal/graph"
	"codevisual/internal/project"
	"context"
)

type Analyzer interface {
	Analyze(context.Context, *project.Project) (graph.AnalysisResult, error)
}

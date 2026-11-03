// Package cli owns process lifecycle; analysis and serving remain independent.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"codevisual/internal/analyzer"
	"codevisual/internal/analyzer/callgraph"
	goanalyzer "codevisual/internal/analyzer/golang"
	"codevisual/internal/analyzer/httproutes"
	"codevisual/internal/analyzer/sqlschema"
	"codevisual/internal/analyzer/symbols"
	"codevisual/internal/graph"
	"codevisual/internal/project"
	"codevisual/internal/server"
)

type Options struct {
	Path      string
	Port      int
	NoBrowser bool
	Tags      []string
}

func Parse(args []string, out io.Writer) (Options, error) {
	opts := Options{Port: 7331}
	if len(args) == 0 || args[0] != "analyze" {
		return opts, fmt.Errorf("usage: codevisual analyze PATH [--port 7331] [--no-browser] [--tags tag1,tag2]")
	}
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.IntVar(&opts.Port, "port", 7331, "local port")
	fs.BoolVar(&opts.NoBrowser, "no-browser", false, "do not open a browser")
	tags := fs.String("tags", "", "comma-separated Go build tags")
	// The standard flag package stops at the first positional argument. Move PATH
	// after flags so the documented PATH-first spelling also works.
	rest := args[1:]
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		opts.Path = rest[0]
		rest = rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return opts, err
	}
	if opts.Path == "" && fs.NArg() == 1 {
		opts.Path = fs.Arg(0)
	} else if fs.NArg() != 0 {
		return opts, fmt.Errorf("expected exactly one project path")
	}
	if opts.Path == "" {
		return opts, fmt.Errorf("project path is required")
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return opts, fmt.Errorf("port must be between 1 and 65535")
	}
	if *tags != "" {
		for _, t := range strings.Split(*tags, ",") {
			t = strings.TrimSpace(t)
			if t == "" || strings.ContainsAny(t, " \t\r\n") {
				return opts, fmt.Errorf("invalid build tag")
			}
			opts.Tags = append(opts.Tags, t)
		}
	}
	return opts, nil
}
func Run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := Parse(args, out)
	if err != nil {
		return err
	}
	authority := net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.Port))
	listener, err := net.Listen("tcp4", authority)
	if err != nil {
		return fmt.Errorf("cannot listen on %s; choose another --port: %w", authority, err)
	}
	defer listener.Close()
	start := time.Now()
	fmt.Fprintf(out, "Analyzing %s (offline)…\n", opts.Path)
	p, err := project.Discover(ctx, opts.Path, opts.Tags)
	if err != nil {
		return err
	}
	if err = p.Load(ctx); err != nil {
		return err
	}
	index := symbols.New(p)
	results := []graph.AnalysisResult{}
	var routes graph.AnalysisResult
	for _, worker := range []analyzer.Analyzer{goanalyzer.Analyzer{}, httproutes.Analyzer{Index: index}, sqlschema.Analyzer{}, callgraph.Analyzer{Index: index}} {
		result, err := worker.Analyze(ctx, p)
		if err != nil {
			return err
		}
		if _, ok := worker.(httproutes.Analyzer); ok {
			routes = result
		}
		results = append(results, result)
	}
	results = append(results, callgraph.ConnectRoutes(routes, index))
	result := graph.Merge(results...)
	snapshot, err := graph.NewSnapshot(p.Project, result)
	if err != nil {
		return err
	}
	// Keep only the compact snapshot after analysis.
	p = nil
	debug.FreeOSMemory()
	fmt.Fprintf(out, "Analyzed %d nodes, %d edges in %s; status: %s (%d diagnostics)\n", len(snapshot.Nodes), len(snapshot.Edges), time.Since(start).Round(time.Millisecond), snapshot.Status, len(snapshot.Diagnostics))
	srv := &http.Server{Handler: server.New(snapshot, authority), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(listener) }()
	localURL := "http://" + authority
	fmt.Fprintf(out, "CodeVisual → %s\nPress Ctrl+C to stop.\n", localURL)
	if !opts.NoBrowser { // Serve is running before an OS browser is launched.
		if err := openBrowser(localURL); err != nil {
			fmt.Fprintf(out, "Could not open browser: %v. Open %s manually.\n", err, localURL)
		}
	}
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return err
		}
		err := <-done
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

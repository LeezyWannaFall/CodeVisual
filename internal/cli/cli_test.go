package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, args := range [][]string{{"analyze", "/project", "--port", "7441", "--no-browser", "--tags", "one,two"}, {"analyze", "--port=7441", "--no-browser", "--tags=one,two", "/project"}} {
		o, err := Parse(args, io.Discard)
		if err != nil || o.Path != "/project" || o.Port != 7441 || !o.NoBrowser || len(o.Tags) != 2 {
			t.Fatalf("%v: %+v %v", args, o, err)
		}
	}
	for _, args := range [][]string{nil, {"analyze"}, {"other", "."}, {"analyze", ".", "extra"}, {"analyze", ".", "--port=0"}, {"analyze", ".", "--tags=a,,b"}} {
		if _, err := Parse(args, io.Discard); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestVersionDoesNotRequireGo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, arg := range []string{"--version", "version"} {
		var out bytes.Buffer
		if err := Run(context.Background(), []string{arg}, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "CodeVisual "+Version) {
			t.Fatalf("version output: %s", out.String())
		}
	}
}

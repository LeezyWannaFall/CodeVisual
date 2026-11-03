package sqlschema

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codevisual/internal/graph"
	"codevisual/internal/project"
)

func analyzeSQL(t *testing.T, files map[string]string) graph.AnalysisResult {
	t.Helper()
	root := t.TempDir()
	for path, source := range files {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p := &project.Project{Project: graph.Project{Root: root, ModulePath: "example.test/app"}}
	r, err := (Analyzer{}).Analyze(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestPostgresDDLAndDownExclusion(t *testing.T) {
	r := analyzeSQL(t, map[string]string{
		"migrations/001.sql": `-- +goose Up
CREATE TABLE public.users (
 id BIGINT PRIMARY KEY,
 name TEXT NOT NULL DEFAULT 'a,b; CREATE TABLE fake(x int)',
 balance numeric(12, 2) DEFAULT 0,
 CONSTRAINT positive CHECK (balance >= 0)
);
CREATE TABLE public.orders (
 id BIGINT PRIMARY KEY,
 user_id BIGINT REFERENCES public.users(id),
 CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES public.users(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS users_name ON public.users USING btree (name);
-- +goose Down
DROP TABLE public.users;
CREATE TABLE must_not_appear(id int);`,
		"migrations/002.down.sql": "CREATE TABLE also_not_visible(id int);",
		"migrations/003.sql": `/* nested /* CREATE TABLE fake(x int); */ comment */
DO $$ BEGIN PERFORM 'CREATE TABLE fake(x int);'; END $$;
CREATE TABLE "Odd Name" ("Select" text, items text[], PRIMARY KEY ("Select"));`,
		"vendor/schema.sql": "CREATE TABLE vendor_hidden(id int);",
	})
	if len(r.Nodes) != 4 || len(r.Diagnostics) != 0 {
		t.Fatalf("unexpected SQL result: nodes=%d diagnostics=%+v", len(r.Nodes), r.Diagnostics)
	}
	tables := map[string]*graph.SQLTable{}
	for _, n := range r.Nodes {
		if n.Source == nil || n.Source.StartLine < 1 {
			t.Fatal("missing SQL source")
		}
		if n.Attributes.Table != nil {
			tables[n.Label] = n.Attributes.Table
		}
	}
	if len(tables["public.users"].Columns) != 3 || !strings.Contains(tables["public.users"].Columns[2].Type, "numeric") {
		t.Fatal("column type/comma parsing failed")
	}
	orders := tables["public.orders"]
	if len(orders.References) != 2 || orders.References[0].Columns != "user_id" || orders.References[1].TargetColumns != "id" {
		t.Fatalf("foreign keys: %+v", orders.References)
	}
	if _, ok := tables[`"Odd Name"`]; !ok {
		t.Fatal("quoted identifiers lost")
	}
	if len(r.Edges) != 2 {
		t.Fatalf("wanted one references edge and one index edge: %+v", r.Edges)
	}
}
func TestUnsupportedSQLAndMalformedInput(t *testing.T) {
	r := analyzeSQL(t, map[string]string{"ddl.sql": "ALTER TABLE users ADD COLUMN x int; CREATE TABLE AS SELECT 1; CREATE TABLE broken (x text;"})
	if len(r.Nodes) != 0 || len(r.Diagnostics) != 3 {
		t.Fatalf("unsupported DDL not reported: %+v", r)
	}
	if _, err := lex("CREATE TABLE x (name text DEFAULT 'oops"); err == nil {
		t.Fatal("unterminated quote accepted")
	}
	if _, err := lex("/* oops"); err == nil {
		t.Fatal("unterminated comment accepted")
	}
}

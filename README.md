# CodeVisual

Local architecture explorer for Go projects. Analyze package imports, functions,
methods, structs, interfaces, and named types in a dark blue interactive graph.
Separate API and Database sections inventory chi registrations and PostgreSQL DDL.
Everything runs on your machine: no LLM, cloud service, database, or account.

## Quick start

The repository includes the built frontend. **Go 1.26+** is sufficient to build
and run the application; the analyzed project also needs a compatible local Go
toolchain and its dependencies in the local module cache.

```sh
go build -o bin/codevisual ./cmd/codevisual
./bin/codevisual analyze ./path/to/project
```

Open **http://127.0.0.1:7331**. The command opens a browser automatically unless
`--no-browser` is set. Stop the server with Ctrl+C.

```sh
./bin/codevisual analyze ./path/to/project --port 7441 --no-browser
./bin/codevisual analyze ./path/to/project --tags integration,custom
```

Flags may follow the project path or precede it. A busy port is an error; select
another explicitly with `--port`. The listener always uses `127.0.0.1`.

Building CodeVisual may download its development dependencies. **Analyzing a
project never downloads its dependencies or toolchain.** Prepare dependencies
separately if full type information is needed. With missing dependencies or type
errors, CodeVisual preserves usable AST declarations and displays a partial
analysis with diagnostics.

## Explore a project

Use the **Code / API / Database** tabs to switch between Go architecture, HTTP
registrations, and SQL declarations. The inspector can be collapsed using its
header button or the toolbar toggle; the selected item is retained and the
preference survives reloads. Selecting an item opens the inspector again.

The minimap uses matching SVG/container dimensions. Click to recenter, drag to
pan, or scroll to zoom. It remains above the graph legend.

### API

Lists chi `Get`, `Post`, other verb methods, `Handle`/`HandleFunc`, and
`Method`/`MethodFunc` with static paths, handlers, middleware, registration scope,
and source. Supports inline `Route`/`Group`, `With`/`Use`, local router-parameter
helpers, and resolvable `Mount` builders, including builders stored in variables.
Filter by method or search path/handler text. Paths are relative to their named
builder scope unless a supported mount supplies the prefix. Unknown external
mounts, dynamic paths, and conditional/loop registrations appear in diagnostics;
these are not claimed as known runtime endpoints.

### Database

Reads `.sql` files without a database connection. Displays `CREATE TABLE` columns,
table constraints, inline/table foreign keys, and `CREATE [UNIQUE] INDEX`, with
source links. Skips `.down.sql` files and Down sections marked by goose or
sql-migrate. Quoted identifiers, comments, nested parentheses, and dollar-quoted
bodies are tokenized without interpreting text inside strings as DDL.

This is an inventory of DDL declarations, not a final migrated schema. ALTER,
DROP, and unsupported CREATE forms are diagnosed. Migrations are not executed or
ordered to simulate a database. SQL constructed inside Go strings is outside the
inventory. Explicit foreign keys and indexes connect only to an unambiguous
matching table declaration.

### Code

- Select a node to inspect details; double-click a package to explore its symbols.
- Use the directory view to group imports, then double-click a directory to drill
  down. The Architecture breadcrumb returns to the overview.
- The left search finds packages and symbols throughout the module. The graph
  filter searches only the current view.
- Filter symbols by kind or exported visibility; optionally show external imports.
- Drag nodes, pan, zoom, use fit-to-view, or navigate with the minimap. Selecting a
  package highlights direct incoming and outgoing imports.
- Select a declaration to read its source with highlighted lines. If the file has
  changed since analysis, restart the command; stale locations are not reused.
- Click the analysis status in the header to inspect diagnostics.

Each graph response contains at most 200 nodes. Larger modules start in directory
view; pages expose larger groups and symbol lists. Only edges whose endpoints
are visible are shown. Directory edges explicitly count aggregated imports.
Node positions are kept while inspecting the current view, but are not persisted.

## Development

Frontend development additionally requires Node.js 22.16+ and npm. Versions and
lockfiles are pinned. The production result is one Go binary with embedded HTML,
CSS, and JavaScript; Node.js is not needed at runtime.

```sh
make build                  # npm ci when needed → TypeScript/Vite → Go binary
make test                   # Go tests and frontend tests
make dev                    # backend :7331 and Vite :5173; Ctrl+C stops both
```

The Vite development server proxies `/api` to the Go backend. Visit its printed
local URL. It is a development convenience; production uses only the embedded
frontend. Commit regenerated `internal/server/webdist` assets after frontend
changes so a direct `go build` remains usable.

Additional checks:

```sh
go test -race ./...
go vet ./...
cd web
npx playwright install chromium
npm run test:e2e             # build bin/codevisual first; uses port 17331
```

A production smoke test runs the binary from a temporary working directory,
checks embedded assets and APIs, samples process-tree RSS on Linux, and compares
input files before and after analysis:

```sh
python3 scripts/verify_projects.py . /path/to/prepared/module
```

It neither downloads dependencies nor runs analyzed code. See
[verification notes](docs/verification.md) for the initial real-project run.

## Architecture

```text
cmd/codevisual → internal/cli
                      ↓
              internal/project → internal/analyzer/golang
                                          ↓
                                    internal/graph
                                          ↓
                                    internal/server → React
```

- `project` discovers one module and loads syntax/type information with
  `go/packages`; `go/parser` retains available declarations on load failures.
- `analyzer` defines a small interface returning language-independent graph data.
- `graph` validates node/edge IDs and builds an immutable, indexed snapshot plus
  bounded projections. Semantic IDs do not depend on the checkout's absolute path.
- `server` uses chi for the read-only REST API and serves the embedded SPA.
- `web` uses React, TypeScript, Tailwind, XYFlow, and Dagre. There is no runtime CDN.

The Code graph includes module/directory/package containment, package imports,
and explicit method declarations. Independent `httproutes` and `sqlschema`
analyzers add route/table/index nodes to the same snapshot; dedicated bounded
projections keep them separate from the Code graph. Interface members and struct fields appear in
symbol details. It does **not** infer calls or interface implementations.
See [API contract](docs/api.md) for DTOs and endpoints.

## Analysis boundaries and safety

- One module only. A path inside a module resolves to its root. Workspaces are
  ignored with a diagnostic; nested modules are skipped.
- The build context is the host OS/architecture and explicit `--tags`, with cgo
  disabled. No attempt is made to combine all possible build configurations.
- Tests, `.git`, hidden directories, `vendor`, `node_modules`, `testdata`, and
  symlinked directories are excluded. Symlinked source files are not read.
- External packages, including the standard library, are import references only;
  their symbols and source are outside the analysis.
- Source programs, tests, generators, cgo tools, and custom package drivers are
  not executed. Loading may invoke the installed Go toolchain and use Go caches.
- Module metadata is copied to a private temporary directory for `-modfile`,
  preventing changes to the project's `go.mod` and `go.sum`. Relative `replace`
  directives retain their normal module-root meaning.
- Go network lookups and automatic toolchain downloads are disabled. Proxy bypass
  settings and package-driver hooks are overridden. Toolchain telemetry is
  isolated in a temporary directory with mode `off`, using Go's telemetry-directory
  hook; global user telemetry settings are not changed.
- The API validates Host and Origin, does not enable CORS, and only reads files
  registered in the snapshot through an OS-constrained root and hash validation.
- Source positions use one-based lines and byte columns. Syntax/type errors and
  uncertain fallback configuration are visible in diagnostics.

This is static analysis of one build configuration. A partial graph is useful,
but should be interpreted together with its diagnostics.

Request flow, live database connections, migration execution, Docker
infrastructure, live reload, persistence, IDE integration, and plugin loading
remain outside this release. New analyzers can later return nodes, edges, and diagnostics through the
existing analyzer interface without changing the CLI or frontend transport.

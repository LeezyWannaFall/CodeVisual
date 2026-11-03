# CodeVisual

Local architecture explorer for Go projects. Analyze package imports, functions,
methods, structs, interfaces, and named types in a black interactive interface with blue accents.
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

Use the **Code / API / Database / Request Flow** tabs to switch between Go architecture, HTTP
registrations, and SQL declarations. The inspector can be collapsed using its
header button or the toolbar toggle; the selected item is retained and the
preference survives reloads. Selecting an item opens the inspector again.

Cards have fixed positions. Drag the background to pan, scroll to zoom, or use
the fit-to-view controls. Selecting a card animates its incident relationships
with moving dashed lines. The graph does not include a minimap.

### API

Lists chi `Get`, `Post`, other verb methods, `Handle`/`HandleFunc`, and
`Method`/`MethodFunc` with static paths, handlers, middleware, registration scope,
and source. Supports inline and resolved named `Route`/`Group` callbacks, typed chi router
aliases, `With`/`Use`, middleware helpers, delegated routers, and resolvable `Mount`
builders, including builders stored in variables.
Use **Open handler** to inspect a resolved handler declaration and open middleware
source from the inspector. A factory returning a known function links to that
returned function; **Open factory** separately opens its construction code.
Custom `http.Handler` objects resolve to their `ServeHTTP` method when known.
Filter by method or search path/handler text. Paths are relative to their named
builder scope unless a supported mount supplies the prefix. Unknown external
mounts, dynamic paths, and conditional/loop registrations appear in diagnostics;
these are not claimed as known runtime endpoints.

### Database

The Database tab opens its own **Map** of tables and indexes. Table cards show
columns and PK/FK markers; arrows follow declared foreign keys to referenced
tables and indexes to their tables. Cards remain fixed, with the same pan/zoom,
selection, and optional neighbor-focus controls as the Code graph. **List** keeps
the detailed DDL inventory available alongside the map.

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
- Pan the background, zoom, or use fit-to-view. Selecting a
  package highlights its relationships and opens its inspector; the graph stays complete.
- Select a declaration to read its source with highlighted lines. The inspector
  automatically widens for source and wraps long lines by default. Toggle
  **Wrap lines** to use horizontal scrolling instead. Line numbers and indentation
  remain tied to the original file. If the file has
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

Optional browser checks can also use local sibling checkouts:

```sh
cd web
CODEVISUAL_TEST_SELF=1 CODEVISUAL_TEST_BOOKING=1 CODEVISUAL_TEST_SHORTENER=1 npm run test:e2e
```

The optional scenarios expect `../Room-Booking-Service` and `../Go-URL-Shortener`
next to this repository, with dependencies already available locally.

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
- `web` uses React, TypeScript, Tailwind, XYFlow, and ELK. There is no runtime CDN.

The Code graph includes module/directory/package containment, package imports,
and explicit method declarations. Independent `httproutes` and `sqlschema`
analyzers add route/table/index nodes to the same snapshot; dedicated bounded
projections keep them separate from the Code graph. Interface members and struct fields appear in
symbol details. Call relationships belong to the separate Request Flow
projection; interface implementation matches are explicitly marked as possible.
The read-only API uses `/api/v1`: `/project`, `/tree`, `/graph`, `/search`,
`/nodes/{id}`, `/files/{id}`, and `/diagnostics`. Graph projections use `level`;
Request Flow additionally accepts `rootId`, `depth`, and `includePossible`.

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

Live database connections, migration execution, Docker
infrastructure, live reload, persistence, IDE integration, and plugin loading
remain outside this release. New analyzers can later return nodes, edges, and diagnostics through the
existing analyzer interface without changing the CLI or frontend transport.

### Reading dependency diagrams

The Code view uses layered orthogonal routing with a separate input/output port
for each relationship. Arrow `A → B` means **A imports B**; it does not represent
request execution. Layout runs in an embedded local Web Worker.

Clicking a package highlights it and opens its inspector without changing the
active graph mode or card positions. **All connections** is the default. To open
a compact graph of the selected package and its direct neighbors, explicitly
click **Selected node**. **Incoming & outgoing** can then be narrowed to either
direction; **All connections** restores the complete loaded view. Incoming
relationships are light blue and outgoing relationships are a stronger blue. Counters refer to the loaded graph
before the focus filter; the summary shows how many nodes and edges remain.

Card dragging is disabled, preserving the automatic layout and its routed lines.
Selected relationships use animated dashed strokes; unrelated relationships stay
solid. Reduced-motion preferences disable the animation. The viewport refits
when the inspector expands or collapses.


### Request Flow

Open **Request flow** from an API route or a callable's inspector, or use the
Request Flow tab and choose an entry point. The diagram follows source call sites
from the registered handler through module-local functions and methods. Each node
opens its source; each edge retains representative call-site evidence in the API.

This is a **static potential-call graph**, not an executed request trace or a
promise of branch order. Direct calls use resolved Go symbols. Interface dispatch
stops at an **INTERFACE** boundary by default. Enable **Possible implementations**
to explore local methods satisfying that interface; these nodes and dashed edges
are marked possible, never confirmed runtime bindings. Dynamic function values
stop at **DYNAMIC** boundaries. External calls are hidden by default and can be
shown as terminal **EXTERNAL** nodes.

Depth is configurable from 1 to 10 (default 6), with at most 200 visible nodes.
The notice reports truncation and omitted external/candidate targets. Recursive
calls terminate safely. Calls inside a closure belong to that closure; constructing
an unused closure does not make its body part of a handler. `go` and `defer` call
sites are distinguished in edge metadata/tooltips. Factory execution during route
registration is not placed on the request-time path.

Analysis remains offline and source-only. It does not run handlers or middleware,
perform pointer analysis, prove injected interface bindings, expand unknown
higher-order callbacks, or trace across dependency source outside the module.


Package symbols open in a compact **Overview**: types own collapsible lists of
their declared methods. Interfaces, free functions, and remaining declarations
share a separate compact list; fields and contracts appear in the inspector. Selecting a method
opens its source; **Declaration graph** retains the full structural diagram.
Grouping uses declaration ownership only, not inferred calls or naming patterns.
Filters and paging apply to the current set of declarations, so a filtered-out
owner leaves its matching methods in the remaining-declarations list.


Package/directory dependency maps default to **Simplified**: a direct import line
is hidden when another dependency path connects the same packages in the current
view. All packages and dependency reachability are preserved; cyclic groups keep
their internal edges. This is a visual reduction, not a removal of real imports.
**All imports** restores every direct relationship. Explicit **Selected node**
focus always shows that package's actual direct connections. Guidance and controls
occupy a dedicated header above the canvas.

Selecting a package on the package map loads its **Package overview below the
map**, on the same scrollable page. Expand types there and select declarations to
open source in the inspector; the package map stays in place. Double-click and
Explorer navigation still offer a dedicated symbol view. Inspector members and
signatures use formatted Go code blocks.

The database map uses compact ER table cards with yellow primary keys and blue
foreign keys. Relationships attach to their declared source and target column
rows, including composite keys. Indexes appear within their table; their complete
DDL and source remain available in List. Unknown target columns stay at the table
header rather than being attached to an assumed field. No cardinality is inferred.

Project sections are arranged vertically above Explorer. Inspector controls are
local to the right panel. Interfaces share the compact **Functions & other
declarations** list; selecting one opens its definition/source in the inspector.
Table index footers contain subdued, keyboard-accessible buttons that open the
actual index and its DDL without selecting the table or changing map views.

Explorer folder labels expand their branch; directories containing a single Go
package open that package's declarations directly. Package-map selection preserves
scroll position during and after loading a replacement overview. Inspector controls
are local to the right panel, and source centering scrolls only the code area.

Go source, declaration signatures, fields/members, and route handler expressions
share local syntax highlighting. Multiline raw strings/comments and struct tags
retain their colors across lines. Source text, line numbers, wrapping, and selection
ranges remain unchanged. Members use a compact numbered code panel.

The inspector supports vertical resizing between details and source as well as
horizontal resizing of the full panel. Drag the divider, use its Up/Down keys, or
double-click to reset; the ratio is saved locally. Type overview cards show methods,
while fields/members remain in the inspector. Folder labels toggle on repeated clicks.

Database source, DDL, constraints, and column types now share local SQL highlighting.
Request Flow's entry picker supports route/handler search, keyboard navigation,
and Enter/Escape. API routes use individual cards with distinct handler, middleware,
status, and action sections.

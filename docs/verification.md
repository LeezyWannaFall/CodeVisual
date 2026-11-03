# Initial verification — 2026-11-03

Verified on Linux amd64 with Go 1.27.3 and Node.js 22.16.0. Go 1.26 is the declared
minimum. The frontend is built into the Go binary and requires no Node.js at
runtime. The smoke script starts the binary from a separate temporary directory.

Automated checks:

- Go unit/integration fixtures, including syntax errors, import cycles, missing
  dependencies, build tags, generics, aliases, receivers, source positions,
  exclusions, relative replacements, cancellation, and missing toolchain.
- Snapshot validation, stable analysis output, bounded projections, directory
  aggregation, and shared JSON contract.
- API status/error contracts, source hash checks, symlink rejection, Host/Origin
  guards, read-only methods, and embedded SPA serving.
- `go test -race ./...` and `go vet ./...`.
- Frontend TypeScript production build and five Vitest checks.
- Two Playwright scenarios against the production binary: graph → package →
  interface → highlighted source, filtering/grouping, external imports, global
  symbol search, and a narrow viewport. No browser page errors in the main flow.
- npm dependency audit after tool updates: zero reported vulnerabilities.

## Prepared real projects

No preparation or downloads occur during analysis. All four inputs were
fingerprinted before/after the smoke test; contents and file sets were unchanged.
The test also requests graphs, symbol source, the SPA, and its embedded assets.

| Project | Revision | Packages | Status | Ready time | Package graph JSON |
| --- | --- | ---: | --- | ---: | ---: |
| CodeVisual | Current implementation working tree | 7 | complete | 3.567 s | 5,108 B |
| GoreFlow | `358926e0375cd449dbefe7339fafecde9dd14a53` | 8 | complete | 0.589 s | 5,804 B |
| chi v5.3.1 | `8b258c7bb28f97a5f2a856ff7ef962578fec9215` | 12 | complete | 0.389 s | 7,238 B |
| x/tools v0.50.0 | `265dd1a6ecf0ee85548c7a8d1787d25fc5675e06` | 214 | partial | 0.372 s | 186,037 B |

chi and x/tools used immutable module-cache copies; revisions come from their Go
module origin metadata. The medium service used its existing checkout without
changing or running it. CodeVisual's own working tree includes test helpers in
its source files but excludes `_test.go` from analysis.

x/tools had unavailable dependencies in the local cache. Its partial snapshot
contained 4,917 declarations and 537 diagnostics, exercising AST fallback and
large-project paging. This is **not** a benchmark of complete type checking of
x/tools. Prepare its dependencies separately to measure that case.

The smoke script sampled process-tree RSS every 50 ms, reporting roughly 13–18
MiB in this environment. These samples are environment-dependent and may miss
short-lived subprocess peaks; they are not a memory guarantee. Ready times include
startup and may reflect warm caches. Every graph response was bounded to at most
200 nodes with no dangling visible edges.

Reproduce with already-prepared project paths:

```sh
make build
python3 scripts/verify_projects.py . /path/to/service /path/to/chi /path/to/tools
```

The script prints JSON metrics per module. Regular `make test` requires no
network after development dependencies are installed. Real-project verification
is opt-in and does not make the fixture suite dependent on external repositories.

## SQL/API sections and UI update — 2026-11-03

- Fixed minimap container/SVG sizing and overflow, verified equal dimensions and
  placement above the legend in Chromium. Click-to-recenter, pan, and zoom are
  enabled; the browser test verifies recentering changes the viewport.
- Added a collapsible inspector that retains selection, restores its preference
  after reload, and reopens on selection. Browser tests confirm the graph gains
  width when the inspector is collapsed.
- Reworked the UI with a dark blue palette, rounded cards/controls, and separate
  Code / API / Database tabs.
- Added independent chi and PostgreSQL DDL inventory analyzers with tests for
  mounted builder variables, helpers, inherited middleware, nested prefixes,
  dynamic/conditional omissions, quoting, foreign keys, indexes, and Down sections.
- `make build test`, `go test -race ./...`, and `go vet ./...` pass. All four
  production-browser scenarios pass, including SQL/API source navigation.

Real-project smoke verification on KitchenService revision
`5785648f0c3f559f08982b2e2cffde1165459720` found **33 HTTP registrations, 4 tables,
and 3 indexes** with status `complete` and no diagnostics. The script verified
bounded Code/API/Database projections and unchanged input files. Initial response
sizes: HTTP 21,836 bytes; Database 12,212 bytes. The application did not execute
KitchenService or connect to its database.

## Readable dependency routing — 2026-11-03

Replaced Dagre placement plus generic smoothstep edges with ELK layered orthogonal
routing. Each relationship has its own port, and the renderer preserves the
computed bends. A local embedded Web Worker performs layout. No remote layout
service is involved.

The geometry fixture matches the 8-package / 12-import dependency shape of
Go-URL-Shortener. Tests verify distinct port positions, no shared nonzero line
segments, no routes through unrelated cards, cycle/disconnected-node handling,
correct direction filtering, and attached endpoints after dragging.

A production-browser test analyzed the actual Go-URL-Shortener checkout at
`ad76b2d99bd9cb023ca7432e42c9d84aa43cee4d`. Selecting `handler` produced three nodes
and two source-backed edges (`main → handler → model`); incoming/outgoing filters
and restoring all 12 imports passed. The test also verifies focused cards remain
inside the viewport when the inspector opens, and saves overview/focus screenshots.

`make build test` passes (9 frontend tests); all 5 browser scenarios pass with:

```sh
cd web
CODEVISUAL_TEST_SHORTENER=1 npm run test:e2e
```

The optional real-project test expects a sibling `Go-URL-Shortener` checkout;
regular browser tests do not require that repository. Selecting a package now
lays out only its immediate neighbors; All connections restores the loaded graph.
Counts remain relative to that loaded graph, and the summary states the visible
subset. Arrange recomputes routes after manual card movement.


## Black theme and explicit focus — 2026-11-03

Changed base surfaces to neutral black/gray and retained blue only for interaction
accents. Incoming/outgoing relationships use two blue shades. All connections is
now the default: clicking a node highlights it and opens the inspector without
automatically activating Selected node or changing card positions. Compact focus
is an explicit action.

Browser regression coverage checks neutral dark backgrounds in the header,
navigation, explorer, graph container, inspector, and footer. The real-shortener
scenario verifies node/edge counts and card positions survive selection unchanged,
then explicitly activates Selected node to check direction filtering.

## Fixed cards and animated selection — 2026-11-03

Disabled node dragging and keyboard deletion, and removed the minimap and manual
Arrange control. Cards opt out of canvas panning so click/double-click handling
remains reliable while the background can still be panned. Incident relationships
of the selected node animate as dashed strokes; unrelated edges and crossing
halos stay solid. Reduced-motion preferences stop the animation.

The five browser scenarios pass. Coverage includes attempted card dragging with
unchanged positions, absent minimap, selected-only animation counts, computed dash
styles and changing dash offset, double-click navigation, and preserving the
complete graph on ordinary selection. The production binary was rebuilt with
these assets.

## Database map and source reading panel — 2026-11-03

Database now defaults to a dedicated Map with table columns, PK/FK markers,
foreign-key relationships, and index-to-table links. List remains available for
DDL details. SQL cards use their own measured layout dimensions and retain the
read-only graph interactions and animated selection.

Inspecting a source-backed node automatically widens the inspector to 44vw
(with desktop limits), while small screens use a bounded overlay. Source text
preserves indentation and wraps by default; Wrap lines can restore horizontal
scrolling without changing file line numbers.

`make build test` passed with 10 frontend checks. All 6 browser scenarios passed,
including the SQL map/list switch, selecting a table to open its source, and the
actual GetByFullUrl method in Go-URL-Shortener. The latter verifies expanded panel
width, unchanged line counts when toggling wrapping, no horizontal clipping with
wrapping enabled, and a usable 600px viewport. Screenshots cover the map and wide
source panel. The single binary was rebuilt with the updated frontend assets.

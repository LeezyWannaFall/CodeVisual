# REST API v1

Base URL: `http://127.0.0.1:7331/api/v1`. Read-only JSON, camelCase, same-origin
requests only. No authentication, remote listener, CORS, or file-path parameters.

| GET endpoint | Response |
| --- | --- |
| `/project` | `{schemaVersion, analysisId, status, project, statistics, diagnosticCount}` |
| `/tree` | Flat array of module, directory, package nodes; hierarchy uses `parentId` |
| `/graph?level=packages` | Bounded graph projection |
| `/graph?level=directories` | Immediate directory/package children and aggregated imports |
| `/graph?level=symbols&packageId=ID` | Declarations for one package |
| `/graph?level=http` | chi registration nodes |
| `/graph?level=database` | SQL table/index nodes and explicit references/index edges |
| `/search?q=TEXT` | `{nodes, total}`; global package/symbol/route/SQL search, maximum 200 results |
| `/nodes/ID` | One complete node |
| `/files/ID` | `{id, relativePath, content}`; validates hash before returning source |
| `/diagnostics` | Diagnostic array |

Graph query parameters:

| Parameter | Meaning |
| --- | --- |
| `level` | `packages` (default), `directories`, `symbols`, `http`, `database` |
| `packageId` | Required for `symbols` |
| `groupId` | Directory/module ID; immediate children in directory view, descendant packages in package view |
| `includeExternal` | `true` or `false` (default); package view |
| `q` | Case-insensitive match on label, import path, relative path, or signature |
| `kind` | Exact node kind filter (including `table`, `index`, `httpRoute`) |
| `method` | Exact HTTP method filter for route nodes |
| `exportedOnly` | `true` or `false` (default); primarily useful for symbols |
| `offset` | Non-negative node offset, default 0; page size fixed at 200 |

Graph response: `{analysisId, nodes, edges, totalNodes, hiddenNodes}`.
`totalNodes` counts filtered nodes before paging; `hiddenNodes` counts nodes
outside this page. Edges include only visible endpoints. Directory aggregation
omits imports internal to the same group and sets `derived: true` and `count`.
Directory view starts at the module root when `groupId` is absent. External
packages do not enter directory aggregation.

## Model

```ts
interface SourceRange {
  fileId: string
  startLine: number
  startColumn: number
  endLine: number
  endColumn: number
}
interface Node {
  id: string
  kind: 'module' | 'directory' | 'package' | 'externalPackage'
      | 'function' | 'method' | 'struct' | 'interface' | 'type'
      | 'httpRoute' | 'table' | 'index'
  label: string
  parentId?: string
  packageId?: string
  source?: SourceRange
  attributes: {
    importPath?: string
    relativePath?: string
    exported: boolean
    unresolved?: boolean
    standard?: boolean
    signature?: string
    documentation?: string
    receiver?: string
    members?: string[]
    http?: { method: string; path: string; handler: string; middleware: string[]; scope: string }
    table?: {
      name: string
      columns: { name: string; type: string; constraints?: string }[]
      constraints: string[]
      references: { columns: string; table: string; targetColumns: string }[]
    }
    index?: { name: string; table: string; unique: boolean; definition: string }
  }
}
interface Edge {
  id: string
  kind: 'contains' | 'imports' | 'declares_method' | 'references' | 'indexes'
  sourceId: string
  targetId: string
  evidence?: SourceRange
  count?: number
  derived?: boolean
}
interface Diagnostic {
  severity: 'info' | 'warning' | 'error'
  code: string
  message: string
  nodeId?: string
  source?: SourceRange
}
```

Positions are one-based Go token positions: byte columns and an exclusive end.
Node source ranges cover declarations; import edges retain a representative
import location. Repeated imports between a package pair produce one graph edge.
Imported module-local packages with no available source use `package` with
`attributes.unresolved: true`; they are not presented as external dependencies.
Aliases and other named types use kind `type`. Anonymous functions and test
symbols are omitted. Method ownership is based on explicit receiver declarations,
including declarations recovered from erroneous source; it is not a call relation.

Project includes `modulePath`, `rootName`, and `buildContext` with `goos`, `goarch`,
`tags`, `cgoEnabled`, and `configurationKnown`. No absolute project-root field is
exposed. Status is `complete` or `partial`; warnings/errors make it partial.
Informational exclusions alone do not. An analysis ID identifies the sorted
snapshot content; node IDs identify semantic entities across snapshots.

Errors:

```json
{"error":{"code":"source_changed","message":"Source changed or is unavailable. Restart codevisual analyze."}}
```

- `400 invalid_query`: unsupported graph level or invalid parameter.
- `403 invalid_host` / `invalid_origin`: rejected nonlocal/cross-origin request.
- `404 node_not_found`, `file_not_found`, or `not_found`.
- `405 method_not_allowed`: unsupported method.
- `409 source_changed`: modified, removed, unreadable, or symlinked source.

Unknown `/api/*` paths always return JSON instead of SPA HTML. Missing static
assets return 404; other frontend routes fall back to the SPA.

The Go model is authoritative. TypeScript mirrors it in `web/src/api/types.ts`;
Go serialization and frontend consumption share `testdata/contracts/graph.json`.

## SQL and HTTP semantics

HTTP nodes are source registrations with a named builder scope. Resolving a
supported mount prefixes the builder's routes; unmounted builders retain their
local paths and scope. Static analysis does not assert runtime reachability.
Dynamic control flow, unknown mount handlers, named callbacks, and dynamic paths
are accompanied by `http_*` diagnostics.

SQL table/index nodes represent declarations with SQL source ranges; files use
the same registered-ID and hash-checked source API as Go. Relationships are added
only for explicit references to one matching table declaration. Schema evolution
is not simulated: unsupported `ALTER`, `DROP`, or `CREATE` forms produce
`sql_unsupported`. Down migrations/sections are excluded. All explorer projections
use the same 200-node paging limit and never expose arbitrary file paths.

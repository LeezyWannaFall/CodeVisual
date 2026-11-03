import { useEffect, useState } from 'react'
import { get } from './api/client'
import type { Diagnostic, GraphNode, GraphView, Level, ProjectInfo } from './api/types'
import { Icon } from './components/Icon'
import { Explorer } from './features/explorer/Explorer'
import { GraphCanvas } from './features/graph/GraphCanvas'
import { ResourceExplorer } from './features/explorers/ResourceExplorer'
import { Details } from './features/source/Details'

const emptyView: GraphView = { analysisId: '', nodes: [], edges: [], totalNodes: 0, hiddenNodes: 0 }
export default function App() {
 const [project, setProject] = useState<ProjectInfo | null>(null)
 const [tree, setTree] = useState<GraphNode[]>([])
 const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([])
 const [view, setView] = useState<GraphView>(emptyView)
 const [level, setLevel] = useState<Level>('packages')
 const [groupId, setGroupId] = useState('')
 const [packageId, setPackageId] = useState('')
 const [inspectorCollapsed, setInspectorCollapsed] = useState(() => { try { return localStorage.getItem('codevisual.inspectorCollapsed') === 'true' } catch { return false } })
 const [selected, setSelected] = useState<GraphNode | null>(null)
 const [external, setExternal] = useState(false)
 const [exported, setExported] = useState(false)
 const [kind, setKind] = useState('')
 const [query, setQuery] = useState('')
 const [debouncedQuery, setDebouncedQuery] = useState('')
 const [offset, setOffset] = useState(0)
 const [loading, setLoading] = useState(true)
 const [error, setError] = useState('')
 const [showDiagnostics, setShowDiagnostics] = useState(false)
 const [routeMethod, setRouteMethod] = useState('')
 const [revision, setRevision] = useState(0)
 useEffect(() => { try { localStorage.setItem('codevisual.inspectorCollapsed', String(inspectorCollapsed)) } catch { /* Storage may be unavailable in private mode. */ } }, [inspectorCollapsed])
 useEffect(() => {
  const controller = new AbortController(); setError('')
  Promise.all([get<ProjectInfo>('/project', controller.signal), get<GraphNode[]>('/tree', controller.signal), get<Diagnostic[]>('/diagnostics', controller.signal)])
   .then(([p, t, d]) => { setProject(p); setTree(t); setDiagnostics(d); if ((p.statistics.package ?? 0) > 200) setLevel('directories') })
   .catch(e => { if (!controller.signal.aborted) { setError(e.message); setLoading(false) } })
  return () => controller.abort()
 }, [revision])
 useEffect(() => { const timer = window.setTimeout(() => { setDebouncedQuery(query); setOffset(0) }, 180); return () => clearTimeout(timer) }, [query])
 useEffect(() => {
  if (!project) return
  const controller = new AbortController(); setLoading(true); setError('')
  const params = new URLSearchParams({ level, includeExternal: String(external), exportedOnly: String(exported && level === 'symbols'), offset: String(offset), q: debouncedQuery })
  if (packageId) params.set('packageId', packageId)
  if (groupId) params.set('groupId', groupId)
  if (kind && (level === 'symbols' || level === 'database')) params.set('kind', kind)
  if (routeMethod && level === 'http') params.set('method', routeMethod)
  get<GraphView>(`/graph?${params}`, controller.signal).then(setView).catch(e => { if (!controller.signal.aborted) setError(e.message) }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
  return () => controller.abort()
 }, [project, level, groupId, packageId, external, exported, kind, debouncedQuery, offset, revision, routeMethod])
 function selectNode(n: GraphNode) { setSelected(n); setInspectorCollapsed(false) }
 function navigate(n: GraphNode) {
  setOffset(0); setQuery(''); setDebouncedQuery(''); setKind(''); setExported(false); setRouteMethod(''); selectNode(n)
  if (n.kind === 'httpRoute') { setLevel('http'); setPackageId(''); setGroupId('') }
  else if (n.kind === 'table' || n.kind === 'index') { setLevel('database'); setPackageId(''); setGroupId('') }
  else if (n.kind === 'package') { setLevel('symbols'); setPackageId(n.id); setGroupId('') }
  else if (n.kind === 'directory' || n.kind === 'module') { setLevel('directories'); setGroupId(n.kind === 'module' ? '' : n.id); setPackageId('') }
  else if (n.packageId) { setLevel('symbols'); setPackageId(n.packageId); setGroupId('') }
 }
 function overview(next: Level = 'packages') { setLevel(next); setGroupId(''); setPackageId(''); setSelected(null); setOffset(0); setQuery(''); setDebouncedQuery(''); setKind(''); setExported(false); setRouteMethod('') }
 const section = level === 'http' ? 'api' : level === 'database' ? 'database' : 'code'
 const scope = tree.find(n => n.id === (level === 'symbols' ? packageId : groupId))
 const symbols = (project?.statistics.function ?? 0) + (project?.statistics.method ?? 0) + (project?.statistics.struct ?? 0) + (project?.statistics.interface ?? 0) + (project?.statistics.type ?? 0)
 return <div className="app-shell">
  <header className="app-header"><a className="brand" href="/" aria-label="CodeVisual home"><span className="brand-mark"><Icon name="graph" size={22}/></span><span>Code<span className="brand-light">Visual</span></span><span className="version">v0.1</span></a><span className="header-divider"/><span className="header-project"><Icon name="folder"/>{project?.project.rootName ?? 'Architecture explorer'}</span><div className="header-right"><span className="local-pill"><span className="status-dot"/>LOCAL SESSION</span><button className={`diagnostics-button ${project?.status === 'partial' ? 'warning' : ''}`} onClick={() => setShowDiagnostics(v => !v)}><Icon name={project?.status === 'partial' ? 'info' : 'check'} size={14}/>{project?.status === 'partial' ? 'Partial analysis' : 'Analysis complete'}<span>{diagnostics.length}</span></button></div></header>
  <nav className="workspace-tabs" aria-label="Project sections">
 <button className={section === 'code' ? 'active' : ''} aria-pressed={section === 'code'} onClick={() => overview()}><Icon name="code"/>Code<span>{project?.statistics.package ?? 0}</span></button>
 <button className={section === 'api' ? 'active' : ''} aria-pressed={section === 'api'} onClick={() => overview('http')}><Icon name="api"/>API<span>{project?.statistics.httpRoute ?? 0}</span></button>
 <button className={section === 'database' ? 'active' : ''} aria-pressed={section === 'database'} onClick={() => overview('database')}><Icon name="database"/>Database<span>{project?.statistics.table ?? 0}</span></button>
 <span className="section-note">{section === 'api' ? 'chi route registrations' : section === 'database' ? 'SQL declarations · no database connection' : 'Go packages & symbols'}</span>
 <button className="inspector-toggle" aria-label={inspectorCollapsed ? 'Expand inspector' : 'Collapse inspector'} aria-expanded={!inspectorCollapsed} onClick={() => setInspectorCollapsed(v => !v)}><Icon name="panel"/><span>Inspector</span></button>
 </nav>
 <div className="workspace"><Explorer tree={tree} selected={selected} onOpen={navigate}/><main className="graph-main">
   <div className="page-heading"><div className="breadcrumb"><button onClick={() => overview()}><Icon name="graph" size={13}/>Architecture</button>{scope && <><Icon name="arrow" size={11}/><button onClick={() => navigate(scope)}>{scope.attributes.relativePath ?? scope.label}</button></>}<span className="view-label">{section === 'api' ? 'HTTP EXPLORER' : section === 'database' ? 'DATABASE EXPLORER' : level === 'symbols' ? 'SYMBOL VIEW' : 'DEPENDENCY MAP'}</span></div><div className="heading-row"><div><h1>{section === 'api' ? 'HTTP routes' : section === 'database' ? 'Database schema' : level === 'symbols' ? scope?.label ?? 'Package symbols' : groupId ? scope?.attributes.relativePath ?? 'Directory' : 'Project architecture'}<span className="heading-dot">.</span></h1><p>{section === 'api' ? 'Methods, paths, middleware, and handler registrations from source.' : section === 'database' ? 'Tables, columns, indexes, and declared foreign keys from SQL files.' : level === 'symbols' ? 'Declarations and type relationships in this package.' : 'Explore how your Go packages connect.'}</p></div>{section === 'code' && <div className="view-switch" aria-label="Graph level"><button className={level === 'packages' ? 'active' : ''} onClick={() => overview('packages')}><Icon name="package" size={14}/>Packages</button><button className={level === 'directories' ? 'active' : ''} onClick={() => overview('directories')}><Icon name="folder" size={14}/>Directories</button></div>}</div></div>
   <div className="graph-toolbar"><div className="filter-search"><Icon name="search" size={14}/><input aria-label="Filter graph" placeholder="Filter this view…" value={query} onChange={e => setQuery(e.target.value)}/></div><div className="toolbar-separator"/>{level === 'http' ? <select aria-label="HTTP method" value={routeMethod} onChange={e => { setRouteMethod(e.target.value); setOffset(0) }}><option value="">All methods</option>{['GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS','CONNECT','TRACE','ANY'].map(m => <option key={m}>{m}</option>)}</select> : level === 'database' ? <select aria-label="Database object kind" value={kind} onChange={e => { setKind(e.target.value); setOffset(0) }}><option value="">Tables & indexes</option><option value="table">Tables</option><option value="index">Indexes</option></select> : level === 'symbols' ? <><select aria-label="Symbol kind" value={kind} onChange={e => { setKind(e.target.value); setOffset(0) }}><option value="">All symbols</option><option value="function">Functions</option><option value="method">Methods</option><option value="struct">Structs</option><option value="interface">Interfaces</option><option value="type">Types</option></select><label className="filter-check"><input type="checkbox" checked={exported} onChange={e => { setExported(e.target.checked); setOffset(0) }}/>Exported only</label></> : level === 'packages' ? <label className="filter-check"><input type="checkbox" checked={external} onChange={e => { setExternal(e.target.checked); setOffset(0) }}/>External dependencies</label> : <span className="toolbar-note">Imports grouped by directory</span>}<span className="view-count">{view.totalNodes} {section === 'code' ? 'nodes' : 'declarations'}<span> / </span>{view.edges.length} {section === 'code' ? 'edges' : 'relations'}</span></div>
   <div className={`canvas-wrap ${section !== 'code' ? 'resource-wrap' : ''}`} aria-label={section === 'code' ? 'Architecture graph' : section === 'api' ? 'HTTP routes' : 'Database declarations'}>{!loading && !error && view.nodes.length > 0 && (section === 'code' ? <GraphCanvas view={view} selected={selected} onSelect={selectNode} onOpen={navigate}/> : <ResourceExplorer view={view} section={section} selected={selected} onSelect={selectNode}/>)}{section === 'code' && <div className="canvas-label"><span className="status-dot"/>{level === 'symbols' ? 'DECLARATION GRAPH' : 'IMPORT DEPENDENCIES'}</div>}
    {loading && <div className="canvas-message" role="status"><span className="spinner"/>Loading architecture…</div>}
    {error && <div className="canvas-message error" role="alert"><Icon name="info" size={24}/><h3>Could not load analysis</h3><p>{error}</p><button className="open-button" onClick={() => setRevision(v => v + 1)}>Try again</button></div>}
    {!loading && !error && view.nodes.length === 0 && <div className="canvas-message"><Icon name="search" size={26}/><h3>{section === 'code' ? 'No nodes in this view' : section === 'api' ? 'No HTTP registrations found' : 'No SQL declarations found'}</h3><p>{section === 'code' ? 'Try another filter or explore a package.' : section === 'api' ? 'Supports static chi registrations. Check filters and analysis diagnostics for unresolved builders.' : 'Supports CREATE TABLE and CREATE INDEX in .sql files. Migrations are never executed.'}</p></div>}
    {view.hiddenNodes > 0 && !loading && <div className="pagination"><span>{offset + 1}–{Math.min(offset + 200, view.totalNodes)} of {view.totalNodes} · {view.hiddenNodes} hidden</span><button disabled={offset === 0} onClick={() => setOffset(v => Math.max(0, v - 200))}>Previous</button><button disabled={offset + 200 >= view.totalNodes} onClick={() => setOffset(v => v + 200)}>Next</button></div>}
    {section === 'code' && <div className="canvas-legend"><span><i className="legend-package"/>Package / declaration</span><span><i className="legend-line"/>{level === 'symbols' ? 'Declared method' : 'Imports'}</span><span className="legend-hint">Drag to move · Scroll to zoom</span></div>}
   </div>
   {showDiagnostics && <section className="diagnostics-panel"><div className="flex items-center justify-between"><h2>Analysis diagnostics <span>{diagnostics.length}</span></h2><button aria-label="Close diagnostics" onClick={() => setShowDiagnostics(false)}><Icon name="close"/></button></div>{diagnostics.length === 0 ? <p>No analysis errors. cgo and test files are excluded by design.</p> : diagnostics.map((d,i) => <div key={i} className={`diagnostic diagnostic-${d.severity}`}><span>{d.severity}</span><code>{d.code}</code><p>{d.message}</p></div>)}</section>}
  </main><Details node={selected} collapsed={inspectorCollapsed} onToggle={() => setInspectorCollapsed(v => !v)} onClose={() => setInspectorCollapsed(true)} onOpen={navigate}/></div>
  <footer className="app-footer"><span className="footer-module"><Icon name="package" size={12}/>{project?.project.modulePath ?? 'Loading module…'}</span><span>{project?.statistics.package ?? 0} packages</span><span>{symbols} symbols</span><div className="ml-auto flex items-center gap-4"><span>{project?.project.buildContext.goos}/{project?.project.buildContext.goarch}{project?.project.buildContext.tags.length ? ` · tags: ${project.project.buildContext.tags.join(', ')}` : ''}{project && !project.project.buildContext.configurationKnown ? ' · build configuration uncertain' : ''}</span><span className="footer-offline"><span className="status-dot"/>Offline · Read only</span></div></footer>
 </div>
}

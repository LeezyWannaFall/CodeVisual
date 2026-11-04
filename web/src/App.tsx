import { flushSync } from 'react-dom'
import { RoutePicker } from './components/RoutePicker'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { get } from './api/client'
import type { Diagnostic, GraphNode, GraphView, Level, ProjectInfo } from './api/types'
import { Icon } from './components/Icon'
import { Explorer } from './features/explorer/Explorer'
import { PackagePanel } from './features/graph/PackagePanel'
import { SymbolOverview } from './features/graph/SymbolOverview'
import { GraphCanvas } from './features/graph/GraphCanvas'
import { ResourceExplorer } from './features/explorers/ResourceExplorer'
import { Details } from './features/source/Details'

const emptyView: GraphView = { analysisId: '', nodes: [], edges: [], totalNodes: 0, hiddenNodes: 0 }
interface NavigationState {
 inspectedPackage: GraphNode | null; level: Level; groupId: string; packageId: string; selected: GraphNode | null; inspectorCollapsed: boolean
 external: boolean; exported: boolean; kind: string; query: string; offset: number; routeMethod: string
 symbolView: 'overview' | 'graph'; databaseView: 'map' | 'list'; flowRoot: GraphNode | null; flowDepth: number; includePossible: boolean
}
export default function App() {
 const themeTransition = useRef(false)
 const [theme, setTheme] = useState<'dark' | 'light'>(() => { try { return localStorage.getItem('codevisual.theme') === 'light' ? 'light' : 'dark' } catch { return 'dark' } })
 useLayoutEffect(() => {
  document.documentElement.dataset.theme = theme
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'light' ? '#f8fafc' : '#050505')
  try { localStorage.setItem('codevisual.theme', theme) } catch { /* Storage can be unavailable. */ }
 }, [theme])
 function toggleTheme(button: HTMLButtonElement) {
  if (themeTransition.current) return
  const changeTheme = () => setTheme(current => current === 'dark' ? 'light' : 'dark')
  if (!document.startViewTransition || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
   changeTheme()
   return
  }
  const { left, top, width, height } = button.getBoundingClientRect()
  const x = left + width / 2, y = top + height / 2
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  const root = document.documentElement
  root.style.setProperty('--theme-reveal-x', `${x}px`)
  root.style.setProperty('--theme-reveal-y', `${y}px`)
  root.style.setProperty('--theme-reveal-radius', `${Math.ceil(radius)}px`)
  root.classList.add('theme-transition')
  themeTransition.current = true
  const transition = document.startViewTransition(() => flushSync(changeTheme))
  void transition.finished.catch(() => { /* Interrupted transitions still apply the theme. */ }).finally(() => {
   root.classList.remove('theme-transition')
   themeTransition.current = false
  })
 }
 const [navigationHistory, setNavigationHistory] = useState<NavigationState[]>([])
 const [project, setProject] = useState<ProjectInfo | null>(null)
 const [tree, setTree] = useState<GraphNode[]>([])
 const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([])
 const [view, setView] = useState<GraphView>(emptyView)
 const [symbolView, setSymbolView] = useState<'overview' | 'graph'>('overview')
 const [databaseView, setDatabaseView] = useState<'map' | 'list'>('map')
 const [flowRoot, setFlowRoot] = useState<GraphNode | null>(null)
 const [routes, setRoutes] = useState<GraphNode[]>([])
 const [flowDepth, setFlowDepth] = useState(6)
 const [includePossible, setIncludePossible] = useState(false)
 const [level, setLevel] = useState<Level>('packages')
 const [groupId, setGroupId] = useState('')
 const [packageId, setPackageId] = useState('')
 const [inspectorCollapsed, setInspectorCollapsed] = useState(() => { try { return localStorage.getItem('codevisual.inspectorCollapsed') === 'true' } catch { return false } })
 const documentRef = useRef<HTMLDivElement>(null)
 const packageScroll = useRef<number | null>(null)
 const [inspectedPackage, setInspectedPackage] = useState<GraphNode | null>(null)
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
  Promise.all([get<ProjectInfo>('/project', controller.signal), get<GraphNode[]>('/tree', controller.signal), get<Diagnostic[]>('/diagnostics', controller.signal), get<GraphView>('/graph?level=http', controller.signal)])
   .then(([p, t, d, r]) => { setRoutes(r.nodes.filter(n => n.kind === 'httpRoute')); setProject(p); setTree(t); setDiagnostics(d); if ((p.statistics.package ?? 0) > 200) setLevel('directories') })
   .catch(e => { if (!controller.signal.aborted) { setError(e.message); setLoading(false) } })
  return () => controller.abort()
 }, [revision])
 useEffect(() => { if (query === debouncedQuery) return; const timer = window.setTimeout(() => { setDebouncedQuery(query); setOffset(0) }, 180); return () => clearTimeout(timer) }, [query, debouncedQuery])
 useEffect(() => {
  if (!project) return
  if (level === 'flow' && !flowRoot) { setView(emptyView); setLoading(false); return }
  const controller = new AbortController(); setLoading(true); setError('')
  const params = new URLSearchParams({ level, includeExternal: String(external), exportedOnly: String(exported && level === 'symbols'), offset: String(offset), q: debouncedQuery })
  if (level === 'flow' && flowRoot) { params.set('rootId', flowRoot.id); params.set('depth', String(flowDepth)); params.set('includePossible', String(includePossible)) }
  if (packageId) params.set('packageId', packageId)
  if (groupId) params.set('groupId', groupId)
  if (kind && (level === 'symbols' || level === 'database')) params.set('kind', kind)
  if (routeMethod && level === 'http') params.set('method', routeMethod)
  get<GraphView>(`/graph?${params}`, controller.signal).then(setView).catch(e => { if (!controller.signal.aborted) setError(e.message) }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
  return () => controller.abort()
 }, [project, level, groupId, packageId, external, exported, kind, debouncedQuery, offset, revision, routeMethod, flowRoot, flowDepth, includePossible])
 function rememberView() {
  const current = { inspectedPackage, level, groupId, packageId, selected, inspectorCollapsed, external, exported, kind, query, offset, routeMethod, symbolView, databaseView, flowRoot, flowDepth, includePossible }
  setNavigationHistory(old => [...old.slice(-49), current])
 }
 function goBack() {
  const previous = navigationHistory.at(-1)
  if (!previous) return
  setNavigationHistory(old => old.slice(0, -1))
  setInspectedPackage(previous.inspectedPackage); setLevel(previous.level); setGroupId(previous.groupId); setPackageId(previous.packageId)
  setSelected(previous.selected); setInspectorCollapsed(previous.inspectorCollapsed)
  setExternal(previous.external); setExported(previous.exported); setKind(previous.kind)
  setQuery(previous.query); setDebouncedQuery(previous.query); setOffset(previous.offset); setRouteMethod(previous.routeMethod)
  setSymbolView(previous.symbolView); setDatabaseView(previous.databaseView)
  setFlowRoot(previous.flowRoot); setFlowDepth(previous.flowDepth); setIncludePossible(previous.includePossible)
 }
 async function openTarget(id: string) { try { const node = await get<GraphNode>(`/nodes/${encodeURIComponent(id)}`); rememberView(); selectNode(node) } catch(e) { setError(e instanceof Error ? e.message : String(e)) } }
 function openFlow(node?: GraphNode) { rememberView(); setFlowRoot(node ?? flowRoot ?? routes[0] ?? null); setLevel('flow'); setSelected(null); setGroupId(''); setPackageId(''); setOffset(0); setQuery(''); setDebouncedQuery(''); setKind(''); setExternal(false) }
 function selectNode(n: GraphNode) { setSelected(n); if (n.kind === 'package' && level === 'packages') { packageScroll.current = documentRef.current?.scrollTop ?? 0; setInspectedPackage(n); return }; setInspectorCollapsed(false) }
 useLayoutEffect(() => {
  if (packageScroll.current !== null && documentRef.current) { documentRef.current.scrollTop = packageScroll.current; packageScroll.current = null }
 }, [inspectedPackage])
 function navigate(n: GraphNode) {
  rememberView()
  setOffset(0); setQuery(''); setDebouncedQuery(''); setKind(''); setExported(false); setRouteMethod(''); selectNode(n)
  if (n.kind === 'httpRoute') { setLevel('http'); setPackageId(''); setGroupId('') }
  else if (n.kind === 'table' || n.kind === 'index') { setLevel('database'); setPackageId(''); setGroupId('') }
  else if (n.kind === 'package') { setSymbolView('overview'); setLevel('symbols'); setPackageId(n.id); setGroupId('') }
  else if (n.kind === 'directory' || n.kind === 'module') { setLevel('directories'); setGroupId(n.kind === 'module' ? '' : n.id); setPackageId('') }
  else if (n.packageId) { setLevel('symbols'); setPackageId(n.packageId); setGroupId('') }
 }
 function overview(next: Level = 'packages') { rememberView(); setLevel(next); setGroupId(''); setPackageId(''); setSelected(null); setOffset(0); setQuery(''); setDebouncedQuery(''); setKind(''); setExported(false); setRouteMethod('') }
 const section = level === 'flow' ? 'flow' : level === 'http' ? 'api' : level === 'database' ? 'database' : 'code'
 const tableMap = level === 'database' && databaseView === 'map' && view.nodes.some(n => n.kind === 'table')
 const scope = tree.find(n => n.id === (level === 'symbols' ? packageId : groupId))

 return <div className="app-shell">
  <header className="app-header"><a className="brand" href="/" aria-label="CodeVisual home"><span className="brand-mark"><Icon name="graph" size={22}/></span><span>Code<span className="brand-light">Visual</span></span><span className="version">v0.1</span></a><span className="header-divider"/><span className="header-project"><Icon name="folder"/>{project?.project.rootName ?? 'Architecture explorer'}</span><div className="header-right"><button className="theme-toggle" aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'} title={theme === 'dark' ? 'Light theme' : 'Dark theme'} onClick={event => toggleTheme(event.currentTarget)}><Icon name={theme === 'dark' ? 'sun' : 'moon'}/></button><span className="local-pill"><span className="status-dot"/>LOCAL SESSION</span><button className={`diagnostics-button ${project?.status === 'partial' ? 'warning' : ''}`} onClick={() => setShowDiagnostics(v => !v)}><Icon name={project?.status === 'partial' ? 'info' : 'check'} size={14}/>{project?.status === 'partial' ? 'Partial analysis' : 'Analysis complete'}<span>{diagnostics.length}</span></button></div></header>

 <div className="workspace"><div className="workspace-sidebar">  <nav className="workspace-tabs" aria-label="Project sections">
 <button className={section === 'code' ? 'active' : ''} aria-pressed={section === 'code'} onClick={() => overview()}><Icon name="code"/>Code<span>{project?.statistics.package ?? 0}</span></button>
 <button className={section === 'api' ? 'active' : ''} aria-pressed={section === 'api'} onClick={() => overview('http')}><Icon name="api"/>API<span>{project?.statistics.httpRoute ?? 0}</span></button>
 <button className={section === 'database' ? 'active' : ''} aria-pressed={section === 'database'} onClick={() => overview('database')}><Icon name="database"/>Database<span>{project?.statistics.table ?? 0}</span></button>
 <button className={section === 'flow' ? 'active' : ''} aria-pressed={section === 'flow'} onClick={() => openFlow()}><Icon name="graph"/>Request Flow</button>


 </nav><Explorer tree={tree} selected={selected} onOpen={navigate}/></div><main className="graph-main">
   <div className="page-heading"><div className="breadcrumb"><button className="back-button" aria-label="Go back" disabled={navigationHistory.length === 0} onClick={goBack}><span className="back-arrow"><Icon name="arrow" size={12}/></span>Back</button><button onClick={() => overview()}><Icon name="graph" size={13}/>Architecture</button>{scope && <><Icon name="arrow" size={11}/><button onClick={() => navigate(scope)}>{scope.attributes.relativePath ?? scope.label}</button></>}<span className="view-label">{section === 'flow' ? 'REQUEST FLOW' : section === 'api' ? 'HTTP EXPLORER' : section === 'database' ? 'DATABASE EXPLORER' : level === 'symbols' ? 'SYMBOL VIEW' : 'DEPENDENCY MAP'}</span></div><div className="heading-row"><div><h1>{section === 'flow' ? 'Request flow' : section === 'api' ? 'HTTP routes' : section === 'database' ? 'Database schema' : level === 'symbols' ? scope?.label ?? 'Package symbols' : groupId ? scope?.attributes.relativePath ?? 'Directory' : 'Project architecture'}<span className="heading-dot">.</span></h1><p>{section === 'flow' ? 'Source call sites reachable from a handler. This is not runtime execution order.' : section === 'api' ? 'Methods, paths, middleware, and handler registrations from source.' : section === 'database' ? 'Tables, columns, indexes, and declared foreign keys from SQL files.' : level === 'symbols' ? symbolView === 'overview' ? 'Types, contracts, and functions. Expand only what you need.' : 'Declarations and type relationships in this package.' : 'Follow imports from left to right. Click a package to inspect it.'}</p></div>{section === 'database' && <div className="view-switch" aria-label="Database view"><button className={databaseView === 'map' ? 'active' : ''} aria-pressed={databaseView === 'map'} onClick={() => setDatabaseView('map')}><Icon name="graph" size={14}/>Map</button><button className={databaseView === 'list' ? 'active' : ''} aria-pressed={databaseView === 'list'} onClick={() => setDatabaseView('list')}><Icon name="database" size={14}/>List</button></div>}{section === 'code' && <div className="view-switch" aria-label="Graph level"><button className={level === 'packages' ? 'active' : ''} onClick={() => overview('packages')}><Icon name="package" size={14}/>Packages</button><button className={level === 'directories' ? 'active' : ''} onClick={() => overview('directories')}><Icon name="folder" size={14}/>Directories</button></div>}</div></div>
   {section === 'flow' && <div className="flow-controls"><label>Entry point<RoutePicker routes={routes} value={flowRoot} onChange={root=>{setFlowRoot(root);setSelected(null)}}/></label><label>Depth<select aria-label="Flow depth" value={flowDepth} onChange={e=>setFlowDepth(Number(e.target.value))}>{[1,2,3,4,5,6,8,10].map(n=><option key={n}>{n}</option>)}</select></label><label className="filter-check"><input type="checkbox" checked={includePossible} onChange={e=>setIncludePossible(e.target.checked)}/>Possible implementations</label><label className="filter-check"><input type="checkbox" checked={external} onChange={e=>setExternal(e.target.checked)}/>External calls</label></div>}
   {section === 'flow' && <div className="flow-notice">Interface and dynamic calls stop at an unresolved boundary. Possible implementations are opt-in structural matches, not confirmed runtime targets.{view.flow && <span>{view.flow.omittedExternal} external targets hidden · {view.flow.omittedPossible} possible dispatches hidden{view.flow.truncated ? ' · Depth or 200-node limit reached' : ''}</span>}</div>}
   {level === 'symbols' && <div className="symbol-view-switch"><button aria-pressed={symbolView === 'overview'} onClick={() => setSymbolView('overview')}>Overview</button><button aria-pressed={symbolView === 'graph'} onClick={() => setSymbolView('graph')}>Declaration graph</button><span>{view.nodes.length} declarations · grouped by type</span></div>}
   <div className="graph-toolbar"><div className="filter-search"><Icon name="search" size={14}/><input disabled={section === 'flow'} aria-label="Filter graph" placeholder="Filter this view…" value={query} onChange={e => setQuery(e.target.value)}/></div><div className="toolbar-separator"/>{level === 'flow' ? <span className="toolbar-note">{flowRoot?.label ?? 'Choose an entry point from API'}</span> : level === 'http' ? <select aria-label="HTTP method" value={routeMethod} onChange={e => { setRouteMethod(e.target.value); setOffset(0) }}><option value="">All methods</option>{['GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS','CONNECT','TRACE','ANY'].map(m => <option key={m}>{m}</option>)}</select> : level === 'database' ? <select aria-label="Database object kind" value={kind} onChange={e => { setKind(e.target.value); setOffset(0) }}><option value="">Tables & indexes</option><option value="table">Tables</option><option value="index">Indexes</option></select> : level === 'symbols' ? <><select aria-label="Symbol kind" value={kind} onChange={e => { setKind(e.target.value); setOffset(0) }}><option value="">All symbols</option><option value="function">Functions</option><option value="method">Methods</option><option value="struct">Structs</option><option value="interface">Interfaces</option><option value="type">Types</option></select><label className="filter-check"><input type="checkbox" checked={exported} onChange={e => { setExported(e.target.checked); setOffset(0) }}/>Exported only</label></> : level === 'packages' ? <label className="filter-check"><input type="checkbox" checked={external} onChange={e => { setExternal(e.target.checked); setOffset(0) }}/>External dependencies</label> : <span className="toolbar-note">Imports grouped by directory</span>}<span className="view-count">{tableMap ? <>{view.nodes.filter(n => n.kind === 'table').length} tables · {view.nodes.filter(n => n.kind === 'index').length} indexes</> : <>{view.totalNodes} {level === 'symbols' && symbolView === 'overview' ? 'declarations' : section === 'code' || section === 'flow' ? 'nodes' : 'declarations'}{!(level === 'symbols' && symbolView === 'overview') && <><span> / </span>{view.edges.length} {section === 'code' || section === 'flow' ? 'edges' : 'relations'}</>}</>}</span></div>
   <div ref={documentRef} className={`map-document ${level === 'packages' && inspectedPackage ? 'with-package-overview' : ''}`}>
   <div className={`canvas-wrap ${section === 'api' || (section === 'database' && databaseView === 'list') ? 'resource-wrap' : ''}`} aria-label={section === 'flow' ? 'Request flow graph' : section === 'code' ? 'Architecture graph' : section === 'api' ? 'HTTP routes' : 'Database declarations'}>{!loading && !error && view.nodes.length > 0 && (section === 'code' || section === 'flow' || (section === 'database' && databaseView === 'map') ? level === 'symbols' && symbolView === 'overview' ? <SymbolOverview view={view} selected={selected} onSelect={selectNode}/> : <GraphCanvas view={view} selected={level === 'packages' ? inspectedPackage : selected} onSelect={selectNode} onOpen={section === 'database' || section === 'flow' ? selectNode : navigate}/> : <ResourceExplorer view={view} section={section as 'api' | 'database'} selected={selected} onSelect={selectNode} onOpenTarget={openTarget} onFlow={openFlow}/>)}{section === 'code' && <div className="canvas-label"><span className="status-dot"/>{level === 'symbols' ? 'DECLARATION GRAPH' : 'IMPORT DEPENDENCIES'}</div>}
    {loading && <div className="canvas-message" role="status"><span className="spinner"/>Loading architecture…</div>}
    {error && <div className="canvas-message error" role="alert"><Icon name="info" size={24}/><h3>Could not load analysis</h3><p>{error}</p><button className="open-button" onClick={() => setRevision(v => v + 1)}>Try again</button></div>}
    {!loading && !error && view.nodes.length === 0 && <div className="canvas-message"><Icon name="search" size={26}/><h3>{section === 'flow' ? 'Choose a route to explore its calls' : section === 'code' ? 'No nodes in this view' : section === 'api' ? 'No HTTP registrations found' : 'No SQL declarations found'}</h3><p>{section === 'flow' ? 'Open Request flow from a route in API or select an entry point above.' : section === 'code' ? 'Try another filter or explore a package.' : section === 'api' ? 'Supports static chi registrations. Check filters and analysis diagnostics for unresolved builders.' : 'Supports CREATE TABLE and CREATE INDEX in .sql files. Migrations are never executed.'}</p></div>}
    {view.hiddenNodes > 0 && !loading && <div className="pagination"><span>{offset + 1}–{Math.min(offset + 200, view.totalNodes)} of {view.totalNodes} · {view.hiddenNodes} hidden</span><button disabled={offset === 0} onClick={() => setOffset(v => Math.max(0, v - 200))}>Previous</button><button disabled={offset + 200 >= view.totalNodes} onClick={() => setOffset(v => v + 200)}>Next</button></div>}
    {((section === 'code' && (level !== 'symbols' || symbolView === 'graph')) || section === 'flow' || (section === 'database' && databaseView === 'map')) && <div className="canvas-legend"><span><i className="legend-line"/>{section === 'flow' ? 'Calls → target · Dashed = interface / possible dispatch' : section === 'database' ? 'Foreign-key column — referenced key · Indexes inside tables' : level === 'symbols' ? 'Declared method' : 'A → B means A imports B'}</span></div>}
   </div>
   {level === 'packages' && inspectedPackage && <PackagePanel node={inspectedPackage} selected={selected} onSelect={selectNode}/>}
   </div>
   {showDiagnostics && <section className="diagnostics-panel"><div className="flex items-center justify-between"><h2>Analysis diagnostics <span>{diagnostics.length}</span></h2><button aria-label="Close diagnostics" onClick={() => setShowDiagnostics(false)}><Icon name="close"/></button></div>{diagnostics.length === 0 ? <p>No analysis errors. cgo and test files are excluded by design.</p> : diagnostics.map((d,i) => <div key={i} className={`diagnostic diagnostic-${d.severity}`}><span>{d.severity}</span><code>{d.code}</code><p>{d.message}</p></div>)}</section>}
  </main><Details node={level === 'packages' && selected?.kind === 'package' ? null : selected} collapsed={inspectorCollapsed} onToggle={() => setInspectorCollapsed(v => !v)} onClose={() => setInspectorCollapsed(true)} onOpen={navigate} onOpenTarget={openTarget} onFlow={openFlow}/></div>

 </div>
}

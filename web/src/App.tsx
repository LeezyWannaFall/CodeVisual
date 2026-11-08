import { flushSync } from 'react-dom'
import { RoutePicker } from './components/RoutePicker'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { get, getRoutes } from './api/client'
import type { Diagnostic, GraphNode, GraphView, ProjectInfo } from './api/types'
import { Icon } from './components/Icon'
import { Explorer } from './features/explorer/Explorer'
import { PackagePanel } from './features/graph/PackagePanel'
import { SymbolOverview } from './features/graph/SymbolOverview'
import { GraphCanvas } from './features/graph/GraphCanvas'
import { ResourceExplorer } from './features/explorers/ResourceExplorer'
import { Details } from './features/source/Details'
import { useNavigation } from './features/navigation/useNavigation'
import { ViewFeedback } from './features/navigation/ViewFeedback'

const emptyView: GraphView = { analysisId: '', nodes: [], edges: [], totalNodes: 0, hiddenNodes: 0 }
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
 const [project, setProject] = useState<ProjectInfo | null>(null)
 const [tree, setTree] = useState<GraphNode[]>([])
 const [diagnostics, setDiagnostics] = useState<Diagnostic[]>([])
 const [view, setView] = useState<GraphView>(emptyView)
 const [routes, setRoutes] = useState<GraphNode[]>([])
 const navigation = useNavigation(project, tree, routes)
 const { state, requestQuery: debouncedQuery, selected: linkedSelection, inspectedPackage, flowRoot, navigate, overview, openFlow, openTarget, chooseFlowRoot, goBack } = navigation
 const { level, groupId, packageId, inspectorCollapsed, external, exported, kind, query, searchQuery, offset, panelOffset, routeMethod, symbolView, databaseView, flowDepth, includePossible } = state
 const selected = level === 'flow' ? view.nodes.find(n => n.id === linkedSelection?.id) ?? linkedSelection : linkedSelection
 const documentRef = useRef<HTMLDivElement>(null)
 const packageScroll = useRef<number | null>(null)
 const [loading, setLoading] = useState(true)
 const [error, setError] = useState('')
 const [showDiagnostics, setShowDiagnostics] = useState(false)
 const [revision, setRevision] = useState(0)
 useEffect(() => {
  const controller = new AbortController(); setError('')
  Promise.all([get<ProjectInfo>('/project', controller.signal), get<GraphNode[]>('/tree', controller.signal), get<Diagnostic[]>('/diagnostics', controller.signal), getRoutes(controller.signal)])
   .then(([p, t, d, r]) => { if (!controller.signal.aborted) { setRoutes(r); setProject(p); setTree(t); setDiagnostics(d) } })
   .catch(e => { if (!controller.signal.aborted) { setError(e.message); setLoading(false) } })
  return () => controller.abort()
 }, [revision])
 useEffect(() => {
  if (!project || navigation.restoring) return
  if (level === 'flow' && !flowRoot) { setView(emptyView); setLoading(false); return }
  const controller = new AbortController(); setLoading(true); setError('')
  const params = new URLSearchParams({ level, includeExternal: String(external), exportedOnly: String(exported && level === 'symbols'), offset: String(offset), q: debouncedQuery })
  if (level === 'flow' && flowRoot) { params.set('rootId', flowRoot.id); params.set('depth', String(flowDepth)); params.set('includePossible', String(includePossible)) }
  if (packageId) params.set('packageId', packageId)
  if (groupId) params.set('groupId', groupId)
  if (kind && (level === 'symbols' || level === 'database' || level === 'containers')) params.set('kind', kind)
  if (routeMethod && level === 'http') params.set('method', routeMethod)
  get<GraphView>(`/graph?${params}`, controller.signal).then(result => { if (!controller.signal.aborted) setView(result) }).catch(e => { if (!controller.signal.aborted) setError(e.message) }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
  return () => controller.abort()
 }, [project, navigation.restoring, level, groupId, packageId, external, exported, kind, debouncedQuery, offset, revision, routeMethod, flowRoot, flowDepth, includePossible])
 function selectNode(node: GraphNode) {
  if (node.kind === 'package' && level === 'packages') packageScroll.current = documentRef.current?.scrollTop ?? 0
  navigation.selectNode(node)
 }
 useLayoutEffect(() => {
  if (packageScroll.current !== null && documentRef.current) { documentRef.current.scrollTop = packageScroll.current; packageScroll.current = null }
 }, [inspectedPackage])
 const section = level === 'containers' ? 'containers' : level === 'flow' ? 'flow' : level === 'http' ? 'api' : level === 'database' ? 'database' : 'code'
 const busy = !error && (loading || navigation.restoring)
 const tableMap = level === 'database' && databaseView === 'map' && view.nodes.some(n => n.kind === 'table')
 const scope = tree.find(n => n.id === (level === 'symbols' ? packageId : groupId))

 return <div className="app-shell">
  <header className="app-header"><a className="brand" href="/" aria-label="CodeVisual home"><span className="brand-mark"><Icon name="graph" size={22}/></span><span>Code<span className="brand-light">Visual</span></span><span className="version">v0.1</span></a><span className="header-divider"/><span className="header-project"><Icon name="folder"/>{project?.project.rootName ?? 'Architecture explorer'}</span><div className="header-right"><button className="theme-toggle" aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'} title={theme === 'dark' ? 'Light theme' : 'Dark theme'} onClick={event => toggleTheme(event.currentTarget)}><Icon name={theme === 'dark' ? 'sun' : 'moon'}/></button><span className="local-pill"><span className="status-dot"/>LOCAL SESSION</span><button className={`diagnostics-button ${project?.status === 'partial' ? 'warning' : ''}`} onClick={() => setShowDiagnostics(v => !v)}><Icon name={project?.status === 'partial' ? 'info' : 'check'} size={14}/>{project?.status === 'partial' ? 'Partial analysis' : 'Analysis complete'}<span>{diagnostics.length}</span></button></div></header>

 <div className="workspace"><div className="workspace-sidebar">  <nav className="workspace-tabs" aria-label="Project sections">
 <button className={section === 'code' ? 'active' : ''} aria-pressed={section === 'code'} onClick={() => overview()}><Icon name="code"/>Code<span>{project?.statistics.package ?? 0}</span></button>
 <button className={section === 'api' ? 'active' : ''} aria-pressed={section === 'api'} onClick={() => overview('http')}><Icon name="api"/>API<span>{project?.statistics.httpRoute ?? 0}</span></button>
 <button className={section === 'database' ? 'active' : ''} aria-pressed={section === 'database'} onClick={() => overview('database')}><Icon name="database"/>Database<span>{project?.statistics.table ?? 0}</span></button>
 <button className={section === 'containers' ? 'active' : ''} aria-pressed={section === 'containers'} onClick={() => overview('containers')}><Icon name="container"/>Containers<span>{project?.statistics.container ?? 0}</span></button>
 <button className={section === 'flow' ? 'active' : ''} aria-pressed={section === 'flow'} onClick={() => openFlow()}><Icon name="graph"/>Request Flow</button>


 </nav><Explorer languages={project?.project.languages} tree={tree} selected={selected} onOpen={navigate} query={searchQuery} onQueryChange={searchQuery => navigation.patch({ searchQuery })}/></div><main className="graph-main">
   <div className="page-heading"><div className="breadcrumb"><button className="back-button" aria-label="Go back" disabled={!navigation.canGoBack} onClick={goBack}><span className="back-arrow"><Icon name="arrow" size={12}/></span>Back</button><button aria-label="Copy view link" title="Copy a link to this view in the current local session" onClick={() => void navigation.copyLink()}>Copy link</button><button onClick={() => overview()}><Icon name="graph" size={13}/>Architecture</button>{selected?.packageId && level === 'flow' && <button onClick={() => { const owner = tree.find(n => n.id === selected.packageId); if (owner) navigate(owner) }}>Open package</button>}{scope && <><Icon name="arrow" size={11}/><button onClick={() => navigate(scope)}>{scope.attributes.relativePath ?? scope.label}</button></>}<span className="view-label">{section === 'containers' ? 'CONTAINERS' : section === 'flow' ? 'REQUEST FLOW' : section === 'api' ? 'HTTP EXPLORER' : section === 'database' ? 'DATABASE EXPLORER' : level === 'symbols' ? 'SYMBOL VIEW' : 'DEPENDENCY MAP'}</span></div><div className="heading-row"><div><h1>{section === 'containers' ? 'Containers' : section === 'flow' ? 'Request flow' : section === 'api' ? 'HTTP routes' : section === 'database' ? 'Database schema' : level === 'symbols' ? scope?.label ?? 'Package symbols' : groupId ? scope?.attributes.relativePath ?? 'Directory' : 'Project architecture'}<span className="heading-dot">.</span></h1><p>{section === 'containers' ? 'Compose services, Docker build stages, dependencies, and declared contents.' : section === 'flow' ? 'Source call sites reachable from a handler. This is not runtime execution order.' : section === 'api' ? 'Methods, paths, middleware, and handler registrations from source.' : section === 'database' ? 'Tables, columns, indexes, and declared foreign keys from SQL files.' : level === 'symbols' ? symbolView === 'overview' ? 'Types, contracts, and functions. Expand only what you need.' : 'Declarations and type relationships in this package.' : 'Follow imports from left to right. Click a package to inspect it.'}</p></div>{section === 'database' && <div className="view-switch" aria-label="Database view"><button className={databaseView === 'map' ? 'active' : ''} aria-pressed={databaseView === 'map'} onClick={() => navigation.patch({ databaseView: 'map' })}><Icon name="graph" size={14}/>Map</button><button className={databaseView === 'list' ? 'active' : ''} aria-pressed={databaseView === 'list'} onClick={() => navigation.patch({ databaseView: 'list' })}><Icon name="database" size={14}/>List</button></div>}{section === 'code' && <div className="view-switch" aria-label="Graph level"><button className={level === 'packages' ? 'active' : ''} onClick={() => overview('packages')}><Icon name="package" size={14}/>Packages</button><button className={level === 'directories' ? 'active' : ''} onClick={() => overview('directories')}><Icon name="folder" size={14}/>Directories</button></div>}</div></div>
   {section === 'containers' && <div className="flow-notice">Static definitions from each Compose file and Dockerfile. Images are not built or inspected; contents show COPY/ADD declarations and mounts. Compose files are shown independently.</div>}
   {navigation.notice && <div className="navigation-notice" role="status"><span>{navigation.notice}</span><button onClick={navigation.dismissNotice}>Dismiss</button></div>}
   {!busy && level !== 'packages' && level !== 'directories' && selected && !view.nodes.some(n => n.id === selected.id) && selected.kind !== 'package' && <div className="navigation-notice" role="status">The selected declaration is outside this page or its filters. Its source is still available in the inspector.</div>}
   {section === 'flow' && <div className="flow-controls"><label>Entry point<RoutePicker routes={routes} value={flowRoot} onChange={chooseFlowRoot}/></label><label>Depth<select aria-label="Flow depth" value={flowDepth} onChange={e=>navigation.patch({ flowDepth: Number(e.target.value) })}>{[1,2,3,4,5,6,7,8,9,10].map(n=><option key={n}>{n}</option>)}</select></label><label className="filter-check"><input type="checkbox" checked={includePossible} onChange={e=>navigation.patch({ includePossible: e.target.checked })}/>Possible implementations</label><label className="filter-check"><input type="checkbox" checked={external} onChange={e=>navigation.patch({ external: e.target.checked })}/>External calls</label></div>}
   {section === 'flow' && <div className="flow-notice">Interface and dynamic calls stop at an unresolved boundary. Possible implementations are opt-in structural matches, not confirmed runtime targets.{view.flow && <span>{view.flow.omittedExternal} external targets hidden · {view.flow.omittedPossible} possible dispatches hidden{view.flow.truncated ? ' · Depth or 200-node limit reached' : ''}</span>}</div>}
   {level === 'symbols' && <div className="symbol-view-switch"><button aria-pressed={symbolView === 'overview'} onClick={() => navigation.patch({ symbolView: 'overview' })}>Overview</button><button aria-pressed={symbolView === 'graph'} onClick={() => navigation.patch({ symbolView: 'graph' })}>Declaration graph</button><span>{view.nodes.length} declarations · grouped by type</span></div>}
   <div className="graph-toolbar"><div className="filter-search"><Icon name="search" size={14}/><input disabled={section === 'flow'} aria-label="Filter graph" placeholder="Filter this view…" value={query} onChange={e => navigation.filters({ query: e.target.value })}/></div><div className="toolbar-separator"/>{level === 'flow' ? <span className="toolbar-note">{flowRoot?.label ?? 'Choose an entry point from API'}</span> : level === 'http' ? <select aria-label="HTTP method" value={routeMethod} onChange={e => navigation.filters({ routeMethod: e.target.value })}><option value="">All methods</option>{['GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS','CONNECT','TRACE','ANY'].map(m => <option key={m}>{m}</option>)}</select> : level === 'database' ? <select aria-label="Database object kind" value={kind} onChange={e => navigation.filters({ kind: e.target.value })}><option value="">Tables & indexes</option><option value="table">Tables</option><option value="index">Indexes</option></select> : level === 'symbols' ? <><select aria-label="Symbol kind" value={kind} onChange={e => navigation.filters({ kind: e.target.value })}><option value="">All symbols</option><option value="function">Functions</option><option value="method">Methods</option><option value="struct">Structs</option><option value="class">Classes</option><option value="interface">Interfaces</option><option value="type">Types</option></select><label className="filter-check"><input type="checkbox" checked={exported} onChange={e => navigation.filters({ exported: e.target.checked })}/>Exported only</label></> : level === 'containers' ? <div className="view-switch" aria-label="Container source">{[['', 'All'], ['compose', 'Docker Compose'], ['dockerfile', 'Dockerfile']].map(([value, label]) => <button key={value} className={kind === value ? 'active' : ''} aria-pressed={kind === value} onClick={() => navigation.filters({ kind: value })}>{label}</button>)}</div> : level === 'packages' ? <label className="filter-check"><input type="checkbox" checked={external} onChange={e => navigation.filters({ external: e.target.checked })}/>External dependencies</label> : <span className="toolbar-note">Imports grouped by directory</span>}<span className="view-count">{tableMap ? <>{view.nodes.filter(n => n.kind === 'table').length} tables · {view.nodes.filter(n => n.kind === 'index').length} indexes</> : <>{view.totalNodes} {level === 'symbols' && symbolView === 'overview' ? 'declarations' : section === 'code' || section === 'flow' || section === 'containers' ? 'nodes' : 'declarations'}{!(level === 'symbols' && symbolView === 'overview') && <><span> / </span>{view.edges.length} {section === 'code' || section === 'flow' || section === 'containers' ? 'edges' : 'relations'}</>}</>}</span></div>
   <div ref={documentRef} className={`map-document ${level === 'packages' && inspectedPackage ? 'with-package-overview' : ''}`}>
   <div className={`canvas-wrap ${section === 'api' || (section === 'database' && databaseView === 'list') ? 'resource-wrap' : ''}`} aria-label={section === 'containers' ? 'Container architecture graph' : section === 'flow' ? 'Request flow graph' : section === 'code' ? 'Architecture graph' : section === 'api' ? 'HTTP routes' : 'Database declarations'}>{!busy && !error && view.nodes.length > 0 && (section === 'code' || section === 'flow' || section === 'containers' || (section === 'database' && databaseView === 'map') ? level === 'symbols' && symbolView === 'overview' ? <SymbolOverview view={view} selected={selected} onSelect={selectNode}/> : <GraphCanvas options={state} onOptionsChange={navigation.patch} view={view} selected={level === 'packages' ? inspectedPackage : selected} onSelect={selectNode} onOpen={section === 'database' || section === 'flow' || section === 'containers' ? selectNode : navigate}/> : <ResourceExplorer view={view} section={section as 'api' | 'database'} selected={selected} onSelect={selectNode} onOpenTarget={openTarget} onFlow={openFlow}/>)}{section === 'code' && <div className="canvas-label"><span className="status-dot"/>{level === 'symbols' ? 'DECLARATION GRAPH' : 'IMPORT DEPENDENCIES'}</div>}
    {busy && <div className="canvas-message" role="status"><span className="spinner"/>Loading architecture…</div>}
    {error && <div className="canvas-message error" role="alert"><Icon name="info" size={24}/><h3>Could not load analysis</h3><p>{error}</p><button className="open-button" onClick={() => setRevision(v => v + 1)}>Try again</button></div>}
    {!busy && !error && view.nodes.length === 0 && <ViewFeedback state={state} view={view} partial={project?.status === 'partial'} clearFilters={navigation.clearFilters} firstPage={() => navigation.page(0)} showDiagnostics={() => setShowDiagnostics(true)}/>}
    {view.hiddenNodes > 0 && view.nodes.length > 0 && !busy && <div className="pagination"><span>{offset + 1}–{Math.min(offset + 200, view.totalNodes)} of {view.totalNodes} · {view.hiddenNodes} on other pages.</span><button disabled={offset === 0} onClick={() => navigation.page(Math.max(0, offset - 200))}>Previous</button><button disabled={offset + 200 >= view.totalNodes} onClick={() => navigation.page(offset + 200)}>Next</button></div>}
    {((section === 'code' && (level !== 'symbols' || symbolView === 'graph')) || section === 'flow' || section === 'containers' || (section === 'database' && databaseView === 'map')) && <div className="canvas-legend"><span><i className="legend-line"/>{section === 'containers' ? 'Declared dependencies, builds, networks and mounts · Click a service to inspect its contents' : section === 'flow' ? 'Calls → target · Dashed = interface / possible dispatch' : section === 'database' ? 'Foreign-key column — referenced key · Indexes inside tables' : level === 'symbols' ? 'Declared method' : 'A → B means A imports B'}</span></div>}
   </div>
   {level === 'packages' && inspectedPackage && <PackagePanel node={inspectedPackage} selected={selected} onSelect={selectNode} offset={panelOffset} onPage={navigation.panelPage}/>}
   </div>
   {showDiagnostics && <section className="diagnostics-panel"><div className="flex items-center justify-between"><h2>Analysis diagnostics <span>{diagnostics.length}</span></h2><button aria-label="Close diagnostics" onClick={() => setShowDiagnostics(false)}><Icon name="close"/></button></div>{diagnostics.length === 0 ? <p>No analysis errors. cgo and test files are excluded by design.</p> : diagnostics.map((d,i) => <div key={i} className={`diagnostic diagnostic-${d.severity}`}><span>{d.severity}</span><code>{d.code}</code><p>{d.message}</p></div>)}</section>}
  </main><Details nodes={tree} node={level === 'packages' && selected?.kind === 'package' ? null : selected} collapsed={inspectorCollapsed} onToggle={() => navigation.patch({ inspectorCollapsed: !inspectorCollapsed })} onClose={() => navigation.patch({ inspectorCollapsed: true })} onOpen={navigate} onOpenTarget={openTarget} onFlow={openFlow}/></div>

 </div>
}

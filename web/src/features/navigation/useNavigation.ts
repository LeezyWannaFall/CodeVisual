import { useCallback, useEffect, useLayoutEffect, useMemo, useReducer, useRef, useState } from 'react'
import { get, APIError } from '../../api/client'
import type { GraphNode, Level, ProjectInfo } from '../../api/types'
import { initialNavigation, navigationReducer, navigationURL, readNavigation, type NavigationAction, type NavigationState } from './model'

function collapsedPreference() {
 try { return localStorage.getItem('codevisual.inspectorCollapsed') === 'true' } catch { return false }
}
function historyIndex(): number {
 const index = window.history.state?.codevisualNavigation?.index
 return Number.isSafeInteger(index) && index >= 0 ? index : 0
}
function writeHistory(state: NavigationState, index: number, mode: 'push' | 'replace') {
 const data = { ...window.history.state, codevisualNavigation: { index } }
 window.history[mode === 'push' ? 'pushState' : 'replaceState'](data, '', navigationURL(state))
}

export function useNavigation(project: ProjectInfo | null, tree: GraphNode[], routes: GraphNode[]) {
 const [state, dispatch] = useReducer(navigationReducer, undefined, () => readNavigation(window.location.search, collapsedPreference()))
 const current = useRef(state)
 const [index, setIndex] = useState(historyIndex)
 const indexRef = useRef(index)
 const [cache, setCache] = useState<Record<string, GraphNode>>({})
 const [restoring, setRestoring] = useState(true)
 const [notice, setNotice] = useState('')
 const [requestQuery, setRequestQuery] = useState(state.query)
 const initialized = useRef('')
 const explicitView = useRef(new URLSearchParams(window.location.search).has('level'))
 const pendingTarget = useRef<AbortController | null>(null)
 const knownNodes = useMemo(() => new Map([...tree, ...routes, ...Object.values(cache)].map(n => [n.id, n])), [tree, routes, cache])
 const rememberNode = useCallback((node: GraphNode) => setCache(old => old[node.id] === node ? old : { ...old, [node.id]: node }), [])
 const commit = useCallback((action: NavigationAction, mode: 'push' | 'replace' = 'push') => {
  pendingTarget.current?.abort()
  const next = navigationReducer(current.current, action)
  if (JSON.stringify(next) === JSON.stringify(current.current)) return
  const nextIndex = indexRef.current + (mode === 'push' ? 1 : 0)
  writeHistory(next, nextIndex, mode)
  current.current = next
  indexRef.current = nextIndex
  setIndex(nextIndex)
  dispatch({ type: 'restore', state: next })
  if (action.type !== 'filters') setRequestQuery(next.query)
 }, [])
 useLayoutEffect(() => {
  // Seed our first entry so application Back never leaves the local session.
  // History may have moved while the bundle was loading after a reload.
  const fresh = readNavigation(window.location.search, collapsedPreference())
  current.current = fresh
  indexRef.current = historyIndex()
  setIndex(indexRef.current)
  dispatch({ type: 'restore', state: fresh })
  setRequestQuery(fresh.query)
  writeHistory(fresh, indexRef.current, 'replace')
  const pop = () => {
   pendingTarget.current?.abort()
   let next = readNavigation(window.location.search, collapsedPreference())
   const stale = initialized.current && next.analysisId && next.analysisId !== initialized.current
   if (stale) {
    next = { ...initialNavigation(next.inspectorCollapsed), analysisId: initialized.current }
    setNotice('This link belongs to a different analysis. Showing the project overview.')
   }
   current.current = next
   indexRef.current = historyIndex()
   if (stale) writeHistory(next, indexRef.current, 'replace')
   setIndex(indexRef.current)
   dispatch({ type: 'restore', state: next })
   setRequestQuery(next.query)
  }
  const show = (event: PageTransitionEvent) => { if (event.persisted) pop() }
  window.addEventListener('popstate', pop)
  window.addEventListener('pageshow', show)
  return () => { window.removeEventListener('popstate', pop); window.removeEventListener('pageshow', show); pendingTarget.current?.abort() }
 }, [])
 useEffect(() => {
  if (!project || initialized.current === project.analysisId) return
  initialized.current = project.analysisId
  const previous = current.current
  let next: NavigationState
  if (previous.analysisId && previous.analysisId !== project.analysisId) {
   next = { ...initialNavigation(previous.inspectorCollapsed), analysisId: project.analysisId }
   setCache({})
   setNotice('This link belongs to a different analysis. Showing the project overview.')
  } else next = { ...previous, analysisId: project.analysisId }
  if ((project.statistics.package ?? 0) > 200 && !explicitView.current && next.level === 'packages') next.level = 'directories'
  commit({ type: 'restore', state: next }, 'replace')
 }, [project, commit])
 useEffect(() => {
  if (state.query === requestQuery) return
  const timer = window.setTimeout(() => setRequestQuery(state.query), 180)
  return () => clearTimeout(timer)
 }, [state.query, requestQuery])
 useEffect(() => {
  try { localStorage.setItem('codevisual.inspectorCollapsed', String(state.inspectorCollapsed)) } catch { /* Optional preference. */ }
 }, [state.inspectorCollapsed])
 useEffect(() => {
  if (!project) return
  if (state.analysisId && state.analysisId !== project.analysisId) return
  const expected = new Map<string, string | null>([
   [state.selectedId, null], [state.packageId, 'package'], [state.groupId, 'group'],
   [state.inspectedPackageId, 'package'], [state.flowRootId, 'callable'],
  ].filter(([id]) => id) as [string, string | null][])
  const valid = (node: GraphNode, kind: string | null) => !kind || (kind === 'group' ? ['directory','module'].includes(node.kind) : kind === 'callable' ? ['httpRoute','function','method','closure','interfaceMethod'].includes(node.kind) : node.kind === kind)
  if ([...expected].every(([id, kind]) => { const node = knownNodes.get(id); return node && valid(node, kind) })) { setRestoring(false); return }
  const controller = new AbortController()
  setRestoring(true)
  Promise.all([...expected].map(async ([id, kind]) => {
   const node = knownNodes.get(id) ?? await get<GraphNode>(`/nodes/${encodeURIComponent(id)}`, controller.signal)
   if (node.id !== id || !valid(node, kind)) throw new APIError(404, 'node_not_found', 'Invalid navigation target')
   return node
  })).then(nodes => {
   if (controller.signal.aborted) return
   setCache(old => {
    const additions = nodes.filter(n => !knownNodes.has(n.id))
    return additions.length ? { ...old, ...Object.fromEntries(additions.map(n => [n.id, n])) } : old
   })
   setRestoring(false)
  }).catch(error => {
   if (controller.signal.aborted) return
   setRestoring(false)
   if (error instanceof APIError && error.status === 404) {
    setNotice('The linked object is unavailable in this analysis. Showing the project overview.')
    commit({ type: 'restore', state: { ...initialNavigation(current.current.inspectorCollapsed), analysisId: project.analysisId } }, 'replace')
   } else setNotice(`Could not restore the linked view: ${error instanceof Error ? error.message : String(error)}. Reload the page to retry.`)
  })
  return () => controller.abort()
 }, [project, state.selectedId, state.packageId, state.groupId, state.inspectedPackageId, state.flowRootId, knownNodes, commit])

 const navigate = (node: GraphNode) => { rememberNode(node); commit({ type: 'navigate', node }) }
 const selectNode = (node: GraphNode) => { rememberNode(node); commit({ type: 'select', node }, node.kind === 'package' && current.current.level === 'packages' ? 'replace' : 'push') }
 const overview = (level: Level = 'packages') => commit({ type: 'overview', level })
 const openFlow = (node?: GraphNode) => {
  if (node) rememberNode(node)
  commit({ type: 'flow', rootId: node?.id || current.current.flowRootId || routes[0]?.id || '' })
 }
 const chooseFlowRoot = (node: GraphNode) => { rememberNode(node); commit({ type: 'patch', patch: { flowRootId: node.id, selectedId: '', inspectorCollapsed: true } }) }
 async function openTarget(id: string) {
  pendingTarget.current?.abort()
  const controller = new AbortController()
  pendingTarget.current = controller
  try {
   const node = knownNodes.get(id) ?? await get<GraphNode>(`/nodes/${encodeURIComponent(id)}`, controller.signal)
   if (!controller.signal.aborted) navigate(node)
  } catch (error) {
   if (!controller.signal.aborted) setNotice(`Could not open source: ${error instanceof Error ? error.message : String(error)}`)
  }
 }
 async function copyLink() {
  try {
   await navigator.clipboard.writeText(window.location.href)
   setNotice('View link copied. Links open this analysis at the same local address.')
  } catch { setNotice("Copy the view URL from your browser's address bar.") }
 }
 return {
  copyLink,
  state, requestQuery, restoring, notice, dismissNotice: () => setNotice(''), canGoBack: index > 0,
  goBack: () => { if (indexRef.current > 0) window.history.back() },
  selected: knownNodes.get(state.selectedId) ?? null,
  inspectedPackage: knownNodes.get(state.inspectedPackageId) ?? null,
  flowRoot: knownNodes.get(state.flowRootId) ?? null,
  navigate, selectNode, overview, openFlow, openTarget, chooseFlowRoot,
  patch: (patch: Partial<NavigationState>) => commit({ type: 'patch', patch }, 'replace'),
  filters: (patch: Extract<NavigationAction, { type: 'filters' }>['patch']) => commit({ type: 'filters', patch }, 'replace'),
  clearFilters: () => commit({ type: 'clearFilters' }, 'replace'),
  page: (offset: number) => commit({ type: 'patch', patch: { offset } }),
  panelPage: (panelOffset: number) => commit({ type: 'patch', patch: { panelOffset } }),
 }
}

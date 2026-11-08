import type { GraphNode, Level } from '../../api/types'
import type { GraphOptions } from '../graph/layout'

// Keep IDs in navigation; source objects always belong to the current snapshot.
export interface NavigationState extends GraphOptions {
 analysisId: string
 level: Level
 groupId: string
 packageId: string
 selectedId: string
 inspectedPackageId: string
 inspectorCollapsed: boolean
 external: boolean
 exported: boolean
 kind: string
 query: string
 searchQuery: string
 offset: number
 panelOffset: number
 routeMethod: string
 symbolView: 'overview' | 'graph'
 databaseView: 'map' | 'list'
 flowRootId: string
 flowDepth: number
 includePossible: boolean
}
export function initialNavigation(inspectorCollapsed = false): NavigationState {
 return { analysisId: '', level: 'packages', groupId: '', packageId: '', selectedId: '', inspectedPackageId: '', inspectorCollapsed, external: false, exported: false, kind: '', query: '', searchQuery: '', offset: 0, panelOffset: 0, routeMethod: '', symbolView: 'overview', databaseView: 'map', flowRootId: '', flowDepth: 6, includePossible: false, connectionMode: 'all', connectionDirection: 'both', importDetail: 'simplified' }
}
export type NavigationAction =
 | { type: 'restore'; state: NavigationState }
 | { type: 'patch'; patch: Partial<NavigationState> }
 | { type: 'filters'; patch: Partial<Pick<NavigationState, 'query' | 'kind' | 'exported' | 'routeMethod' | 'external'>> }
 | { type: 'clearFilters' }
 | { type: 'navigate'; node: GraphNode }
 | { type: 'select'; node: GraphNode }
 | { type: 'overview'; level: Level }
 | { type: 'flow'; rootId: string }

export function navigationReducer(state: NavigationState, action: NavigationAction): NavigationState {
 switch (action.type) {
  case 'restore': return action.state
  case 'patch': return { ...state, ...action.patch }
  case 'filters': return { ...state, ...action.patch, offset: 0 }
  case 'clearFilters': return { ...state, query: '', kind: '', exported: false, routeMethod: '', offset: 0 }
  case 'select': {
   if (action.node.kind === 'package' && state.level === 'packages') {
    return { ...state, selectedId: action.node.id, inspectedPackageId: action.node.id, panelOffset: state.inspectedPackageId === action.node.id ? state.panelOffset : 0 }
   }
   return { ...state, selectedId: action.node.id, inspectorCollapsed: false }
  }
  case 'navigate': {
   const node = action.node
   const next = { ...navigationReducer(state, { type: 'clearFilters' }), selectedId: node.id, inspectorCollapsed: false, inspectedPackageId: '', panelOffset: 0, packageId: '', groupId: '' }
   if (['container','dockerStage','containerNetwork','containerVolume'].includes(node.kind)) return { ...next, level: 'containers' }
   if (node.kind === 'httpRoute') return { ...next, level: 'http' }
   if (node.kind === 'table' || node.kind === 'index') return { ...next, level: 'database' }
   if (node.kind === 'package') return { ...next, level: 'symbols', packageId: node.id, symbolView: 'overview' }
   if (node.kind === 'directory' || node.kind === 'module') return { ...next, level: 'directories', groupId: node.kind === 'module' ? '' : node.id }
   if (node.packageId) return { ...next, level: 'symbols', packageId: node.packageId, symbolView: 'overview' }
   // External/dynamic boundaries have no package view. Keep their current graph.
   return { ...state, selectedId: node.id, inspectorCollapsed: false }
  }
  case 'overview': return { ...navigationReducer(state, { type: 'clearFilters' }), level: action.level, inspectorCollapsed: action.level === 'containers' ? true : state.inspectorCollapsed, groupId: '', packageId: '', selectedId: '', inspectedPackageId: '', panelOffset: 0 }
  case 'flow': return { ...navigationReducer(state, { type: 'overview', level: 'flow' }), flowRootId: action.rootId, external: false, inspectorCollapsed: true }
 }
}

const levels: Level[] = ['packages', 'directories', 'symbols', 'http', 'database', 'flow', 'containers']
function integer(value: string | null, fallback: number, min = 0, max = 1_000_000_000): number {
 if (value === null || !/^\d+$/.test(value)) return fallback
 const number = Number(value)
 return Number.isSafeInteger(number) && number >= min && number <= max ? number : fallback
}
export function readNavigation(search: string, collapsed = false): NavigationState {
 const params = new URLSearchParams(search), state = initialNavigation(collapsed)
 for (const key of ['analysisId', 'groupId', 'packageId', 'selectedId', 'inspectedPackageId', 'flowRootId', 'query', 'searchQuery', 'kind', 'routeMethod'] as const) {
  state[key] = params.get(key)?.slice(0, key === 'query' || key === 'searchQuery' ? 1000 : 200) ?? ''
 }
 if (levels.includes(params.get('level') as Level)) state.level = params.get('level') as Level
 for (const key of ['external', 'exported', 'includePossible'] as const) state[key] = params.get(key) === 'true'
 if (params.has('inspector')) state.inspectorCollapsed = params.get('inspector') === 'closed'
 state.offset = integer(params.get('offset'), 0)
 state.panelOffset = integer(params.get('panelOffset'), 0)
 state.flowDepth = integer(params.get('flowDepth'), 6, 1, 10)
 if (params.get('connectionMode') === 'selection') state.connectionMode = 'selection'
 if (['incoming','outgoing'].includes(params.get('connectionDirection') ?? '')) state.connectionDirection = params.get('connectionDirection') as NavigationState['connectionDirection']
 if (params.get('importDetail') === 'all') state.importDetail = 'all'
 if (params.get('symbolView') === 'graph') state.symbolView = 'graph'
 if (params.get('databaseView') === 'list') state.databaseView = 'list'
 if (state.level === 'symbols' && !state.packageId) state.level = 'packages'
 return state
}
export function navigationURL(state: NavigationState): string {
 const params = new URLSearchParams(), defaults = initialNavigation()
 for (const key of Object.keys(defaults) as (keyof NavigationState)[]) {
  if (key === 'inspectorCollapsed') continue
  if (state[key] !== defaults[key]) params.set(key, String(state[key]))
 }
 params.set('level', state.level)
 params.set('inspector', state.inspectorCollapsed ? 'closed' : 'open')
 return `${window.location.pathname}?${params}`
}

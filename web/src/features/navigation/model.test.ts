import { describe, expect, it } from 'vitest'
import { initialNavigation, navigationReducer, navigationURL, readNavigation } from './model'
import type { GraphNode } from '../../api/types'

describe('navigation URLs', () => {
 it('round-trips a filtered source view and its associated flow and graph options', () => {
  const state = { ...initialNavigation(), analysisId: 'analysis', level: 'symbols' as const, packageId: 'pkg', selectedId: 'method', inspectedPackageId: 'pkg', flowRootId: 'route', inspectorCollapsed: true, query: 'Type & func()', searchQuery: 'имя метода', offset: 200, panelOffset: 400, exported: true, kind: 'method', external: true, routeMethod: 'POST', flowDepth: 8, includePossible: true, symbolView: 'graph' as const, databaseView: 'list' as const, connectionMode: 'selection' as const, connectionDirection: 'incoming' as const, importDetail: 'all' as const }
  expect(readNavigation(new URL(navigationURL(state), window.location.origin).search)).toEqual(state)
 })
 it('normalizes malformed URL values before sending requests', () => {
  const state = readNavigation('?level=symbols&offset=-1&panelOffset=Infinity&flowDepth=99&external=not-true&connectionDirection=wrong')
  expect(state.level).toBe('packages')
  expect(state.offset).toBe(0)
  expect(state.panelOffset).toBe(0)
  expect(state.flowDepth).toBe(6)
  expect(state.external).toBe(false)
  expect(state.connectionDirection).toBe('both')
 })
 it('opens a source in its owning package without carrying incompatible filters', () => {
  const node: GraphNode = { id: 'fn', kind: 'function', label: 'Handler', packageId: 'pkg', attributes: { exported: true } }
  const state = navigationReducer({ ...initialNavigation(), level: 'http', kind: 'table', routeMethod: 'POST', query: '/api', offset: 200, inspectorCollapsed: true }, { type: 'navigate', node })
  expect(state).toMatchObject({ level: 'symbols', packageId: 'pkg', selectedId: 'fn', kind: '', query: '', routeMethod: '', offset: 0, inspectorCollapsed: false })
 })
})

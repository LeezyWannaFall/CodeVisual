import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import App from './App'
import type { GraphNode, GraphView } from './api/types'

vi.mock('./features/graph/GraphCanvas', () => ({ GraphCanvas: ({ view, onSelect, onOpen }: {view: GraphView; onSelect:(n: GraphNode)=>void; onOpen:(n: GraphNode)=>void}) => <div>{view.nodes.map(n => <button key={n.id} onClick={() => onSelect(n)} onDoubleClick={() => onOpen(n)}>{n.label} graph node</button>)}</div> }))
const pkg: GraphNode = { id: 'pkg', kind: 'package', label: 'demo', parentId: 'root', attributes: { importPath: 'example.test/demo', relativePath: '.', exported: false } }
const symbol: GraphNode = { id: 'fn', kind: 'function', label: 'Hello', packageId: 'pkg', parentId: 'pkg', attributes: { exported: true, signature: 'func Hello()' }, source: { fileId: 'file', startLine: 2, startColumn: 1, endLine: 2, endColumn: 16 } }
let requests: string[]
beforeEach(() => {
 requests = []
 vi.stubGlobal('fetch', vi.fn(async (input: string) => {
  requests.push(input)
  const url = new URL(input, 'http://localhost')
  const data = url.pathname.endsWith('/project') ? { schemaVersion: 1, analysisId: 'a', status: 'partial', diagnosticCount: 1, project: { modulePath: 'example.test/demo', rootName: 'demo', buildContext: { goos: 'linux', goarch: 'amd64', tags: [], cgoEnabled: false, configurationKnown: true } }, statistics: { package: 1, function: 1 } }
   : url.pathname.endsWith('/tree') ? [{ id: 'root', kind: 'module', label: 'demo', attributes: { exported: false } }, pkg]
   : url.pathname.endsWith('/diagnostics') ? [{ severity: 'warning', code: 'package_load', message: 'Missing dependency' }]
   : url.pathname.includes('/files/') ? { id: 'file', relativePath: 'demo.go', content: 'package demo\nfunc Hello() {}' }
   : url.pathname.endsWith('/search') ? { nodes: [symbol], total: 1 }
   : { analysisId: 'a', nodes: url.searchParams.get('q') === 'absent' ? [] : url.searchParams.get('level') === 'symbols' ? [symbol] : [pkg], edges: [], totalNodes: 1, hiddenNodes: 0 }
  return { ok: true, json: async () => data }
 }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
describe('architecture explorer', () => {
 it('navigates package → symbol → source and exposes diagnostics', async () => {
  render(<App/>)
  fireEvent.doubleClick(await screen.findByRole('button', { name: 'demo graph node' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Hello graph node' }))
  expect(await screen.findByText('demo.go')).toBeInTheDocument()
  expect(screen.getByLabelText('Source code')).toHaveTextContent('func Hello() {}')
  fireEvent.click(screen.getByRole('button', { name: /Partial analysis/ }))
  expect(screen.getByText('Missing dependency')).toBeInTheDocument()
 })
 it('sends filters to the API and renders empty results', async () => {
  render(<App/>)
  await screen.findByRole('button', { name: 'demo graph node' })
  fireEvent.click(screen.getByLabelText('External dependencies'))
  await waitFor(() => expect(requests.some(p => p.includes('includeExternal=true'))).toBe(true))
  fireEvent.change(screen.getByLabelText('Filter graph'), { target: { value: 'absent' } })
  expect(await screen.findByText('No nodes in this view')).toBeInTheDocument()
 })
 it('finds symbols outside the current graph through global search', async () => {
  render(<App/>)
  fireEvent.change(screen.getByLabelText('Search packages and symbols'), { target: { value: 'Hello' } })
  fireEvent.click(await screen.findByRole('button', { name: 'ƒ Hello function' }))
  expect(await screen.findByText('demo.go')).toBeInTheDocument()
 })
})

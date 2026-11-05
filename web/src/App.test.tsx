import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import App from './App'
import type { GraphNode, GraphView } from './api/types'

vi.mock('./features/graph/GraphCanvas', () => ({ GraphCanvas: ({ view, onSelect, onOpen }: {view: GraphView; onSelect:(n: GraphNode)=>void; onOpen:(n: GraphNode)=>void}) => <div>{view.nodes.map(n => <button key={n.id} onClick={() => onSelect(n)} onDoubleClick={() => onOpen(n)}>{n.label} graph node</button>)}</div> }))
const pkg: GraphNode = { id: 'pkg', kind: 'package', label: 'demo', parentId: 'root', attributes: { importPath: 'example.test/demo', relativePath: '.', exported: false } }
const symbol: GraphNode = { id: 'fn', kind: 'function', label: 'Hello', packageId: 'pkg', parentId: 'pkg', attributes: { exported: true, signature: 'func Hello()' }, source: { fileId: 'file', startLine: 2, startColumn: 1, endLine: 2, endColumn: 16 } }
let requests: string[]
beforeEach(() => {
 window.history.replaceState({}, '', '/')
 requests = []
 vi.stubGlobal('fetch', vi.fn(async (input: string) => {
  requests.push(input)
  const url = new URL(input, 'http://localhost')
  const data = url.pathname.endsWith('/project') ? { schemaVersion: 1, analysisId: 'a', status: 'partial', diagnosticCount: 1, project: { modulePath: 'example.test/demo', rootName: 'demo', buildContext: { goos: 'linux', goarch: 'amd64', tags: [], cgoEnabled: false, configurationKnown: true } }, statistics: { package: 1, function: 1 } }
   : url.pathname.endsWith('/tree') ? [{ id: 'root', kind: 'module', label: 'demo', attributes: { exported: false } }, pkg]
   : url.pathname.endsWith('/diagnostics') ? [{ severity: 'warning', code: 'package_load', message: 'Missing dependency' }]
   : url.pathname.includes('/nodes/') ? (url.pathname.endsWith('/fn') ? symbol : pkg)
   : url.pathname.includes('/files/') ? { id: 'file', relativePath: 'demo.go', content: 'package demo\nfunc Hello() {}' }
   : url.pathname.endsWith('/search') ? { nodes: [symbol], total: 1 }
   : { analysisId: 'a', nodes: url.searchParams.get('q') === 'absent' ? [] : url.searchParams.get('level') === 'symbols' ? [symbol] : [pkg], edges: [], totalNodes: 1, hiddenNodes: 0 }
  return { ok: true, json: async () => data }
 }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
describe('architecture explorer', () => {
 it('switches themes and restores the saved choice after remounting', async () => {
  const saved = new Map<string, string>()
  vi.stubGlobal('localStorage', { getItem: (key: string) => saved.get(key) ?? null, setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key) })
  const first = render(<App/>)
  fireEvent.click(screen.getByRole('button', { name: 'Switch to light theme' }))
  expect(document.documentElement).toHaveAttribute('data-theme', 'light')
  expect(localStorage.getItem('codevisual.theme')).toBe('light')
  first.unmount()
  const second = render(<App/>)
  expect(screen.getByRole('button', { name: 'Switch to dark theme' })).toBeInTheDocument()
  expect(document.documentElement).toHaveAttribute('data-theme', 'light')
  fireEvent.click(screen.getByRole('button', { name: 'Switch to dark theme' }))
  expect(document.documentElement).toHaveAttribute('data-theme', 'dark')
  expect(localStorage.getItem('codevisual.theme')).toBe('dark')
  await screen.findByRole('button', { name: 'demo graph node' })
  second.unmount()
  localStorage.removeItem('codevisual.theme')
 })
 it('navigates package → symbol → source and exposes diagnostics', async () => {
  render(<App/>)
  fireEvent.doubleClick(await screen.findByRole('button', { name: 'demo graph node' }))
  fireEvent.click(await screen.findByRole('button', { name: /Hello public/ }))
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
  expect(await screen.findByText('No matches for these filters')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Clear view filters' }))
  expect(await screen.findByRole('button', { name: 'demo graph node' })).toBeInTheDocument()
  expect(screen.getByLabelText('Filter graph')).toHaveValue('')
 })
 it('finds symbols outside the current graph through global search', async () => {
  render(<App/>)
  fireEvent.change(screen.getByLabelText('Search packages and symbols'), { target: { value: 'Hello' } })
  fireEvent.click(await screen.findByRole('button', { name: 'ƒ Hello function' }))
  expect(await screen.findByText('demo.go')).toBeInTheDocument()
 })
})

it('finds a flow entry point beyond the first 200 routes', async () => {
 const original = globalThis.fetch
 const routes: GraphNode[] = Array.from({ length: 201 }, (_, i) => ({
  id: `route-${i}`, kind: 'httpRoute', label: `GET /route-${i}`, attributes: { exported: false, http: { method: 'GET', path: `/route-${i}`, handler: 'Hello', middleware: [], scope: 'demo.Routes' } },
 }))
 vi.stubGlobal('fetch', vi.fn(async (input: string, init?: RequestInit) => {
  const url = new URL(input, 'http://localhost')
  if (url.pathname.endsWith('/graph') && url.searchParams.get('level') === 'http') {
   const offset = Number(url.searchParams.get('offset') ?? 0)
   const nodes = routes.slice(offset, offset + 200)
   return { ok: true, json: async () => ({ analysisId: 'a', nodes, edges: [], totalNodes: routes.length, hiddenNodes: routes.length - nodes.length }) }
  }
  return original(input, init)
 }))
 render(<App/> )
 await screen.findByRole('button', { name: 'demo graph node' })
 fireEvent.click(screen.getByRole('button', { name: /Request Flow/ }))
 fireEvent.click(screen.getByRole('combobox', { name: 'Flow entry point' }))
 fireEvent.change(screen.getByLabelText('Search routes'), { target: { value: '/route-200' } })
 fireEvent.click(screen.getByRole('option', { name: /route-200/ }))
 expect(screen.getByRole('combobox', { name: 'Flow entry point' })).toHaveTextContent('/route-200')
 await waitFor(() => expect(requests.some(p => p.includes('rootId=route-200'))).toBe(true))
})

it('shows a later route-page error and retries loading', async () => {
 const original = globalThis.fetch
 let fail = true
 vi.stubGlobal('fetch', vi.fn(async (input: string, init?: RequestInit) => {
  const url = new URL(input, 'http://localhost')
  if (url.pathname.endsWith('/graph') && url.searchParams.get('level') === 'http') {
   const offset = Number(url.searchParams.get('offset') ?? 0)
   if (offset > 0 && fail) return { ok: false, status: 503, json: async () => ({ error: { code: 'unavailable', message: 'Route page unavailable' } }) }
   return { ok: true, json: async () => ({ analysisId: 'a', nodes: [pkg], edges: [], totalNodes: 2, hiddenNodes: 1 }) }
  }
  return original(input, init)
 }))
 render(<App/> )
 expect(await screen.findByRole('alert')).toHaveTextContent('Route page unavailable')
 fail = false
 fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
 expect(await screen.findByRole('button', { name: 'demo graph node' })).toBeInTheDocument()
})

it('restores a source link, filters, and declaration mode after remounting', async () => {
 const first = render(<App/> )
 fireEvent.doubleClick(await screen.findByRole('button', { name: 'demo graph node' }))
 fireEvent.click(await screen.findByRole('button', { name: /Hello public/ }))
 fireEvent.click(screen.getByRole('button', { name: 'Declaration graph' }))
 fireEvent.change(screen.getByLabelText('Symbol kind'), { target: { value: 'function' } })
 fireEvent.change(screen.getByLabelText('Filter graph'), { target: { value: 'Hello' } })
 await screen.findByLabelText('Source code')
 const link = new URL(window.location.href)
 expect(link.searchParams.get('selectedId')).toBe('fn')
 expect(link.searchParams.get('packageId')).toBe('pkg')
 expect(link.searchParams.get('query')).toBe('Hello')
 first.unmount()
 render(<App/> )
 expect(await screen.findByLabelText('Source code')).toHaveTextContent('func Hello() {}')
 expect(screen.getByLabelText('Filter graph')).toHaveValue('Hello')
 expect(screen.getByLabelText('Symbol kind')).toHaveValue('function')
 expect(screen.getByRole('button', { name: 'Declaration graph' })).toHaveAttribute('aria-pressed', 'true')
})

it('recovers from a link whose object is missing', async () => {
 const original = globalThis.fetch
 window.history.replaceState({}, '', '/?analysisId=a&level=symbols&packageId=missing&selectedId=fn')
 vi.stubGlobal('fetch', vi.fn(async (input: string, init?: RequestInit) => {
  if (input.includes('/nodes/missing')) return { ok: false, status: 404, json: async () => ({ error: { code: 'node_not_found', message: 'Node not found' } }) }
  return original(input, init)
 }))
 render(<App/> )
 expect(await screen.findByText(/The linked object is unavailable/)).toBeInTheDocument()
 expect(await screen.findByRole('button', { name: 'demo graph node' })).toBeInTheDocument()
 expect(new URLSearchParams(window.location.search).has('packageId')).toBe(false)
})

it('does not resolve IDs from a different analysis', async () => {
 window.history.replaceState({}, '', '/?analysisId=old&level=symbols&packageId=missing&selectedId=old-fn')
 render(<App/> )
 expect(await screen.findByText(/This link belongs to a different analysis/)).toBeInTheDocument()
 await screen.findByRole('button', { name: 'demo graph node' })
 expect(requests.some(path => path.includes('/nodes/'))).toBe(false)
 expect(new URLSearchParams(window.location.search).get('analysisId')).toBe('a')
})

it('copies the current view link', async () => {
 const writeText = vi.fn().mockResolvedValue(undefined)
 Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
 render(<App/> )
 await screen.findByRole('button', { name: 'demo graph node' })
 fireEvent.click(screen.getByRole('button', { name: 'Copy view link' }))
 await waitFor(() => expect(writeText).toHaveBeenCalledWith(window.location.href))
 expect(await screen.findByText(/View link copied/)).toBeInTheDocument()
 Reflect.deleteProperty(navigator, 'clipboard')
})

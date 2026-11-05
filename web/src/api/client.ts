import type { GraphNode, GraphView } from './types'

export class APIError extends Error {
 constructor(public status: number, public code: string, message: string) { super(message) }
}
export async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
 const res = await fetch(`/api/v1${path}`, { signal })
 if (!res.ok) {
  const body = await res.json().catch(() => null)
  throw new APIError(res.status, body?.error?.code ?? 'request_failed', body?.error?.message ?? `Request failed (${res.status})`)
 }
 return res.json() as Promise<T>
}

// Graph pages are bounded; the entry-point picker needs every route.
export async function getRoutes(signal?: AbortSignal): Promise<GraphNode[]> {
 const nodes: GraphNode[] = []
 let analysisId: string | undefined
 while (true) {
  const page = await get<GraphView>(`/graph?level=http&offset=${nodes.length}`, signal)
  if (analysisId !== undefined && analysisId !== page.analysisId) throw new Error('Analysis changed while loading routes. Reload the page.')
  analysisId = page.analysisId
  nodes.push(...page.nodes)
  if (nodes.length >= page.totalNodes) return nodes.filter(n => n.kind === 'httpRoute')
  if (page.nodes.length === 0) throw new Error('Could not load all routes. Try again.')
 }
}

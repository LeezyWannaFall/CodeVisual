import type { GraphEdge } from '../../api/types'

// Reduce the condensation DAG, retaining every edge within a strongly connected
// component. This preserves cycles and reachability without inventing imports.
export function simplifyImports(edges: GraphEdge[]): GraphEdge[] {
 const adjacency = new Map<string, Set<string>>()
 for (const edge of edges) {
  if (edge.kind !== 'imports') continue
  if (!adjacency.has(edge.sourceId)) adjacency.set(edge.sourceId, new Set())
  adjacency.get(edge.sourceId)!.add(edge.targetId)
 }
 const reach = new Map<string, Set<string>>()
 for (const source of adjacency.keys()) {
  const visited = new Set<string>(), pending = [...adjacency.get(source)!]
  while (pending.length) {
   const next = pending.pop()!
   if (visited.has(next)) continue
   visited.add(next)
   pending.push(...(adjacency.get(next) ?? []))
  }
  reach.set(source, visited)
 }
 const sameComponent = (a: string, b: string) => a === b || (!!reach.get(a)?.has(b) && !!reach.get(b)?.has(a))
 return edges.filter(edge => {
  if (edge.kind !== 'imports' || sameComponent(edge.sourceId, edge.targetId)) return true
  for (const middle of reach.get(edge.sourceId) ?? []) {
   if (!sameComponent(middle, edge.sourceId) && !sameComponent(middle, edge.targetId) && reach.get(middle)?.has(edge.targetId)) return false
  }
  return true
 })
}

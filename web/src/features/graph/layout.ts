import { columnPortY } from './schema'
import type { ElkNode, ElkPoint } from 'elkjs/lib/elk-api'
import type { GraphEdge, GraphNode, GraphView } from '../../api/types'

export interface Port { id: string; side: 'in' | 'out'; y: number }
export interface PlacedNode { entity: GraphNode; x: number; y: number; width: number; height: number; ports: Port[] }
export interface PlacedEdge { entity: GraphEdge; points: ElkPoint[]; sourceHandle: string; targetHandle: string }
export interface GraphLayout { nodes: PlacedNode[]; edges: PlacedEdge[] }
export interface LayoutEngine { layout(graph: ElkNode): Promise<ElkNode> }
const portId = (edge: GraphEdge, side: 'in' | 'out') => `${side}:${edge.id}`

// Every relationship gets its own port. Keeping ELK's routed bend points is
// essential: drawing a new smoothstep path would discard obstacle avoidance.
export async function layoutGraph(view: GraphView, engine: LayoutEngine, expanded: ReadonlySet<string> = new Set(), completion: ReadonlySet<string> = new Set()): Promise<GraphLayout> {
 const nodes = [...view.nodes].sort((a, b) => a.id.localeCompare(b.id))
 const edges = [...view.edges].sort((a, b) => a.id.localeCompare(b.id))
 const input: ElkNode = {
  id: 'architecture',
  layoutOptions: {
   'elk.algorithm': 'layered',
   'elk.direction': 'RIGHT',
   'elk.edgeRouting': 'ORTHOGONAL',
   'elk.padding': '[top=40,left=40,bottom=40,right=40]',
   'elk.spacing.nodeNode': '52',
   'elk.spacing.edgeEdge': '18',
   'elk.spacing.edgeNode': '28',
   'elk.layered.spacing.nodeNodeBetweenLayers': '110',
   'elk.layered.spacing.edgeEdgeBetweenLayers': '18',
   'elk.layered.spacing.edgeNodeBetweenLayers': '28',
   'elk.layered.mergeEdges': 'false',
   'elk.layered.nodePlacement.strategy': 'NETWORK_SIMPLEX',
   'elk.layered.crossingMinimization.strategy': 'LAYER_SWEEP',
   'elk.randomSeed': '1',
  },
  children: nodes.map(n => {
   const incoming = edges.filter(e => e.targetId === n.id)
   const outgoing = edges.filter(e => e.sourceId === n.id)
   return {
    id: n.id,
    width: n.attributes.table ? 280 : n.attributes.index ? 300 : 260,
    height: Math.max(n.attributes.container ? 150 + (completion.has(n.id) ? 30 : 0) + (n.attributes.container.ports.length ? 32 : 0) + (expanded.has(n.id) ? Math.min(180, Math.max(40, n.attributes.container.contents.length * 100)) : 0) : n.attributes.table ? 34 + n.attributes.table.columns.length * 26 + (n.attributes.members?.length ? 5 + n.attributes.members.length * 24 : 0) : n.attributes.index ? 176 : n.attributes.possible || (n.attributes.resolution && n.attributes.resolution !== 'static') ? 136 : 112, (Math.max(incoming.length, outgoing.length) + 1) * 18),
    layoutOptions: { 'elk.portConstraints': n.attributes.table ? 'FIXED_POS' : 'FIXED_ORDER' },
    ports: [
     ...incoming.map(e => ({ id: portId(e, 'in'), ...(n.attributes.table ? { x: 0, y: columnPortY(n, e.targetColumn) } : {}), width: 0, height: 0, layoutOptions: { 'elk.port.side': 'WEST' } })),
     ...outgoing.map(e => ({ id: portId(e, 'out'), ...(n.attributes.table ? { x: 280, y: columnPortY(n, e.sourceColumn) } : {}), width: 0, height: 0, layoutOptions: { 'elk.port.side': 'EAST' } })),
    ],
   }
  }),
  edges: edges.map(e => ({ id: e.id, sources: [portId(e, 'out')], targets: [portId(e, 'in')] })),
 }
 const output = await engine.layout(input)
 const byNode = new Map(nodes.map(n => [n.id, n]))
 const byEdge = new Map(edges.map(e => [e.id, e]))
 return {
  nodes: (output.children ?? []).map(n => ({
   entity: byNode.get(n.id)!, x: n.x ?? 0, y: n.y ?? 0, width: n.width!, height: n.height!,
   ports: (n.ports ?? []).map(p => ({ id: p.id, side: p.id.startsWith('in:') ? 'in' : 'out', y: p.y ?? 0 })),
  })),
  edges: (output.edges ?? []).map(e => {
   const section = e.sections?.[0]
   if (!section) throw new Error('The layout engine could not route a relationship.')
   return { entity: byEdge.get(e.id)!, sourceHandle: portId(byEdge.get(e.id)!, 'out'), targetHandle: portId(byEdge.get(e.id)!, 'in'), points: [section.startPoint, ...(section.bendPoints ?? []), section.endPoint] }
  }),
 }
}

export type ConnectionMode = 'selection' | 'all'
export type ConnectionDirection = 'both' | 'incoming' | 'outgoing'
export function visibleConnections(edges: GraphEdge[], selectedId: string | undefined, mode: ConnectionMode, direction: ConnectionDirection): Set<string> {
 return new Set(edges.filter(e => !selectedId || mode === 'all' ||
  (direction !== 'incoming' && e.sourceId === selectedId) ||
  (direction !== 'outgoing' && e.targetId === selectedId)).map(e => e.id))
}

// Align routed terminal legs with measured handle coordinates while keeping
// all other obstacle-aware bends unchanged.
export function movedPoints(points: ElkPoint[], start: ElkPoint, end: ElkPoint): ElkPoint[] {
 if (points.length < 2) return [start, end]
 const result = points.map(p => ({ ...p }))
 const oldStart = points[0], oldEnd = points[points.length - 1]
 if (result.length === 2) {
  if (start.y === end.y) return [start, end]
  const midX = (start.x + end.x) / 2
  return [start, { x: midX, y: start.y }, { x: midX, y: end.y }, end]
 }
 if (points[1].y === oldStart.y) result[1].y = start.y
 else result[1].x = start.x
 const last = result.length - 2
 if (points[last].y === oldEnd.y) result[last].y = end.y
 else result[last].x = end.x
 result[0] = start; result[result.length - 1] = end
 return result
}

export function roundedPath(points: ElkPoint[]): string {
 const clean = points.filter((p, i) => i === 0 || p.x !== points[i - 1].x || p.y !== points[i - 1].y)
 if (clean.length === 0) return ''
 let path = `M ${clean[0].x} ${clean[0].y}`
 for (let i = 1; i < clean.length - 1; i++) {
  const prev = clean[i - 1], corner = clean[i], next = clean[i + 1]
  const before = Math.hypot(corner.x - prev.x, corner.y - prev.y)
  const after = Math.hypot(next.x - corner.x, next.y - corner.y)
  const radius = Math.min(9, before / 2, after / 2)
  const a = { x: corner.x + (prev.x - corner.x) * radius / before, y: corner.y + (prev.y - corner.y) * radius / before }
  const b = { x: corner.x + (next.x - corner.x) * radius / after, y: corner.y + (next.y - corner.y) * radius / after }
  path += ` L ${a.x} ${a.y} Q ${corner.x} ${corner.y} ${b.x} ${b.y}`
 }
 const last = clean[clean.length - 1]
 return `${path} L ${last.x} ${last.y}`
}

export interface GraphOptions { connectionMode: ConnectionMode; connectionDirection: ConnectionDirection; importDetail: 'simplified' | 'all' }

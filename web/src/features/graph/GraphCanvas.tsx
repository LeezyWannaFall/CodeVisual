import { useEffect, useMemo, useState } from 'react'
import { Background, BackgroundVariant, Controls, Handle, MarkerType, Panel, Position, ReactFlow, ReactFlowProvider, useEdgesState, useNodesState, useReactFlow, useStore, type Node, type NodeProps } from '@xyflow/react'
import type { GraphNode, GraphView } from '../../api/types'
import { KindBadge } from '../../components/Icon'
import { layoutGraph, visibleConnections, type ConnectionDirection, type ConnectionMode, type GraphLayout, type Port } from './layout'
import { RoutedEdge, type RoutedGraphEdge } from './RoutedEdge'

type VisualNode = Node<{ entity: GraphNode; ports: Port[]; active: boolean; muted: boolean; incoming: number; outgoing: number; imports: boolean }, 'entity'>
function Entity({ data }: NodeProps<VisualNode>) {
 const n = data.entity
 const table = n.attributes.table
 const index = n.attributes.index
 return <div className={`entity-node nopan ${table ? 'table-map-node' : ''} ${index ? 'index-map-node' : ''} ${data.active ? 'active' : ''} ${data.muted ? 'muted' : ''}`}>
  {data.ports.map(port => <Handle key={port.id} id={port.id} type={port.side === 'in' ? 'target' : 'source'} position={port.side === 'in' ? Position.Left : Position.Right} style={{ top: port.y }} />)}
  <div className="flex items-center gap-2.5"><KindBadge kind={n.kind}/><span className="node-name">{n.label}</span><span className="node-dot"/></div>
  <div className="node-meta">{n.attributes.unresolved ? 'source unavailable' : n.kind === 'externalPackage' ? (n.attributes.standard ? 'standard library' : 'external dependency') : table ? `${table.columns.length} columns · ${table.references.length} foreign keys` : index ? n.attributes.relativePath : n.kind === 'package' ? n.attributes.relativePath || '.' : n.kind}</div>
  {table && <div className="map-table-columns">{table.columns.map((column, i) => {
   const primary = /\bPRIMARY\s+KEY\b/i.test(column.constraints ?? '') || table.constraints.some(c => /\bPRIMARY\s+KEY\b/i.test(c) && c.slice(c.indexOf('(') + 1, c.lastIndexOf(')')).split(',').some(name => name.trim().replace(/^"|"$/g, '') === column.name.replace(/^"|"$/g, '')))
   const foreign = table.references.some(r => r.columns.split(',').some(name => name.trim() === column.name))
   return <div key={i} className="map-column" title={`${column.name}: ${column.type} ${column.constraints ?? ''}`}><span className="map-column-key">{primary ? 'PK' : foreign ? 'FK' : '·'}</span><span className="map-column-name">{column.name}</span><span className="map-column-type">{column.type}</span></div>
  })}</div>}
  {index && <div className="map-index-target"><span>{index.unique ? 'UNIQUE INDEX' : 'INDEX'}</span><code>on {index.table}</code></div>}
  <div className="node-connections"><span title={data.imports ? 'Packages in this view importing this package' : 'Incoming relationships'}>{data.imports ? 'Imported by' : 'Incoming'} {data.incoming}</span><span title={data.imports ? 'Packages imported by this package in this view' : 'Outgoing relationships'}>{data.imports ? 'Imports' : 'Outgoing'} {data.outgoing}</span></div>
 </div>
}
const nodeTypes = { entity: Entity }
const edgeTypes = { routed: RoutedEdge }

function Canvas({ view, selected, onSelect, onOpen }: Props) {
 const { fitView } = useReactFlow()
 const width = useStore(s => s.width)
 const height = useStore(s => s.height)
 const [nodes, setNodes, onNodesChange] = useNodesState<VisualNode>([])
 const [edges, setEdges, onEdgesChange] = useEdgesState<RoutedGraphEdge>([])
 const [layout, setLayout] = useState<GraphLayout | null>(null)
 const [revision, setRevision] = useState(0)
 const [arranging, setArranging] = useState(true)
 const [error, setError] = useState('')
 const [mode, setMode] = useState<ConnectionMode>('all')
 const [direction, setDirection] = useState<ConnectionDirection>('both')
 const selection = view.nodes.find(n => n.id === selected?.id)
 const incoming = view.edges.filter(e => e.targetId === selection?.id).length
 const outgoing = view.edges.filter(e => e.sourceId === selection?.id).length
 const imports = view.nodes.every(n => ['package', 'externalPackage', 'directory'].includes(n.kind)) && view.edges.every(e => e.kind === 'imports')
 const shown = useMemo(() => visibleConnections(view.edges, selection?.id, mode, direction), [view, selection?.id, mode, direction])
 const related = useMemo(() => {
  const ids = new Set<string>()
  if (selection) { ids.add(selection.id); for (const e of view.edges) if (shown.has(e.id)) { ids.add(e.sourceId); ids.add(e.targetId) } }
  return ids
 }, [view, selection, shown])
 const layoutView = useMemo<GraphView>(() => mode === 'selection' && selection
  ? { ...view, nodes: view.nodes.filter(n => related.has(n.id)), edges: view.edges.filter(e => shown.has(e.id)) }
  : view, [view, mode, selection, related, shown])
 useEffect(() => {
  let cancelled = false
  let engine: ReturnType<typeof import('./layoutWorker').createLayoutEngine> | undefined
  let stopped = false
  const stop = () => { if (engine && !stopped) { stopped = true; engine.terminateWorker() } }
  setArranging(true); setError('')
  import('./layoutWorker').then(async ({ createLayoutEngine }) => {
   if (cancelled) return
   engine = createLayoutEngine()
   const next = await layoutGraph(layoutView, engine)
   if (cancelled) return
   setNodes(next.nodes.map(n => ({
    id: n.entity.id, type: 'entity', position: { x: n.x, y: n.y }, style: { width: n.width, height: n.height },
    data: { entity: n.entity, ports: n.ports, active: false, muted: false, incoming: view.edges.filter(e => e.targetId === n.entity.id).length, outgoing: view.edges.filter(e => e.sourceId === n.entity.id).length, imports },
   })))
   setLayout(next)
  }).catch(e => { if (!cancelled) setError(e.message ?? 'Could not arrange this graph.') }).finally(() => { stop(); if (!cancelled) setArranging(false) })
  return () => { cancelled = true; stop() }
 }, [layoutView, view, revision, setNodes, imports])
 useEffect(() => {
  setNodes(ns => ns.map(n => ({ ...n, data: { ...n.data, active: n.id === selection?.id, muted: mode === 'selection' && related.size > 0 && !related.has(n.id) } })))
  if (!layout) return
  const labels = new Map(view.nodes.map(n => [n.id, n.attributes.importPath || n.attributes.relativePath || n.label]))
  setEdges(layout.edges.filter(e => shown.has(e.entity.id)).map(({ entity: e, points, sourceHandle, targetHandle }) => {
   const relation = e.sourceId === selection?.id ? 'outgoing' : e.targetId === selection?.id ? 'incoming' : 'context'
   const color = relation === 'outgoing' ? '#3b82f6' : relation === 'incoming' ? '#a9caff' : '#686868'
   return {
    id: e.id, source: e.sourceId, target: e.targetId, sourceHandle, targetHandle, type: 'routed', animated: relation !== 'context',
    ariaLabel: `${labels.get(e.sourceId)} ${e.kind === 'imports' ? 'imports' : e.kind === 'references' ? 'has a foreign key to' : e.kind === 'indexes' ? 'indexes' : e.kind} ${labels.get(e.targetId)}`,
    data: { points, relation, description: `${labels.get(e.sourceId)} → ${labels.get(e.targetId)}${e.derived ? ` · ${e.count} imports (aggregated)` : ` · ${e.kind}`}` },
    style: { stroke: color, strokeWidth: relation === 'context' ? 1.4 : 2.2 }, zIndex: relation === 'context' ? 0 : 1,
    markerEnd: { type: MarkerType.ArrowClosed, color, width: 17, height: 17 },
   }
  }))
 }, [selection, shown, related, layout, view, setNodes, setEdges, mode])
 useEffect(() => {
  if (!layout || !width || !height) return
  const frame = requestAnimationFrame(() => { void fitView({ padding: 0.18, duration: 0, maxZoom: 1 }) })
  return () => cancelAnimationFrame(frame)
 }, [layout, fitView, width, height])
 return <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} edgeTypes={edgeTypes} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onNodeClick={(_, n) => onSelect(n.data.entity)} onNodeDoubleClick={(_, n) => onOpen(n.data.entity)} nodesConnectable={false} nodesDraggable={false} deleteKeyCode={null} colorMode="dark" minZoom={0.08} maxZoom={2} proOptions={{ hideAttribution: true }}>
  <Background variant={BackgroundVariant.Dots} color="#272727" gap={22} size={1}/>
  <Panel position="top-right" className="connection-tools">
   <div className="connection-mode" role="group" aria-label="Visible connections">
    <button aria-pressed={mode === 'all' || !selection} onClick={() => setMode('all')}>All connections</button>
    <button aria-pressed={mode === 'selection' && !!selection} disabled={!selection} onClick={() => setMode('selection')}>Selected node</button>
   </div>
   {selection && mode === 'selection' && <select aria-label="Connection direction" value={direction} onChange={e => setDirection(e.target.value as ConnectionDirection)}><option value="both">Incoming & outgoing</option><option value="incoming">Incoming only</option><option value="outgoing">Outgoing only</option></select>}
  </Panel>
  <Panel position="top-left" className="connection-summary">
   {selection ? <><strong>{selection.label}</strong><span className="incoming-key">{incoming} incoming</span><span className="outgoing-key">{outgoing} outgoing</span><span>{shown.size}/{view.edges.length} connections shown · {layoutView.nodes.length}/{view.nodes.length} nodes</span></> : <span>Click a node to inspect it · use Selected node to isolate · arrows follow {imports ? 'imports' : 'relationships'}</span>}
  </Panel>
  {arranging && <div className="layout-status" role="status">Arranging nodes and routing connections…</div>}
  {error && <div className="layout-status error-note" role="alert">{error}<button onClick={() => setRevision(v => v + 1)}>Retry layout</button></div>}
  <Controls showInteractive={false}/>
 </ReactFlow>
}
interface Props { view: GraphView; selected: GraphNode | null; onSelect: (n: GraphNode) => void; onOpen: (n: GraphNode) => void }
export function GraphCanvas(props: Props) { return <ReactFlowProvider><Canvas {...props}/></ReactFlowProvider> }

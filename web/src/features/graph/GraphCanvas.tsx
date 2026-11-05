import { SQLCode } from '../../components/SQLCode'
import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { Background, BackgroundVariant, Controls, Handle, MarkerType, Position, ReactFlow, ReactFlowProvider, useEdgesState, useNodesState, useReactFlow, useStore, type Node, type NodeProps } from '@xyflow/react'
import type { GraphNode, GraphView } from '../../api/types'
import { KindBadge } from '../../components/Icon'
import { layoutGraph, visibleConnections, type ConnectionDirection, type GraphOptions, type GraphLayout, type Port } from './layout'
import { schemaView, primaryColumn, columnNames } from './schema'
import { simplifyImports } from './simplify'
import { RoutedEdge, type RoutedGraphEdge } from './RoutedEdge'

const SchemaInteraction = createContext<{ indexes: Map<string, GraphNode[]>; selectedId?: string; onSelect: (node: GraphNode) => void }>({ indexes: new Map(), onSelect: () => {} })
type VisualNode = Node<{ entity: GraphNode; ports: Port[]; active: boolean; muted: boolean; incoming: number; outgoing: number; imports: boolean }, 'entity'>
function Entity({ data }: NodeProps<VisualNode>) {
 const schema = useContext(SchemaInteraction)
 const n = data.entity
 const indexes = schema.indexes.get(n.id) ?? []
 const table = n.attributes.table
 const index = n.attributes.index
 if (table) return <div className={`entity-node er-table table-map-node nopan ${data.active ? 'active' : ''}`}>
  {data.ports.map(port => <Handle key={port.id} id={port.id} type={port.side === 'in' ? 'target' : 'source'} position={port.side === 'in' ? Position.Left : Position.Right} style={{ top: port.y }} />)}
  <div className="er-table-title"><span className="node-name">{n.label}</span><span title="SQL table">▤</span></div>
  {table.columns.map((column,i) => { const primary = primaryColumn(n,column.name), foreign = table.references.some(r => columnNames(r.columns).includes(column.name)); return <div className="er-column" key={i} data-column={column.name} title={`${column.name}: ${column.type} ${column.constraints ?? ''}`}><span title={primary ? 'Primary key' : foreign ? 'Foreign key' : undefined} className={`er-key ${primary ? 'primary' : foreign ? 'foreign' : ''}`}>{(primary || foreign) && <svg viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="1.4"><circle cx="5" cy="5" r="3"/><path d="M7 7l6 6m-3-3 2-2m-4 0 2-2"/></svg>}</span><code>{column.name}</code><span className="er-type"><SQLCode text={column.type}/></span></div> })}
  {indexes.length > 0 && <div className="er-indexes">{indexes.map(index => <button key={index.id} className="er-index-button nodrag nopan" aria-label={`Open index ${index.label}`} aria-pressed={schema.selectedId === index.id} title={`${index.attributes.index?.unique ? 'Unique index' : 'Index'} · ${index.label}`} onDoubleClick={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()} onClick={event => { event.stopPropagation(); schema.onSelect(index) }}><span aria-hidden="true">≡</span><code>{index.label}</code><span className="index-open-arrow" aria-hidden="true">↗</span></button>)}</div>}
 </div>
 return <div className={`entity-node nopan ${table ? 'table-map-node' : ''} ${index ? 'index-map-node' : ''} ${data.active ? 'active' : ''} ${data.muted ? 'muted' : ''}`}>
  {data.ports.map(port => <Handle key={port.id} id={port.id} type={port.side === 'in' ? 'target' : 'source'} position={port.side === 'in' ? Position.Left : Position.Right} style={{ top: port.y }} />)}
  <div className="flex items-center gap-2.5"><KindBadge kind={n.kind}/><span className="node-name">{n.label}</span><span className="node-dot"/></div>
  <div className="node-meta">{n.attributes.unresolved ? 'source unavailable' : n.kind === 'externalPackage' ? (n.attributes.standard ? 'standard library' : 'external dependency') : index ? n.attributes.relativePath : n.kind === 'package' ? n.attributes.relativePath || '.' : n.attributes.relativePath || n.attributes.importPath || n.kind}</div>
  {index && <div className="map-index-target"><span>{index.unique ? 'UNIQUE INDEX' : 'INDEX'}</span><code>on {index.table}</code></div>}
  {(n.attributes.possible || (n.attributes.resolution && n.attributes.resolution !== 'static')) && <span className="node-resolution">{n.attributes.possible ? 'POSSIBLE IMPLEMENTATION' : n.attributes.resolution?.toUpperCase()}</span>}
  <div className="node-connections"><span title={data.imports ? 'Packages in this view importing this package' : 'Incoming relationships'}>{data.imports ? 'Imported by' : 'Incoming'} {data.incoming}</span><span title={data.imports ? 'Packages imported by this package in this view' : 'Outgoing relationships'}>{data.imports ? 'Imports' : 'Outgoing'} {data.outgoing}</span></div>
 </div>
}
const nodeTypes = { entity: Entity }
const edgeTypes = { routed: RoutedEdge }

function Canvas({ view: originalView, selected, onSelect, onOpen, options, onOptionsChange }: Props) {
 const schemaInteraction = useMemo(() => {
  const indexes = new Map<string, GraphNode[]>()
  for (const edge of originalView.edges.filter(e => e.kind === 'indexes')) {
   const index = originalView.nodes.find(n => n.id === edge.sourceId && n.kind === 'index')
   if (index) indexes.set(edge.targetId, [...(indexes.get(edge.targetId) ?? []), index])
  }
  return { indexes, selectedId: selected?.id, onSelect }
 }, [originalView, selected?.id, onSelect])
 const view = useMemo(() => schemaView(originalView), [originalView])
 const { fitView } = useReactFlow()
 const fittedLayout = useRef<GraphLayout | null>(null)
 const width = useStore(s => s.width)
 const height = useStore(s => s.height)
 const [nodes, setNodes, onNodesChange] = useNodesState<VisualNode>([])
 const [edges, setEdges, onEdgesChange] = useEdgesState<RoutedGraphEdge>([])
 const [layout, setLayout] = useState<GraphLayout | null>(null)
 const [revision, setRevision] = useState(0)
 const [arranging, setArranging] = useState(true)
 const [error, setError] = useState('')
 const { connectionMode: mode, importDetail: detail, connectionDirection: direction } = options
 const selection = view.nodes.find(n => n.id === selected?.id)
 const incoming = view.edges.filter(e => e.targetId === selection?.id).length
 const outgoing = view.edges.filter(e => e.sourceId === selection?.id).length
 const imports = view.nodes.every(n => ['package', 'externalPackage', 'directory'].includes(n.kind)) && view.edges.every(e => e.kind === 'imports')
 const reduced = useMemo(() => imports ? simplifyImports(view.edges) : view.edges, [view, imports])
 const overviewEdges = imports && detail === 'simplified' ? reduced : view.edges
 // Explicit focus always shows the selected package's actual direct imports.
 const baseEdges = mode === 'selection' && selection ? view.edges : overviewEdges
 const shown = useMemo(() => visibleConnections(baseEdges, selection?.id, mode, direction), [baseEdges, selection?.id, mode, direction])
 const related = useMemo(() => {
  const ids = new Set<string>()
  if (selection) { ids.add(selection.id); for (const e of view.edges) if (shown.has(e.id)) { ids.add(e.sourceId); ids.add(e.targetId) } }
  return ids
 }, [view, selection, shown])
 const overviewView = useMemo<GraphView>(() => ({ ...view, edges: overviewEdges }), [view, overviewEdges])
 const layoutView = useMemo<GraphView>(() => mode === 'selection' && selection
  ? { ...view, nodes: view.nodes.filter(n => related.has(n.id)), edges: view.edges.filter(e => shown.has(e.id)) }
  : overviewView, [view, mode, selection, related, shown, overviewView])
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
   const color = relation === 'outgoing' ? '#3b82f6' : relation === 'incoming' ? 'var(--accent-light)' : 'var(--graph-edge, #686868)'
   return {
    id: e.id, source: e.sourceId, target: e.targetId, sourceHandle, targetHandle, type: 'routed', animated: relation !== 'context',
    ariaLabel: e.label ?? `${labels.get(e.sourceId)} ${e.kind === 'imports' ? 'imports' : e.kind === 'references' ? 'has a foreign key to' : e.kind === 'indexes' ? 'indexes' : e.kind} ${labels.get(e.targetId)}`,
    data: { points, relation, description: e.label ?? `${labels.get(e.sourceId)} → ${labels.get(e.targetId)} · ${e.resolution ?? e.kind}${e.invocation && e.invocation !== 'call' ? ` · ${e.invocation}` : ''}${e.derived ? ` · ${e.count} imports (aggregated)` : ` · ${e.kind}`}` },
    style: { strokeDasharray: ['possible','interface','dynamic'].includes(e.resolution ?? '') ? '4 6' : undefined, stroke: color, strokeWidth: relation === 'context' ? 1.4 : 2.2 }, zIndex: relation === 'context' ? 0 : 1,
    markerEnd: e.kind === 'references' ? undefined : { type: MarkerType.ArrowClosed, color, width: 17, height: 17 },
   }
  }))
 }, [selection, shown, related, layout, view, setNodes, setEdges, mode])
 useEffect(() => {
  // Inspector resizing must preserve the user's pan and zoom; fit only a new layout.
  if (!layout || !width || !height || fittedLayout.current === layout) return
  const frame = requestAnimationFrame(() => { fittedLayout.current = layout; void fitView({ padding: 0.18, duration: 0, maxZoom: 1 }) })
  return () => cancelAnimationFrame(frame)
 }, [layout, fitView, width, height])
 return <SchemaInteraction.Provider value={schemaInteraction}><div className="graph-canvas-shell">
  <div className="graph-canvas-header">
   <div className="connection-summary" aria-live="polite" title={selection ? `${selection.label} · ${incoming} incoming · ${outgoing} outgoing · ${shown.size}/${view.edges.length} connections shown · ${layoutView.nodes.length}/${view.nodes.length} nodes` : undefined}>
    {selection ? <><strong>{selection.label}</strong><span className="incoming-key">{incoming} incoming</span><span className="outgoing-key">{outgoing} outgoing</span><span>{shown.size}/{view.edges.length} connections shown · {layoutView.nodes.length}/{view.nodes.length} nodes</span></> : <span>{imports ? `${overviewEdges.length} of ${view.edges.length} imports shown` : 'Select a node to inspect its connections'}</span>}
   </div>
   <div className="connection-tools">
    {imports && <select aria-label="Import detail" value={detail} disabled={mode === 'selection' && !!selection} onChange={e => onOptionsChange({ importDetail: e.target.value as 'simplified' | 'all' })}><option value="simplified">Simplified</option><option value="all">All imports</option></select>}
    <div className="connection-mode" role="group" aria-label="Visible connections">
     <button aria-pressed={mode === 'all' || !selection} onClick={() => onOptionsChange({ connectionMode: 'all' })}>All connections</button>
     <button aria-pressed={mode === 'selection' && !!selection} disabled={!selection} onClick={() => onOptionsChange({ connectionMode: 'selection' })}>Selected node</button>
    </div>
    {selection && mode === 'selection' && <select aria-label="Connection direction" value={direction} onChange={e => onOptionsChange({ connectionDirection: e.target.value as ConnectionDirection })}><option value="both">Incoming & outgoing</option><option value="incoming">Incoming only</option><option value="outgoing">Outgoing only</option></select>}
   </div>
   {imports && detail === 'simplified' && mode === 'all' && reduced.length < view.edges.length && <p className="simplification-note">{view.edges.length - reduced.length} direct imports hidden where another dependency path exists. Select All imports for the complete graph.</p>}
  </div>
  <div className="graph-canvas-viewport">
 <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} edgeTypes={edgeTypes} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onNodeClick={(_, n) => onSelect(n.data.entity)} onNodeDoubleClick={(_, n) => onOpen(n.data.entity)} nodesConnectable={false} nodesDraggable={false} deleteKeyCode={null} colorMode="dark" minZoom={0.08} maxZoom={2} proOptions={{ hideAttribution: true }}>
  <Background variant={BackgroundVariant.Dots} color="var(--graph-dots, #272727)" gap={22} size={1}/>
  {arranging && <div className="layout-status" role="status">Arranging nodes and routing connections…</div>}
  {error && <div className="layout-status error-note" role="alert">{error}<button onClick={() => setRevision(v => v + 1)}>Retry layout</button></div>}
  <Controls showInteractive={false}/>
 </ReactFlow></div></div></SchemaInteraction.Provider>
}
interface Props { options: GraphOptions; onOptionsChange: (options: Partial<GraphOptions>) => void; view: GraphView; selected: GraphNode | null; onSelect: (n: GraphNode) => void; onOpen: (n: GraphNode) => void }
export function GraphCanvas(props: Props) { return <ReactFlowProvider><Canvas {...props}/></ReactFlowProvider> }

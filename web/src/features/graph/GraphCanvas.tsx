import { useEffect, useMemo, useState } from 'react'
import { Background, BackgroundVariant, Controls, Handle, MarkerType, MiniMap, Position, ReactFlow, ReactFlowProvider, useEdgesState, useNodesState, useReactFlow, type Edge, type Node, type NodeProps } from '@xyflow/react'
import dagre from '@dagrejs/dagre'
import type { GraphNode, GraphView } from '../../api/types'
import { KindBadge } from '../../components/Icon'

type VisualNode = Node<{ entity: GraphNode; active: boolean; muted: boolean }, 'entity'>
function Entity({ data }: NodeProps<VisualNode>) {
 const n = data.entity
 return <div className={`entity-node ${data.active ? 'active' : ''} ${data.muted ? 'muted' : ''}`}>
  <Handle type="target" position={Position.Left}/>
  <div className="flex items-center gap-2.5"><KindBadge kind={n.kind}/><span className="node-name">{n.label}</span><span className="node-dot"/></div>
  <div className="node-meta">{n.attributes.unresolved ? 'source unavailable' : n.kind === 'externalPackage' ? (n.attributes.standard ? 'standard library' : 'external dependency') : n.kind === 'package' ? n.attributes.relativePath || '.' : n.kind}</div>
  <Handle type="source" position={Position.Right}/>
 </div>
}
const nodeTypes = { entity: Entity }
function Canvas({ view, selected, onSelect, onOpen }: Props) {
 const { fitView, setCenter, getZoom } = useReactFlow()
 const [nodes, setNodes, onNodesChange] = useNodesState<VisualNode>([])
 const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
 const [layoutRevision, setLayoutRevision] = useState(0)
 useEffect(() => {
  const g = new dagre.graphlib.Graph().setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'LR', nodesep: 34, ranksep: 90, marginx: 40, marginy: 40 })
  for (const n of view.nodes) g.setNode(n.id, { width: 236, height: 82 })
  for (const e of view.edges) g.setEdge(e.sourceId, e.targetId)
  dagre.layout(g)
  setNodes(view.nodes.map(n => ({ id: n.id, type: 'entity', data: { entity: n, active: false, muted: false }, position: { x: g.node(n.id).x - 118, y: g.node(n.id).y - 41 } })))
  setLayoutRevision(x => x + 1)
 }, [view, setNodes])
 const connected = useMemo(() => {
  const ids = new Set<string>()
  if (selected) { ids.add(selected.id); for (const e of view.edges) if (e.sourceId === selected.id || e.targetId === selected.id) { ids.add(e.sourceId); ids.add(e.targetId) } }
  return ids
 }, [selected, view])
 useEffect(() => {
  setNodes(ns => ns.map(n => ({ ...n, data: { ...n.data, active: n.id === selected?.id, muted: connected.size > 0 && !connected.has(n.id) } })))
  setEdges(view.edges.map(e => {
   const active = e.sourceId === selected?.id || e.targetId === selected?.id
   return { id: e.id, source: e.sourceId, target: e.targetId, type: 'smoothstep', label: e.derived ? `${e.count} imports` : undefined, animated: active, style: { stroke: active ? '#79afff' : '#3d577c', strokeWidth: active ? 2 : 1.3, opacity: selected && !active ? 0.25 : 1 }, markerEnd: { type: MarkerType.ArrowClosed, color: active ? '#79afff' : '#3d577c' }, labelStyle: { fill: '#afc2de', fontSize: 10 }, labelBgStyle: { fill: '#142238' } }
  }))
 }, [selected, connected, view, setNodes, setEdges])
 useEffect(() => { const id = window.setTimeout(() => void fitView({ padding: 0.2, duration: 300, maxZoom: 1 }), 70); return () => window.clearTimeout(id) }, [layoutRevision, fitView])
 return <ReactFlow nodes={nodes} edges={edges} nodeTypes={nodeTypes} onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onNodeClick={(_, n) => onSelect(n.data.entity)} onNodeDoubleClick={(_, n) => onOpen(n.data.entity)} nodesConnectable={false} colorMode="dark" minZoom={0.08} maxZoom={2} fitView proOptions={{ hideAttribution: true }}>
  <Background variant={BackgroundVariant.Dots} color="#263955" gap={22} size={1}/>
  <Controls showInteractive={false}/>
  <MiniMap style={{ width: 176, height: 116, backgroundColor: '#101d32' }} ariaLabel="Graph minimap" onClick={(_, point) => void setCenter(point.x, point.y, { zoom: getZoom(), duration: 180 })} pannable zoomable nodeBorderRadius={6} nodeColor={n => n.id === selected?.id ? '#79afff' : '#48688f'} maskColor="#08111fcc" maskStrokeColor="#6b9ee5" maskStrokeWidth={2}/>
 </ReactFlow>
}
interface Props { view: GraphView; selected: GraphNode | null; onSelect: (n: GraphNode) => void; onOpen: (n: GraphNode) => void }
export function GraphCanvas(props: Props) { return <ReactFlowProvider><Canvas {...props}/></ReactFlowProvider> }

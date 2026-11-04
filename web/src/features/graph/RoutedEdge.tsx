import { BaseEdge, type Edge, type EdgeProps } from '@xyflow/react'
import type { ElkPoint } from 'elkjs/lib/elk-api'
import { movedPoints, roundedPath } from './layout'

export type RoutedData = { points: ElkPoint[]; description: string; relation: string }
export type RoutedGraphEdge = Edge<RoutedData, 'routed'>
export function RoutedEdge({ id, sourceX, sourceY, targetX, targetY, markerEnd, style, data }: EdgeProps<RoutedGraphEdge>) {
 if (!data) return null
 const path = roundedPath(movedPoints(data.points, { x: sourceX, y: sourceY }, { x: targetX, y: targetY }))
 return <g className="routed-relationship" data-relation={data.relation}>
  <title>{data.description}</title>
  <path className="edge-halo" d={path} fill="none" stroke="var(--background)" strokeWidth={7} pointerEvents="none"/>
  <BaseEdge id={id} path={path} markerEnd={markerEnd} style={style} interactionWidth={18}/>
 </g>
}

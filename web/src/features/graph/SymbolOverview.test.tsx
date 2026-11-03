import { expect, it } from 'vitest'
import type { GraphNode, GraphView } from '../../api/types'
import { groupSymbols } from './SymbolOverview'
it('groups only declared ownership, preserves unrelated and filtered methods', () => {
 const nodes: GraphNode[] = [
  { id: 'type', label: 'Service', kind: 'struct', attributes: { exported: true } },
  { id: 'method', label: 'Create', kind: 'method', attributes: { exported: true } },
  { id: 'func', label: 'NewService', kind: 'function', attributes: { exported: true } },
  { id: 'interface', label: 'Repository', kind: 'interface', attributes: { exported: true } },
  { id: 'other', label: 'Fetch', kind: 'method', attributes: { exported: true } },
 ]
 const view: GraphView = { analysisId: 'a', nodes, edges: [{ id: 'e', kind: 'declares_method', sourceId: 'type', targetId: 'method' }], totalNodes: 4, hiddenNodes: 0 }
 const groups = groupSymbols(view)
 expect(groups.owners.map(n => n.id)).toEqual(['type'])
 expect(groups.methods.get('type')?.map(n => n.id)).toEqual(['method'])
 expect(groups.standalone.map(n => n.id)).toEqual(['func','interface','other'])
 expect(groupSymbols({ ...view, nodes: nodes.slice(1) }).standalone).toHaveLength(4)
 expect(view.nodes).toHaveLength(5)
})

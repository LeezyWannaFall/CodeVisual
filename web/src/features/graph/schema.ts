import type { GraphEdge, GraphNode, GraphView } from '../../api/types'
export const columnNames = (text: string) => text.split(',').map(s => s.trim().replace(/^"|"$/g, ''))
export function primaryColumn(node: GraphNode, name: string) {
 const table = node.attributes.table!
 return table.columns.some(c => c.name === name && /\bPRIMARY\s+KEY\b/i.test(c.constraints ?? '')) || table.constraints.some(c => /\bPRIMARY\s+KEY\b/i.test(c) && columnNames(c.slice(c.indexOf('(') + 1, c.lastIndexOf(')'))).includes(name))
}
export function schemaView(view: GraphView): GraphView {
 const tables = view.nodes.filter(n => n.attributes.table)
 if (!tables.length) return view
 const edges: GraphEdge[] = []
 for (const edge of view.edges.filter(e => e.kind === 'references')) {
  const source = tables.find(n => n.id === edge.sourceId), target = tables.find(n => n.id === edge.targetId)
  if (!source || !target) continue
  const refs = source.attributes.table!.references.filter(r => r.table === target.attributes.table!.name)
  refs.forEach((ref, ri) => {
   const from = columnNames(ref.columns)
   const to = ref.targetColumns ? columnNames(ref.targetColumns) : target.attributes.table!.columns.filter(c => primaryColumn(target,c.name)).map(c => c.name)
   from.forEach((name, i) => edges.push({ ...edge, id: `${edge.id}:${ri}:${i}`, sourceColumn: name, targetColumn: to[i], label: `${source.label}.${name} → ${target.label}.${to[i] ?? '(key not resolved)'}` }))
  })
 }
 return { ...view, nodes: tables.map(n => ({ ...n, attributes: { ...n.attributes, members: view.nodes.filter(i => i.kind === 'index' && view.edges.some(e => e.kind === 'indexes' && e.sourceId === i.id && e.targetId === n.id)).map(i => i.label) } })), edges }
}
export function columnPortY(node: GraphNode, name?: string) {
 const index = node.attributes.table?.columns.findIndex(c => c.name === name) ?? -1
 return index < 0 ? 16 : 46 + index * 26
}

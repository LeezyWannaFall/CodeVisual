import { useMemo } from 'react'
import type { GraphNode, GraphView } from '../../api/types'
import { KindBadge } from '../../components/Icon'

// Ownership comes only from declaration edges. Names do not imply dependencies.
export function groupSymbols(view: GraphView) {
 const owners = view.nodes.filter(n => ['struct', 'type'].includes(n.kind))
 const methods = new Map<string, GraphNode[]>()
 const owned = new Set<string>()
 const byID = new Map(view.nodes.map(n => [n.id, n]))
 for (const edge of view.edges) {
  const method = byID.get(edge.targetId)
  if (edge.kind !== 'declares_method' || !method || !owners.some(n => n.id === edge.sourceId)) continue
  methods.set(edge.sourceId, [...(methods.get(edge.sourceId) ?? []), method]); owned.add(method.id)
 }
 return { owners, methods, standalone: view.nodes.filter(n => !owners.includes(n) && !owned.has(n.id)) }
}
export function SymbolOverview({ view, selected, onSelect, inline = false }: { view: GraphView; selected: GraphNode | null; onSelect: (node: GraphNode) => void; inline?: boolean }) {
 const { owners, methods, standalone } = useMemo(() => groupSymbols(view), [view])
 function row(n: GraphNode) {
  return <button key={n.id} className={`symbol-member ${selected?.id === n.id ? 'selected' : ''}`} onClick={() => onSelect(n)} title={n.attributes.signature ?? n.label}><KindBadge kind={n.kind}/><span>{n.label}</span><small>{n.attributes.exported ? 'public' : 'private'}</small></button>
 }
 return <div className={`symbol-overview ${inline ? 'inline-overview' : ''}`}>
  <div className="symbol-overview-heading"><strong>Package overview</strong><span>Types own their methods · Select a declaration to read its source</span></div>
  <div className="symbol-groups">{owners.map(n => { const children = methods.get(n.id) ?? []; return <section className={`symbol-group ${selected?.id === n.id ? 'selected' : ''}`} key={n.id}>
   <button className="symbol-group-title" onClick={() => onSelect(n)}><KindBadge kind={n.kind}/><span><strong>{n.label}</strong><small>{n.kind}{children.length ? ` · ${children.length} methods` : ''}{n.attributes.members?.length ? ` · ${n.attributes.members.length} members` : ''}</small></span></button>
   {n.attributes.documentation && <p className="symbol-doc">{n.attributes.documentation.split('\n')[0]}</p>}
   {children.length > 0 && <details className="symbol-methods" open={children.some(n => n.id === selected?.id)}><summary>Methods <span>{children.length}</span></summary><div>{children.sort((a,b) => a.label.localeCompare(b.label)).map(row)}</div></details>}

  </section> })}</div>
  {standalone.length > 0 && <details className="symbol-standalone" open={owners.length === 0 || standalone.some(n => n.id === selected?.id)}><summary>Functions & other declarations <span>{standalone.length}</span></summary><div className="symbol-function-list">{standalone.map(row)}</div></details>}
 </div>
}

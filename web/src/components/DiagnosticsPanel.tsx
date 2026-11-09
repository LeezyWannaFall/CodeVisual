import type { Diagnostic, GraphNode } from '../api/types'
import { Icon } from './Icon'

export function groupDiagnostics(items: Diagnostic[]) {
 const groups = new Map<string, Diagnostic[]>()
 for (const item of items) {
  const key = JSON.stringify([item.severity, item.code, item.message])
  groups.set(key, [...(groups.get(key) ?? []), item])
 }
 return [...groups.values()]
}
const limitations = /^(docker_|compose_|http_conditional|http_dynamic|python_|go_version_mismatch)/
export function DiagnosticsPanel({ items, nodes, onSelect, onClose }: { items: Diagnostic[]; nodes: GraphNode[]; onSelect: (node: GraphNode) => void; onClose: () => void }) {
 const groups = groupDiagnostics(items)
 return <section className="diagnostics-panel"><div className="flex items-center justify-between"><h2>Analysis diagnostics <span>{groups.length} groups · {items.length} messages</span></h2><button aria-label="Close diagnostics" onClick={onClose}><Icon name="close"/></button></div>{!items.length ? <p>No analysis errors. cgo and test files are excluded by design.</p> : groups.map((group, i) => {
  const d = group[0]
  return <details key={i} className={'diagnostic-group diagnostic-' + d.severity}><summary><span>{limitations.test(d.code) ? 'Analysis limitation' : d.code === 'package_load' || d.code === 'load_failed' ? 'Source loading' : d.severity}</span><code>{d.code}</code><p>{d.message}</p><b>×{group.length}</b></summary><div>{group.map((item, j) => {
   const node = nodes.find(n => item.nodeId === n.id) ?? nodes.find(n => item.source && n.source?.fileId === item.source.fileId && n.source.startLine === item.source.startLine)
   return <div className="diagnostic-occurrence" key={j}><span>{item.source ? 'Line ' + item.source.startLine : item.nodeId ?? 'Project-wide message'}</span>{node && <button onClick={() => onSelect(node)}>Open {node.label}</button>}</div>
  })}</div></details>
 })}</section>
}

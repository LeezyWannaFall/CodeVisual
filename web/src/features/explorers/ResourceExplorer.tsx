import type { GraphNode, GraphView } from '../../api/types'
import { Icon } from '../../components/Icon'

export function ResourceExplorer({ view, section, selected, onSelect }: { view: GraphView; section: 'api' | 'database'; selected: GraphNode | null; onSelect: (n: GraphNode) => void }) {
 return <div className="resource-explorer">
  <div className="resource-notice"><Icon name="info"/><span>{section === 'api' ? 'Source registrations, not runtime observations. Paths are scoped to each router builder; dynamic constructs appear in diagnostics.' : 'DDL inventory from SQL files, not a live or simulated database. ALTER/DROP are reported in diagnostics; Down migrations are excluded.'}</span></div>
  {section === 'api' ? <div className="routes-list"><div className="route-list-heading"><span>METHOD</span><span>PATH / HANDLER</span><span>MIDDLEWARE</span></div>{view.nodes.map(n => {
   const route = n.attributes.http
   if (!route) return null
   return <button key={n.id} className={`route-row ${selected?.id === n.id ? 'selected' : ''}`} onClick={() => onSelect(n)}>
    <span className={`http-method method-${route.method}`}>{route.method}</span><span className="route-summary"><strong>{route.path}</strong><code>{route.handler}</code><small>{route.scope}</small></span><span className="middleware-count">{route.middleware.length ? `${route.middleware.length} declared` : '—'}<Icon name="arrow" size={14}/></span>
   </button>
  })}</div> : <div className="database-list">{view.nodes.map(n => {
   const table = n.attributes.table, index = n.attributes.index
   return <article key={n.id} className={`schema-card ${selected?.id === n.id ? 'selected' : ''}`}>
    <button className="schema-heading" onClick={() => onSelect(n)}><span className="schema-icon"><Icon name="database" size={20}/></span><span><strong>{n.label}</strong><small>{table ? `${table.columns.length} columns` : `${index?.unique ? 'Unique index' : 'Index'} on ${index?.table}`}</small></span><span className="schema-type">{n.kind}</span><Icon name="arrow" size={14}/></button>
    {table && <><table className="columns-table"><thead><tr><th>Column</th><th>Type</th><th>Constraints</th></tr></thead><tbody>{table.columns.map((c,i) => <tr key={i}><td>{c.name}</td><td>{c.type}</td><td>{c.constraints || '—'}</td></tr>)}</tbody></table>{table.references.length > 0 && <div className="foreign-keys">{table.references.map((r,i) => <p key={i}><Icon name="external" size={12}/>{r.columns} → {r.table} ({r.targetColumns})</p>)}</div>}{table.constraints.length > 0 && <details className="schema-constraints"><summary>{table.constraints.length} table constraints</summary>{table.constraints.map((c,i) => <code key={i}>{c}</code>)}</details>}</>}
    {index && <pre className="index-definition">{index.definition}</pre>}
    <button className="schema-source" onClick={() => onSelect(n)}><Icon name="code" size={13}/>{n.attributes.relativePath}<span>L{n.source?.startLine}</span></button>
   </article>
  })}</div>}
 </div>
}

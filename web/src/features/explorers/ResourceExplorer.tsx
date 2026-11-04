import { SQLCode } from '../../components/SQLCode'
import { GoCode } from '../../components/GoCode'
import type { GraphNode, GraphView } from '../../api/types'
import { Icon } from '../../components/Icon'

export function ResourceExplorer({ view, section, selected, onSelect, onOpenTarget, onFlow }: { onOpenTarget?: (id: string) => void; onFlow?: (n: GraphNode) => void; view: GraphView; section: 'api' | 'database'; selected: GraphNode | null; onSelect: (n: GraphNode) => void }) {
 return <div className="resource-explorer">
  <div className="resource-notice"><Icon name="info"/><span>{section === 'api' ? 'Source registrations, not runtime observations. Paths are scoped to each router builder; dynamic constructs appear in diagnostics.' : 'DDL inventory from SQL files, not a live or simulated database. ALTER/DROP are reported in diagnostics; Down migrations are excluded.'}</span></div>
  {section === 'api' ? <div className="routes-list">{view.nodes.map(n => {
   const route = n.attributes.http
   if (!route) return null
   return <article className={`route-entry ${selected?.id === n.id ? 'selected' : ''}`} key={n.id}><button className={`route-row ${selected?.id === n.id ? 'selected' : ''}`} onClick={() => onSelect(n)}>
    <span className={`http-method method-${route.method}`}>{route.method}</span><span className="route-summary"><strong>{route.path}</strong><span className="route-handler-block"><span className="route-field-label">Handler</span><code><GoCode text={route.handler}/></code></span></span><span className="middleware-count"><span className="route-field-label">Middleware</span><strong>{route.middleware.length}</strong><span>{route.middleware.length ? 'registered' : 'none'}</span></span>
   </button><div className="route-actions"><small className="route-scope" title={route.scope}><Icon name="package" size={11}/>{route.scope.split('/').slice(-2).join('/')}</small>{route.handlerResolution !== 'static' && <span className="resolution-tag">{route.handlerResolution ?? 'unresolved'}</span>}<button disabled={!route.handlerId || !route.handlerSource || ['dynamic','external'].includes(route.handlerResolution ?? '')} onClick={() => route.handlerId && onOpenTarget?.(route.handlerId)}>Open handler<Icon name="code" size={12}/></button>{route.handlerFactoryId && <button onClick={() => onOpenTarget?.(route.handlerFactoryId!)}>Open factory</button>}<button onClick={() => onFlow?.(n)}>Request flow<Icon name="graph" size={12}/></button></div></article>
  })}</div> : <div className="database-list">{view.nodes.map(n => {
   const table = n.attributes.table, index = n.attributes.index
   return <article key={n.id} className={`schema-card ${selected?.id === n.id ? 'selected' : ''}`}>
    <button className="schema-heading" onClick={() => onSelect(n)}><span className="schema-icon"><Icon name="database" size={20}/></span><span><strong>{n.label}</strong><small>{table ? `${table.columns.length} columns` : `${index?.unique ? 'Unique index' : 'Index'} on ${index?.table}`}</small></span><span className="schema-type">{n.kind}</span><Icon name="arrow" size={14}/></button>
    {table && <><table className="columns-table"><thead><tr><th>Column</th><th>Type</th><th>Constraints</th></tr></thead><tbody>{table.columns.map((c,i) => <tr key={i}><td>{c.name}</td><td><SQLCode text={c.type}/></td><td><SQLCode text={c.constraints || '—'}/></td></tr>)}</tbody></table>{table.references.length > 0 && <div className="foreign-keys">{table.references.map((r,i) => <p key={i}><Icon name="external" size={12}/>{r.columns} → {r.table} ({r.targetColumns})</p>)}</div>}{table.constraints.length > 0 && <details className="schema-constraints"><summary>{table.constraints.length} table constraints</summary>{table.constraints.map((c,i) => <code key={i}><SQLCode text={c}/></code>)}</details>}</>}
    {index && <pre className="index-definition"><code><SQLCode text={index.definition}/></code></pre>}
    <button className="schema-source" onClick={() => onSelect(n)}><Icon name="code" size={13}/>{n.attributes.relativePath}<span>L{n.source?.startLine}</span></button>
   </article>
  })}</div>}
 </div>
}

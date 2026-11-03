import { useEffect, useMemo, useState } from 'react'
import { get } from '../../api/client'
import type { GraphNode } from '../../api/types'
import { Icon, KindBadge } from '../../components/Icon'
export function Explorer({ tree, selected, onOpen }: { tree: GraphNode[]; selected: GraphNode | null; onOpen: (n: GraphNode) => void }) {
 const [query, setQuery] = useState('')
 const [results, setResults] = useState<GraphNode[]>([])
 const [total, setTotal] = useState(0)
 const [error, setError] = useState('')
 const [expanded, setExpanded] = useState<Set<string>>(new Set())
 const children = useMemo(() => { const m = new Map<string, GraphNode[]>(); for (const n of tree) { const id = n.parentId ?? ''; m.set(id, [...(m.get(id) ?? []), n]) }; for (const list of m.values()) list.sort((a,b) => a.label.localeCompare(b.label)); return m }, [tree])
 useEffect(() => { const root = tree.find(n => n.kind === 'module'); if (root) setExpanded(new Set([root.id])) }, [tree])
 useEffect(() => {
  const controller = new AbortController(); setError(''); setResults([])
  if (!query.trim()) return () => controller.abort()
  const timer = window.setTimeout(() => { get<{ nodes: GraphNode[]; total: number }>(`/search?q=${encodeURIComponent(query)}`, controller.signal).then(r => { setResults(r.nodes); setTotal(r.total) }).catch(e => { if (!controller.signal.aborted) setError(String(e)) }) }, 200)
  return () => { clearTimeout(timer); controller.abort() }
 }, [query])
 function branch(parent: string, depth = 0): React.ReactNode {
  return (children.get(parent) ?? []).map(n => {
   const hasChildren = (children.get(n.id)?.length ?? 0) > 0
   const open = expanded.has(n.id)
   return <div key={n.id}><div className={`tree-row ${selected?.id === n.id ? 'selected' : ''}`} style={{ paddingLeft: 12 + depth * 13 }}>
    {hasChildren ? <button className={`tree-toggle ${open ? 'open' : ''}`} aria-label={`${open ? 'Collapse' : 'Expand'} ${n.label}`} onClick={() => setExpanded(old => { const next = new Set(old); if (next.has(n.id)) next.delete(n.id); else next.add(n.id); return next })}><Icon name="arrow" size={11}/></button> : <span className="w-3 shrink-0"/>}
    <button className="tree-label" title={n.attributes.importPath ?? n.attributes.relativePath ?? n.label} onClick={() => onOpen(n)}><Icon name={n.kind === 'package' ? 'package' : 'folder'} size={14}/><span className="truncate">{n.kind === 'module' ? 'Project root' : n.label}</span></button>
    {hasChildren && <span className="tree-count">{children.get(n.id)?.length}</span>}
   </div>{hasChildren && open && branch(n.id, depth + 1)}</div>
  })
 }
 return <aside className="explorer"><div className="sidebar-title"><span>EXPLORER</span><span className="subtle">GO MODULE</span></div>
  <div className="search-box"><Icon name="search"/><input aria-label="Search packages and symbols" placeholder="Find a package or symbol…" value={query} onChange={e => setQuery(e.target.value)}/>{query && <button aria-label="Clear search" onClick={() => setQuery('')}><Icon name="close" size={12}/></button>}</div>
  <div className="tree-scroll">{query.trim() ? <><div className="search-caption">{total} results</div>{error && <p className="error-note">{error}</p>}{results.map(n => <button key={n.id} className="search-result" onClick={() => onOpen(n)}><KindBadge kind={n.kind}/><span><strong>{n.label}</strong><small>{n.kind}{n.attributes.receiver ? ` · ${n.attributes.receiver}` : ''}</small></span></button>)}{results.length === 0 && !error && <p className="empty-small">No matching declarations.</p>}{total > results.length && <p className="empty-small">Refine your search to see more.</p>}</> : branch('')}</div>
  <div className="explorer-footer"><span className="status-dot"/>Local analysis<span className="ml-auto subtle">READ ONLY</span></div>
 </aside>
}

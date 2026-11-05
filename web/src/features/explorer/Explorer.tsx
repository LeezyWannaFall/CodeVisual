import { useEffect, useMemo, useState } from 'react'
import { get } from '../../api/client'
import type { GraphNode } from '../../api/types'
import { Icon, KindBadge } from '../../components/Icon'
export function Explorer({ tree, selected, onOpen, query, onQueryChange }: { tree: GraphNode[]; selected: GraphNode | null; onOpen: (n: GraphNode) => void; query: string; onQueryChange: (query: string) => void }) {
 const [loading, setLoading] = useState(false)
 const [results, setResults] = useState<GraphNode[]>([])
 const [total, setTotal] = useState(0)
 const [error, setError] = useState('')
 const [expanded, setExpanded] = useState<Set<string>>(new Set())
 const children = useMemo(() => { const m = new Map<string, GraphNode[]>(); for (const n of tree) { const id = n.parentId ?? ''; m.set(id, [...(m.get(id) ?? []), n]) }; for (const list of m.values()) list.sort((a,b) => a.label.localeCompare(b.label)); return m }, [tree])
 useEffect(() => { const root = tree.find(n => n.kind === 'module'); if (root) setExpanded(new Set([root.id, ...tree.filter(n => n.parentId === root.id).map(n => n.id)])) }, [tree])
 useEffect(() => {
  const controller = new AbortController(); setError(''); setResults([]); setTotal(0); setLoading(!!query.trim())
  if (!query.trim()) return () => controller.abort()
  const timer = window.setTimeout(() => { get<{ nodes: GraphNode[]; total: number }>(`/search?q=${encodeURIComponent(query)}`, controller.signal).then(r => { if (!controller.signal.aborted) { setResults(r.nodes); setTotal(r.total); setLoading(false) } }).catch(e => { if (!controller.signal.aborted) { setError(String(e)); setLoading(false) } }) }, 200)
  return () => { clearTimeout(timer); controller.abort() }
 }, [query])
 useEffect(() => {
  if (!selected) return
  const byId = new Map(tree.map(n => [n.id, n]))
  const ancestors: string[] = []
  let id = selected.packageId || selected.parentId || selected.id
  while (id && !ancestors.includes(id)) { ancestors.push(id); id = byId.get(id)?.parentId ?? '' }
  setExpanded(old => new Set([...old, ...ancestors]))
 }, [selected, tree])
 function openEntry(node: GraphNode) {
  const entries = children.get(node.id) ?? []
  if (entries.length) {
   setExpanded(old => { const next = new Set(old); if (next.has(node.id)) next.delete(node.id); else next.add(node.id); return next })
   const packages = entries.filter(n => n.kind === 'package')
   if (!expanded.has(node.id) && packages.length === 1) onOpen(packages[0])
   return
  }
  onOpen(node)
 }
 function branch(parent: string, depth = 0): React.ReactNode {
  return (children.get(parent) ?? []).map(n => {
   const hasChildren = (children.get(n.id)?.length ?? 0) > 0
   const open = expanded.has(n.id)
   return <div key={n.id} className="tree-branch"><div className={`tree-row ${selected?.id === n.id ? 'selected' : ''}`} style={{ paddingLeft: 6 }}>
    {hasChildren ? <button className={`tree-toggle ${open ? 'open' : ''}`} aria-label={`${open ? 'Collapse' : 'Expand'} ${n.label}`} onClick={() => setExpanded(old => { const next = new Set(old); if (next.has(n.id)) next.delete(n.id); else next.add(n.id); return next })}><Icon name="arrow" size={11}/></button> : <span className="w-3 shrink-0"/>}
    <button className="tree-label" title={n.attributes.importPath ?? n.attributes.relativePath ?? n.label} onClick={() => openEntry(n)} aria-expanded={hasChildren ? open : undefined}><span className={`tree-entry-icon ${n.kind}`}><Icon name={n.kind === 'package' ? 'package' : 'folder'} size={13}/></span><span className="tree-entry-name"><span className="truncate">{n.kind === 'module' ? 'Project root' : n.label}</span></span></button>
    {hasChildren && <span className="tree-count">{children.get(n.id)?.length}</span>}
   </div>{hasChildren && open && <div className="tree-children">{branch(n.id, depth + 1)}</div>}</div>
  })
 }
 const root = tree.find(n => n.kind === 'module')
 const packageCount = tree.filter(n => n.kind === 'package').length
 return <aside className="explorer"><div className="sidebar-title"><span>Explorer</span><span className="explorer-type">Go</span></div>
  <div className="explorer-project"><span className="project-avatar"><Icon name="folder" size={18}/></span><div><strong title={root?.label}>{root?.label.split('/').filter(Boolean).at(-1) ?? 'Your project'}</strong><span>{packageCount} packages <i/> Local workspace</span></div></div>
  <div className="search-box"><Icon name="search"/><input aria-label="Search packages and symbols" placeholder="Find a package or symbol…" value={query} onChange={e => onQueryChange(e.target.value)}/>{query && <button aria-label="Clear search" onClick={() => onQueryChange('')}><Icon name="close" size={12}/></button>}</div>
  <div className="tree-scroll">{query.trim() ? <><div className="search-caption" role="status">{loading ? 'Searching project…' : `${total} results`}</div>{error && <p className="error-note">{error}</p>}{results.map(n => <button key={n.id} className="search-result" aria-current={selected?.id === n.id ? 'true' : undefined} onClick={() => onOpen(n)}><KindBadge kind={n.kind}/><span><strong>{n.label}</strong><small>{n.kind}{n.attributes.receiver ? ` · ${n.attributes.receiver}` : ''}</small></span></button>)}{results.length === 0 && !error && !loading && <p className="empty-small">No matching declarations in this project. Try a package, symbol name, or import path.</p>}{total > results.length && <p className="empty-small">Only the first {results.length} matches are shown. Refine your search to see the rest.</p>}</> : branch('')}</div>

 </aside>
}

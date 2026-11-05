import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { get } from '../../api/client'
import type { GraphNode, GraphView } from '../../api/types'
import { SymbolOverview } from './SymbolOverview'
export function PackagePanel({ node, selected, onSelect, offset, onPage }: { node: GraphNode; selected: GraphNode | null; onSelect: (n: GraphNode) => void; offset: number; onPage: (offset: number) => void }) {
 const panelRef = useRef<HTMLElement>(null)
 const [minimumHeight, setMinimumHeight] = useState(0)
 const [loading, setLoading] = useState(false)
 useLayoutEffect(() => {
  const panel = panelRef.current, document = panel?.parentElement, canvas = panel?.previousElementSibling
  if (panel && document && canvas) setMinimumHeight(Math.max(0, Math.ceil(document.scrollTop + document.clientHeight - canvas.getBoundingClientRect().height)))
 }, [node.id])
 const [view, setView] = useState<GraphView | null>(null)
 const [error, setError] = useState('')
 useEffect(() => {
  const controller = new AbortController(); setLoading(true); setError('')
  get<GraphView>(`/graph?level=symbols&packageId=${encodeURIComponent(node.id)}&offset=${offset}`, controller.signal).then(result => { if (!controller.signal.aborted) { setView(result); setLoading(false) } }).catch(e => { if (!controller.signal.aborted) { setError(e.message); setLoading(false) } })
  return () => controller.abort()
 }, [node.id, offset])
 return <section ref={panelRef} style={{ minHeight: minimumHeight }} aria-busy={loading} className="package-below-map" aria-label="Selected package overview">
  <header><div><span className="eyebrow">SELECTED PACKAGE</span><h2>{node.label}</h2><code>{node.attributes.importPath}</code></div><span>{loading ? '…' : view?.totalNodes ?? '…'} declarations</span></header>
  {error && <p role="alert" className="error-note">{error}</p>}
  {!view && !error && <p role="status" className="empty-small">Loading package…</p>}
  {view && !error && <div className={loading ? 'package-pending' : ''} inert={loading}><SymbolOverview key={`${node.id}:${offset}`} inline view={view} selected={selected} onSelect={onSelect}/>{view.totalNodes === 0 && <p className="empty-small">No declarations in this build.</p>}{view.totalNodes > 200 && <div className="package-paging"><button disabled={offset === 0} onClick={() => onPage(Math.max(0,offset-200))}>Previous</button><span>{offset+1}–{Math.min(offset+200,view.totalNodes)} of {view.totalNodes}</span><button disabled={offset+200 >= view.totalNodes} onClick={() => onPage(offset+200)}>Next</button></div>}</div>}
 </section>
}

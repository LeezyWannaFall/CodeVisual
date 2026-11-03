import { useEffect, useRef, useState } from 'react'
import { get } from '../../api/client'
import type { GraphNode, SourceFile } from '../../api/types'
import { Icon, KindBadge } from '../../components/Icon'
export function Details({ node, onClose, onOpen, collapsed = false, onToggle }: { collapsed?: boolean; onToggle?: () => void; node: GraphNode | null; onClose: () => void; onOpen: (n: GraphNode) => void }) {
 const [wrapLines, setWrapLines] = useState(true)
 const [file, setFile] = useState<SourceFile | null>(null)
 const [error, setError] = useState('')
 const [loading, setLoading] = useState(false)
 const lineRef = useRef<HTMLDivElement>(null)
 useEffect(() => {
  setFile(null); setError(''); setLoading(false)
  if (!node?.source) return
  const controller = new AbortController(); setLoading(true)
  get<SourceFile>(`/files/${node.source.fileId}`, controller.signal).then(setFile).catch(e => { if (!controller.signal.aborted) setError(e.message) }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
  return () => controller.abort()
 }, [node])
 useEffect(() => { lineRef.current?.scrollIntoView?.({ block: 'center' }) }, [file, node, collapsed])
 if (collapsed) return <aside className="details-rail"><button aria-label="Expand inspector panel" title="Expand inspector" onClick={onToggle}><Icon name="panel"/></button><span>INSPECTOR</span></aside>
 if (!node) return <aside className="details empty-details"><div className="detail-title"><span>INSPECTOR</span><button aria-label="Close inspector" title="Collapse inspector" onClick={onClose}><Icon name="panel"/></button></div><div className="inspector-empty"><div className="empty-icon"><Icon name="code" size={25}/></div><h3>Follow the structure.</h3><p>Select a node to explore its declaration, dependencies, and source.</p><span className="hint-key">Double-click a package to explore</span></div></aside>
 return <aside className={`details ${node.source ? 'details-source' : ''}`}><div className="detail-title"><span>INSPECTOR</span><button aria-label="Close inspector" onClick={onClose}><Icon name="panel"/></button></div>
  <div className="node-detail"><div className="flex items-center gap-2"><KindBadge kind={node.kind}/><span className="eyebrow">{node.kind}</span>{node.attributes.exported && <span className="exported-tag">exported</span>}</div><h2>{node.label}</h2><p className="detail-path">{node.attributes.importPath ?? node.attributes.relativePath ?? node.attributes.receiver}</p>
   {(node.kind === 'package' || node.kind === 'directory' || node.kind === 'module') && <button className="open-button" onClick={() => onOpen(node)}>Explore {node.kind}<Icon name="arrow" size={14}/></button>}
   {node.attributes.http && <div className="route-detail"><span className={`http-method method-${node.attributes.http.method}`}>{node.attributes.http.method}</span><code>{node.attributes.http.path}</code><h3>Handler</h3><code>{node.attributes.http.handler}</code><h3>Middleware</h3>{node.attributes.http.middleware.length ? node.attributes.http.middleware.map((m,i) => <code key={i}>{m}</code>) : <p>No statically declared middleware in this scope.</p>}<h3>Registration scope</h3><p>{node.attributes.http.scope}</p></div>}
   {node.attributes.table && <div className="table-detail"><h3>{node.attributes.table.columns.length} columns</h3>{node.attributes.table.references.map((r,i) => <p key={i}>{r.columns} → {r.table} ({r.targetColumns})</p>)}</div>}
   {node.attributes.index && <p className="detail-path">{node.attributes.index.unique ? 'Unique index' : 'Index'} on {node.attributes.index.table}</p>}
   {node.attributes.signature && <pre className="signature">{node.attributes.signature}</pre>}
   {node.attributes.documentation && <p className="documentation">{node.attributes.documentation}</p>}
   {!!node.attributes.members?.length && <details className="members"><summary>{node.attributes.members.length} members</summary>{node.attributes.members.map((m,i) => <pre key={i}>{m}</pre>)}</details>}
  </div>
  {node.source ? <><div className="source-heading"><Icon name="code" size={14}/><span title={file?.relativePath}>{file?.relativePath ?? 'Source code'}</span><span className="ml-auto">L{node.source.startLine}</span><label className="source-wrap-toggle"><input type="checkbox" checked={wrapLines} onChange={e => setWrapLines(e.target.checked)}/>Wrap lines</label></div>{loading && <p className="empty-small">Loading source…</p>}{error && <p role="alert" className="error-note">{error}</p>}{file && <div className={`source-code ${wrapLines ? 'wrap-lines' : ''}`} tabIndex={0} aria-label="Source code">{file.content.split('\n').map((text, i) => <div key={i} ref={i + 1 === node.source?.startLine ? lineRef : undefined} className={`source-line ${i + 1 >= node.source!.startLine && i + 1 <= node.source!.endLine ? 'highlighted' : ''}`}><span className="line-number">{i + 1}</span><code>{text || ' '}</code></div>)}</div>}</> : <div className="detail-footnote"><Icon name="info" size={14}/>{node.attributes.unresolved ? 'This import has no source in the current build. Check analysis diagnostics.' : node.kind === 'externalPackage' ? 'External source is outside this analysis.' : 'Choose a declaration to view source code.'}</div>}
 </aside>
}

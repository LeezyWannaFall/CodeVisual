import type { Container, GraphNode } from '../../api/types'
export function ContainerDetails({ container, nodes, onOpenTarget }: { container: Container; nodes: GraphNode[]; onOpenTarget?: (id: string) => void }) {
 const labels = new Map(nodes.map(n => [n.id, n.attributes.importPath || n.label]))
 return <div className="container-detail">
  <p className="resolution-note">Declared configuration. Images and containers are not built, started, or inspected.</p>
  <dl>{[['Image',container.image],['Build context',container.buildContext],['Dockerfile',container.dockerfile],['Stage',container.stage],['Working directory',container.workdir],['Command',container.command],['Entrypoint',container.entrypoint]].filter(([,value]) => value).map(([label,value]) => <div key={label}><dt>{label}</dt><dd><code>{value}</code></dd></div>)}</dl>
  {container.ports.length > 0 && <><h3>Ports / exposed ports</h3><ul>{container.ports.map((value,i) => <li key={i}><code>{value}</code></li>)}</ul></>}
  {container.networks.length > 0 && <><h3>Networks</h3><p>{container.networks.join(', ')}</p></>}
  {container.environmentKeys.length > 0 && <><h3>Environment keys</h3><p>{container.environmentKeys.join(', ')}</p></>}
  <h3>Declared contents & mounts</h3>
  {container.contents.length === 0 && <p className="resolution-note">No local COPY or mount declarations. Files inside external images are unknown.</p>}
  <ul className="container-contents">{container.contents.map((item,i) => <li key={i}><span className="eyebrow">{item.kind}{item.from ? ` · from ${item.from}` : ''}</span><code>{item.source || '(anonymous volume)'} → {item.target}</code>{item.packageIds.map(id => <button key={id} className="target-link" onClick={() => onOpenTarget?.(id)}>Explore {labels.get(id) ?? 'source package'}</button>)}{item.kind === 'stage' && <small>Build output; follow the source stage to inspect its declarations.</small>}</li>)}</ul>
 </div>
}

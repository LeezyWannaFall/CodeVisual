import { expect, it } from 'vitest'
import type { GraphEdge } from '../../api/types'
import { simplifyImports } from './simplify'
function edges(pairs: string[]): GraphEdge[] { return pairs.map(pair => { const [sourceId, targetId] = pair.split('>'); return { id: pair, kind: 'imports', sourceId, targetId } }) }
function reachable(graph: GraphEdge[], source: string) {
 const seen = new Set<string>(), pending = [source]
 while (pending.length) { const node = pending.pop()!; for (const edge of graph.filter(e => e.sourceId === node)) if (!seen.has(edge.targetId)) { seen.add(edge.targetId); pending.push(edge.targetId) } }
 return [...seen].sort()
}
it('removes redundant shortcuts while preserving branches and original evidence', () => {
 const graph = edges(['cli>handler','cli>service','cli>model','handler>service','handler>model','service>model','other>model'])
 const reduced = simplifyImports(graph)
 expect(reduced.map(e => e.id)).toEqual(['cli>handler','handler>service','service>model','other>model'])
 for (const node of ['cli','handler','service','model','other']) expect(reachable(reduced,node)).toEqual(reachable(graph,node))
 expect(reduced.every(edge => graph.includes(edge))).toBe(true)
 expect(graph).toHaveLength(7)
})
it('preserves cycles and every necessary connection between cyclic components', () => {
 const graph = edges(['a>b','b>a','a>c','b>c','c>d','d>c','a>e','d>e'])
 const reduced = simplifyImports(graph)
 expect(reduced.map(e => e.id)).toEqual(['a>b','b>a','a>c','b>c','c>d','d>c','d>e'])
 for (const node of ['a','b','c','d','e']) expect(reachable(reduced,node)).toEqual(reachable(graph,node))
})
it('leaves non-import relations intact', () => {
 const graph = edges(['a>b','b>c']); graph.push({id:'call',kind:'calls',sourceId:'a',targetId:'c'})
 expect(simplifyImports(graph)).toEqual(graph)
})

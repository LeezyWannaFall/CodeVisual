import { describe, expect, it } from 'vitest'
import ELK from 'elkjs/lib/elk.bundled.js'
import type { GraphView } from '../../api/types'
import { layoutGraph, movedPoints, roundedPath, visibleConnections } from './layout'

// Same dependency shape as Go-URL-Shortener: bootstrap, factory, implementations,
// service, handler, shared model, and config. The fixture contains no project code.
export const shortenerGraph: GraphView = {
 analysisId: 'shortener-shape', totalNodes: 8, hiddenNodes: 0,
 nodes: ['main', 'repository', 'config', 'postgres', 'memory', 'service', 'handler', 'model'].map(id => ({ id, label: id, kind: 'package', attributes: { exported: false, relativePath: id === 'main' ? 'cmd/shortener' : `internal/${id}` } })),
 edges: [
  ['main','handler'],['main','service'],['main','config'],['main','repository'],
  ['repository','config'],['repository','memory'],['repository','postgres'],['repository','service'],
  ['handler','model'],['service','model'],['postgres','model'],['memory','model'],
 ].map(([sourceId, targetId]) => ({ id: `${sourceId}-${targetId}`, kind: 'imports', sourceId, targetId })),
}
const graph = shortenerGraph
function segments(points: {x:number;y:number}[]) {
 return points.slice(1).map((b,i) => ({ a: points[i], b }))
}
function overlap(a: ReturnType<typeof segments>[number], b: ReturnType<typeof segments>[number]) {
 if (a.a.x === a.b.x && b.a.x === b.b.x && a.a.x === b.a.x) return Math.max(Math.min(a.a.y,a.b.y),Math.min(b.a.y,b.b.y)) < Math.min(Math.max(a.a.y,a.b.y),Math.max(b.a.y,b.b.y))
 if (a.a.y === a.b.y && b.a.y === b.b.y && a.a.y === b.a.y) return Math.max(Math.min(a.a.x,a.b.x),Math.min(b.a.x,b.b.x)) < Math.min(Math.max(a.a.x,a.b.x),Math.max(b.a.x,b.b.x))
 return false
}
describe('obstacle-aware architecture layout', () => {
 it('routes the shortener without shared segments or crossing unrelated cards', async () => {
  const layout = await layoutGraph(graph, new ELK())
  expect(layout.nodes).toHaveLength(8)
  expect(layout.edges).toHaveLength(12)
  for (const edge of layout.edges) {
   const start = layout.nodes.find(n => n.entity.id === edge.entity.sourceId)!
   const end = layout.nodes.find(n => n.entity.id === edge.entity.targetId)!
   expect(edge.points[0].x).toBeCloseTo(start.x + start.width)
   expect(edge.points.at(-1)!.x).toBeCloseTo(end.x)
   for (const {a,b} of segments(edge.points)) {
    expect(a.x === b.x || a.y === b.y).toBe(true)
    for (const node of layout.nodes.filter(n => n !== start && n !== end)) {
     const crosses = a.x === b.x
      ? a.x > node.x && a.x < node.x+node.width && Math.max(a.y,b.y)>node.y && Math.min(a.y,b.y)<node.y+node.height
      : a.y > node.y && a.y < node.y+node.height && Math.max(a.x,b.x)>node.x && Math.min(a.x,b.x)<node.x+node.width
     expect(crosses, `${edge.entity.id} crosses ${node.entity.id}`).toBe(false)
    }
   }
  }
  for (let i=0;i<layout.edges.length;i++) for(let j=i+1;j<layout.edges.length;j++) {
   for (const a of segments(layout.edges[i].points)) for(const b of segments(layout.edges[j].points)) expect(overlap(a,b), `${layout.edges[i].entity.id} overlaps ${layout.edges[j].entity.id}`).toBe(false)
  }
  for (const node of layout.nodes) {
   for (const side of ['in','out']) {
    const ports=node.ports.filter(p=>p.side===side)
    expect(new Set(ports.map(p=>p.y)).size).toBe(ports.length)
   }
  }
 })
 it('preserves disconnected nodes and routes cycles', async () => {
  const cycle: GraphView={...graph,nodes:graph.nodes.slice(0,3),edges:[{id:'a',kind:'imports',sourceId:'main',targetId:'repository'},{id:'b',kind:'imports',sourceId:'repository',targetId:'main'}]}
  const layout=await layoutGraph(cycle,new ELK())
  expect(layout.nodes).toHaveLength(3)
  expect(layout.edges).toHaveLength(2)
  for(const edge of layout.edges) expect(roundedPath(edge.points)).not.toMatch(/NaN|Infinity/)
 })
})
it('isolates real incoming/outgoing edges without changing graph data', () => {
 expect(visibleConnections(graph.edges,'config','selection','both').size).toBe(2)
 expect(visibleConnections(graph.edges,'handler','selection','incoming')).toEqual(new Set(['main-handler']))
 expect(visibleConnections(graph.edges,'handler','selection','outgoing')).toEqual(new Set(['handler-model']))
 expect(visibleConnections(graph.edges,'config','all','both').size).toBe(12)
 expect(visibleConnections(graph.edges,undefined,'selection','both').size).toBe(12)
})
it('aligns edge terminals with rendered ports and produces finite rounded paths', () => {
 const points=[{x:0,y:0},{x:40,y:0},{x:40,y:80},{x:100,y:80}]
 const moved=movedPoints(points,{x:10,y:20},{x:130,y:90})
 expect(moved[0]).toEqual({x:10,y:20});expect(moved.at(-1)).toEqual({x:130,y:90})
 expect(moved[1].y).toBe(20);expect(moved[2].y).toBe(90)
 expect(roundedPath(moved)).not.toMatch(/NaN|Infinity/)
 expect(points[0]).toEqual({x:0,y:0})
})

it('routes SQL references and indexes with room for table columns', async () => {
 const schema: GraphView = { analysisId:'schema', totalNodes:3, hiddenNodes:0, nodes:[
  {id:'users',kind:'table',label:'users',attributes:{exported:false,table:{name:'users',columns:[{name:'id',type:'bigint',constraints:'PRIMARY KEY'}],constraints:[],references:[]}}},
  {id:'orders',kind:'table',label:'orders',attributes:{exported:false,table:{name:'orders',columns:Array.from({length:14},(_,i)=>({name:`field_${i}`,type:'text'})),constraints:[],references:[{columns:'field_0',table:'users',targetColumns:'id'}]}}},
  {id:'idx',kind:'index',label:'users_idx',attributes:{exported:false,index:{name:'users_idx',table:'users',unique:true,definition:'CREATE UNIQUE INDEX users_idx ON users(id)'}}},
 ],edges:[{id:'fk',kind:'references',sourceId:'orders',targetId:'users'},{id:'ix',kind:'indexes',sourceId:'idx',targetId:'users'}]}
 const layout=await layoutGraph(schema,new ELK())
 expect(layout.nodes.find(n=>n.entity.id==='orders')!.height).toBeGreaterThanOrEqual(14*26+34)
 expect(layout.edges.map(e=>e.entity.kind).sort()).toEqual(['indexes','references'])
 expect(layout.nodes.find(n=>n.entity.id==='users')!.ports.filter(p=>p.side==='in')).toHaveLength(2)
 for(const edge of layout.edges) expect(roundedPath(edge.points)).not.toMatch(/NaN|Infinity/)
})

import { expect, it } from 'vitest'
import ELK from 'elkjs/lib/elk.bundled.js'
import type { GraphView } from '../../api/types'
import { schemaView } from './schema'
import { layoutGraph } from './layout'
it('connects composite foreign keys to matching columns and embeds indexes in their table', async () => {
 const view: GraphView = { analysisId: 'schema', totalNodes: 3, hiddenNodes: 0, nodes: [
  {id:'parent',kind:'table',label:'parents',attributes:{exported:false,table:{name:'parents',columns:[{name:'a',type:'INT'},{name:'b',type:'INT'}],constraints:['PRIMARY KEY (a, b)'],references:[]}}},
  {id:'child',kind:'table',label:'children',attributes:{exported:false,table:{name:'children',columns:[{name:'x',type:'INT'},{name:'y',type:'INT'}],constraints:[],references:[{columns:'x, y',table:'parents',targetColumns:'a, b'}]}}},
  {id:'index',kind:'index',label:'idx_child',attributes:{exported:false,index:{name:'idx_child',table:'children',unique:false,definition:''}}},
 ],edges:[{id:'fk',kind:'references',sourceId:'child',targetId:'parent'},{id:'ix',kind:'indexes',sourceId:'index',targetId:'child'}] }
 const projected = schemaView(view)
 expect(projected.nodes).toHaveLength(2)
 expect(projected.nodes.find(n=>n.id==='child')?.attributes.members).toEqual(['idx_child'])
 expect(projected.edges.map(e=>[e.sourceColumn,e.targetColumn])).toEqual([['x','a'],['y','b']])
 const layout = await layoutGraph(projected,new ELK())
 for (const node of layout.nodes) expect(node.ports.map(p=>p.y).sort((a,b)=>a-b)).toEqual([46,72])
 expect(view.nodes).toHaveLength(3)
})

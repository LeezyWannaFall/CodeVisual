import { expect, it } from 'vitest'
import fixture from '../../../testdata/contracts/graph.json'
import type { GraphView } from './types'
it('consumes the shared Go JSON contract', () => {
 const view = fixture as GraphView
 expect(view.analysisId).toBe('fixture')
 expect(view.nodes[0].attributes.importPath).toBe('example.test/demo')
 expect(view.nodes[0].kind).toBe('package')
 expect(view.edges).toEqual([])
 expect(view.totalNodes - view.nodes.length).toBe(view.hiddenNodes)
})

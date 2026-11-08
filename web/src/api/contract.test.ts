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

import mixedFixture from '../../../testdata/contracts/mixed.json'
it('consumes Python and container DTOs from the shared contract', () => {
 const view = mixedFixture as GraphView
 expect(view.nodes[0].attributes.language).toBe('python')
 expect(view.nodes[1].kind).toBe('class')
 expect(view.nodes[2].attributes.container?.contents[0].packageIds).toEqual(['python_module'])
 expect(view.nodes[2].attributes.container?.environmentKeys).toEqual(['MODE'])
})

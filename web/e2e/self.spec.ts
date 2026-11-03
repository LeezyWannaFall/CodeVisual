import { expect, test } from '@playwright/test'
test('CodeVisual imports simplify without hiding packages or overlapping the canvas header', async ({ page, request }) => {
 test.skip(!process.env.CODEVISUAL_TEST_SELF, 'Requires analysis of this checkout')
 const graph = await (await request.get('http://127.0.0.1:17335/api/v1/graph?level=packages')).json()
 await page.setViewportSize({ width: 1680, height: 1050 })
 await page.goto('http://127.0.0.1:17335/')
 await page.getByRole('button', { name: 'Close inspector', exact: true }).click()
 await expect(page.locator('.react-flow__node')).toHaveCount(graph.nodes.length)
 await expect(page.getByText('Arranging nodes and routing connections…')).toHaveCount(0)
 const reduced = await page.locator('.react-flow__edge').count()
 expect(reduced).toBeLessThan(graph.edges.length)
 await expect(page.locator('.simplification-note')).toBeVisible()
 const header = (await page.locator('.graph-canvas-header').boundingBox())!
 const viewport = (await page.locator('.react-flow').boundingBox())!
 expect(header.y + header.height).toBeLessThanOrEqual(viewport.y + 1)
 await page.screenshot({ path: test.info().outputPath('self-simplified.png') })
 const cli = page.locator('.react-flow__node').filter({ has: page.locator('.node-name', { hasText: /^cli$/ }) })
 const positions = await page.locator('.react-flow__node').evaluateAll(nodes => nodes.map(n => (n as HTMLElement).style.transform))
 const beforeSelection = (await page.locator('.graph-canvas-viewport').boundingBox())!
 await cli.click()
 await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
 await expect(page.getByText('Arranging nodes and routing connections…')).toHaveCount(0)
 expect((await page.locator('.graph-canvas-viewport').boundingBox())!.y).toBe(beforeSelection.y)
 await expect(page.getByRole('button', { name: 'All connections', exact: true })).toHaveAttribute('aria-pressed', 'true')
 await expect(page.locator('.react-flow__edge')).toHaveCount(reduced)
 expect(await page.locator('.react-flow__node').evaluateAll(nodes => nodes.map(n => (n as HTMLElement).style.transform))).toEqual(positions)
 await page.getByRole('button', { name: 'Selected node', exact: true }).click()
 const incident = graph.edges.filter((e: { sourceId: string; targetId: string }) => e.sourceId === graph.nodes.find((n: {label: string}) => n.label === 'cli').id || e.targetId === graph.nodes.find((n: {label: string}) => n.label === 'cli').id)
 await expect(page.locator('.react-flow__edge')).toHaveCount(incident.length)
 await expect(page.locator('.react-flow__edge.animated')).toHaveCount(incident.length)
 await page.getByRole('button', { name: 'All connections', exact: true }).click()
 await page.getByLabel('Import detail').selectOption('all')
 await expect(page.locator('.react-flow__edge')).toHaveCount(graph.edges.length)
 await expect(page.locator('.react-flow__node')).toHaveCount(graph.nodes.length)
 await page.getByLabel('Import detail').selectOption('simplified')
 await expect(page.locator('.react-flow__edge')).toHaveCount(reduced)
 await page.setViewportSize({ width: 640, height: 900 })
 const mobileHeader = (await page.locator('.graph-canvas-header').boundingBox())!
 const mobileViewport = (await page.locator('.react-flow').boundingBox())!
 expect(mobileHeader.y + mobileHeader.height).toBeLessThanOrEqual(mobileViewport.y + 1)
})

test('changing packages preserves document scroll during and after loading a shorter overview', async ({ page, request }) => {
 test.skip(!process.env.CODEVISUAL_TEST_SELF, 'Requires analysis of this checkout')
 const graph = await (await request.get('http://127.0.0.1:17335/api/v1/graph?level=packages')).json()
 const cliID = graph.nodes.find((n: {label: string}) => n.label === 'cli').id
 await page.goto('http://127.0.0.1:17335/')
 await page.getByRole('button',{name:'Close inspector',exact:true}).click()
 await page.locator('.react-flow__node').filter({has:page.locator('.node-name',{hasText:/^project$/})}).click()
 await expect(page.locator('.package-below-map')).toHaveAttribute('aria-busy','false')
 const scroll = await page.locator('.map-document').evaluate(el => { el.scrollTop = 120; return el.scrollTop })
 expect(scroll).toBeGreaterThan(50)
 let release!: () => void
 const gate = new Promise<void>(resolve => { release = resolve })
 await page.route('**/api/v1/graph?*', async route => {
  const url = new URL(route.request().url())
  if (url.searchParams.get('level') === 'symbols' && url.searchParams.get('packageId') === cliID) await gate
  await route.continue()
 })
 await page.locator('.react-flow__node').filter({has:page.locator('.node-name',{hasText:/^cli$/})}).click()
 await expect(page.locator('.package-below-map')).toHaveAttribute('aria-busy','true')
 expect(await page.locator('.map-document').evaluate(el=>el.scrollTop)).toBe(scroll)
 release()
 await expect(page.locator('.package-below-map')).toHaveAttribute('aria-busy','false')
 await expect(page.locator('.package-below-map h2')).toHaveText('cli')
 expect(await page.locator('.map-document').evaluate(el=>el.scrollTop)).toBe(scroll)
})

import { afterEach, expect, it, vi } from 'vitest'
import { getRoutes } from './client'
afterEach(() => vi.unstubAllGlobals())

it('rejects an incomplete route page instead of looping forever', async () => {
 vi.stubGlobal('fetch', vi.fn(async () => ({ ok: true, json: async () => ({ analysisId: 'a', nodes: [], totalNodes: 201 }) })))
 await expect(getRoutes()).rejects.toThrow('Could not load all routes')
 expect(fetch).toHaveBeenCalledTimes(1)
})

it('does not combine routes from different snapshots', async () => {
 const fetchMock = vi.fn()
  .mockResolvedValueOnce({ ok: true, json: async () => ({ analysisId: 'a', nodes: [{ id: 'route', kind: 'httpRoute' }], totalNodes: 2 }) })
  .mockResolvedValueOnce({ ok: true, json: async () => ({ analysisId: 'b', nodes: [{ id: 'other', kind: 'httpRoute' }], totalNodes: 2 }) })
 vi.stubGlobal('fetch', fetchMock)
 await expect(getRoutes()).rejects.toThrow('Analysis changed')
})

it('passes cancellation to the next page request', async () => {
 const controller = new AbortController()
 vi.stubGlobal('fetch', vi.fn(async (_url, init) => {
  expect(init.signal).toBe(controller.signal)
  controller.abort()
  throw new DOMException('Aborted', 'AbortError')
 }))
 await expect(getRoutes(controller.signal)).rejects.toMatchObject({ name: 'AbortError' })
 expect(fetch).toHaveBeenCalledTimes(1)
})

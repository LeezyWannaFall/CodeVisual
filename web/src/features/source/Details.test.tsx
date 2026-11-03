import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { Details } from './Details'
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
it('explains that changed source requires a new analysis', async () => {
 vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 409, json: async () => ({ error: { code: 'source_changed', message: 'Source changed. Restart codevisual analyze.' } }) })))
 render(<Details node={{ id: 'f', kind: 'function', label: 'F', attributes: { exported: true }, source: { fileId: 'file', startLine: 1, startColumn: 1, endLine: 2, endColumn: 1 } }} onClose={() => {}} onOpen={() => {}}/>)
 expect(await screen.findByRole('alert')).toHaveTextContent('Restart codevisual analyze')
})

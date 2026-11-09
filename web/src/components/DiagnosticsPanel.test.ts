import { expect, it } from 'vitest'
import { groupDiagnostics } from './DiagnosticsPanel'
it('groups repeated causes while retaining every occurrence and source', () => {
 const source = { fileId: 'f', startLine: 2, endLine: 2, startColumn: 1, endColumn: 5 }
 const a = { severity: 'warning' as const, code: 'unresolved_package', message: 'missing', nodeId: 'p' }
 const groups = groupDiagnostics([a, { ...a, source }, { ...a, message: 'other' }])
 expect(groups).toHaveLength(2)
 expect(groups[0]).toHaveLength(2)
 expect(groups[0][1].source).toEqual(source)
})

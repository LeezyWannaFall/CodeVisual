import { expect, it } from 'vitest'
import { highlightPython } from './PythonCode'
it('preserves Python source and highlights multiline strings, comments, and async declarations', () => {
 const source = 'async def run(value: str):\n    """doc\n    text"""\n    # return value\n    return value\n'
 const lines = highlightPython(source)
 expect(lines.map(line => line.map(token => token.text).join('')).join('\n')).toBe(source)
 expect(lines[0].find(t => t.text === 'async')?.kind).toBe('keyword')
 expect(lines[0].find(t => t.text === 'run')?.kind).toBe('function')
 expect(lines[2][0].kind).toBe('string')
 expect(lines[3].find(t => t.text.startsWith('#'))?.kind).toBe('comment')
})

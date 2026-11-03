import { expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { GoCode, highlightGo } from './GoCode'
it('preserves Go source exactly while coloring multiline strings and comments', () => {
 const source = 'package demo\n/* first\nsecond */\nvar tag = `json:"value"\nraw`\nfunc F() string { return "a\\\"b" }\n'
 const lines = highlightGo(source)
 expect(lines.map(line => line.map(t=>t.text).join('')).join('\n')).toBe(source)
 expect(lines[2].find(t=>t.text.includes('second'))?.kind).toBe('comment')
 expect(lines[4].find(t=>t.text.includes('raw'))?.kind).toBe('string')
 expect(lines[5].find(t=>t.text==='func')?.kind).toBe('keyword')
 expect(lines[5].find(t=>t.text==='F')?.kind).toBe('function')
})
it('recognizes tags, rune literals, builtins, Go numbers and Unicode names', () => {
 const source = "type Данные struct { UserID uuid.UUID `json:\"user_id\"` }; var x = 0x1.fp+2; var y = 1_000i; var r = '\\n'; var n = len(x)"
 const tokens = highlightGo(source).flat()
 expect(tokens.find(t=>t.text==='Данные')?.kind).toBe('type')
 expect(tokens.find(t=>t.text==='0x1.fp+2')?.kind).toBe('number')
 expect(tokens.find(t=>t.text==='1_000i')?.kind).toBe('number')
 expect(tokens.find(t=>t.text==='len')?.kind).toBe('builtin')
 expect(tokens.filter(t=>t.kind==='string')).toHaveLength(2)
})
it('renders embedded HTML as inert source text', () => {
 const markup = renderToStaticMarkup(<GoCode text={'var x = `<script>alert(1)</script>`'}/>)
 expect(markup).not.toContain('<script>')
 expect(markup).toContain('&lt;script&gt;')
 expect(markup).toContain('go-string')
})

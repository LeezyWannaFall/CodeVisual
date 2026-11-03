import { expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { highlightSQL, SQLCode } from './SQLCode'
it('preserves PostgreSQL DDL and colors types, comments and dollar quoted functions',()=>{
 const text='-- migration\nCREATE TABLE "Users" (id UUID PRIMARY KEY, note TEXT DEFAULT \'it\'\'s ok\');\nDO $fn$\nBEGIN RAISE NOTICE \'hi\'; END\n$fn$;\nSELECT $1, 2.5e-3;'
 const lines=highlightSQL(text), tokens=lines.flat()
 expect(lines.map(line=>line.map(t=>t.text).join('')).join('\n')).toBe(text)
 expect(tokens.find(t=>t.text==='CREATE')?.kind).toBe('keyword')
 expect(tokens.find(t=>t.text==='UUID')?.kind).toBe('type')
 expect(tokens.find(t=>t.text==='"Users"')?.kind).toBe('namespace')
 expect(tokens.find(t=>t.text==='$1')?.kind).toBe('constant')
 expect(lines[3].every(t=>t.kind==='string')).toBe(true)
})
it('escapes SQL strings containing markup',()=>{
 const output=renderToStaticMarkup(<SQLCode text={"SELECT '<script>test</script>';"}/>)
 expect(output).not.toContain('<script>')
 expect(output).toContain('go-string')
})

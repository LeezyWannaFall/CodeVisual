import { Fragment, useMemo } from 'react'
import { GoTokens, type GoToken } from './GoCode'
const keywords = new Set('False None True and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield match case'.split(' '))
export function highlightPython(source: string): GoToken[][] {
 const pattern = /#[^\n]*|(?:[rRuUbBfF]{0,2})(?:"""[\s\S]*?(?:"""|$)|'''[\s\S]*?(?:'''|$)|"(?:\\[^]|[^"\\\n])*(?:"|$)|'(?:\\[^]|[^'\\\n])*(?:'|$))|\b(?:0[xX][\da-fA-F_]+|\d[\d_]*(?:\.[\d_]*)?(?:[eE][+-]?[\d_]+)?)\b|[\p{L}_][\p{L}\p{N}_]*|\s+|[^]/guy
 const lines: GoToken[][] = [[]]
 let previous = ''
 for (const match of source.matchAll(pattern)) {
  const text = match[0]
  let kind = ''
  if (text.startsWith('#')) kind = 'comment'
  else if (/^[rRuUbBfF]{0,2}['"]/.test(text)) kind = 'string'
  else if (/^\d/.test(text)) kind = 'number'
  else if (['True','False','None'].includes(text)) kind = 'constant'
  else if (keywords.has(text)) kind = 'keyword'
  else if (previous === 'class') kind = 'type'
  else if (previous === 'def' || /^\s*\(/.test(source.slice(match.index! + text.length))) kind = 'function'
  else if (['import','from','as'].includes(previous)) kind = 'namespace'
  text.split('\n').forEach((piece, i) => { if (i) lines.push([]); if (piece) lines[lines.length - 1].push({ text: piece, kind }) })
  if (kind !== 'comment' && text.trim()) previous = text
 }
 return lines
}
export function PythonCode({ text }: { text: string }) {
 const lines = useMemo(() => highlightPython(text), [text])
 return <>{lines.map((tokens,i) => <Fragment key={i}>{i > 0 ? '\n' : ''}<GoTokens tokens={tokens}/></Fragment>)}</>
}

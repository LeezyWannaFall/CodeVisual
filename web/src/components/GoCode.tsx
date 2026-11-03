import { Fragment, useMemo } from 'react'
export interface GoToken { text: string; kind: string }
const keywords = new Set('break default func interface select case defer go map struct chan else goto package switch const fallthrough if range type continue for import return var'.split(' '))
const types = new Set('any bool byte comparable complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr'.split(' '))
const builtins = new Set('append cap clear close complex copy delete imag len make max min new panic print println real recover'.split(' '))
// Lex the complete document so raw strings and block comments retain their color
// across line boundaries. Rendering uses React text nodes, never injected HTML.
export function highlightGo(source: string): GoToken[][] {
 const pattern = /\/\/[^\n]*|\/\*[\s\S]*?(?:\*\/|$)|`[^`]*(?:`|$)|"(?:\\[^\n]|[^"\\\n])*(?:"|$)|'(?:\\[^\n]|[^'\\\n])*(?:'|$)|(?:0[xX][\da-fA-F_]+(?:\.[\da-fA-F_]*)?(?:[pP][+-]?[\d_]+)?|0[bBoO][\d_]+|(?:\d[\d_]*(?:\.[\d_]*)?|\.[\d_]+)(?:[eE][+-]?[\d_]+)?)[i]?|[\p{L}_][\p{L}\p{N}_]*|\s+|[^]/guy
 const lines: GoToken[][] = [[]]
 let previous = ''
 for (const match of source.matchAll(pattern)) {
  const text = match[0]
  let kind = ''
  if (text.startsWith('//') || text.startsWith('/*')) kind = 'comment'
  else if (/^[`"']/.test(text)) kind = 'string'
  else if (/^(?:\d|\.\d)/.test(text)) kind = 'number'
  else if (keywords.has(text)) kind = 'keyword'
  else if (types.has(text)) kind = 'type'
  else if (['true','false','nil','iota'].includes(text)) kind = 'constant'
  else if (builtins.has(text)) kind = 'builtin'
  else if (/^[\p{L}_]/u.test(text)) {
   const rest = source.slice(match.index! + text.length)
   if (previous === 'type' || /^\p{Lu}/u.test(text) && !/^\s*\(/.test(rest)) kind = 'type'
   else if (/^\s*\(/.test(rest) || previous === 'func') kind = 'function'
   else if (previous === 'package' || /^\s*\./.test(rest)) kind = 'namespace'
  } else if (/^[+*/%&|^!<>=:~.-]+$/.test(text)) kind = 'operator'
  const pieces = text.split('\n')
  pieces.forEach((piece,i) => { if (i) lines.push([]); if (piece) lines[lines.length-1].push({ text: piece, kind }) })
  if (kind !== 'comment' && text.trim()) previous = text
 }
 return lines
}
export function GoTokens({ tokens }: { tokens: GoToken[] }) {
 return <>{tokens.map((token,i) => token.kind ? <span key={i} className={`go-token go-${token.kind}`}>{token.text}</span> : <Fragment key={i}>{token.text}</Fragment>)}</>
}
export function GoCode({ text }: { text: string }) {
 const lines = useMemo(() => highlightGo(text), [text])
 return <>{lines.map((tokens,i) => <Fragment key={i}>{i > 0 ? '\n' : ''}<GoTokens tokens={tokens}/></Fragment>)}</>
}

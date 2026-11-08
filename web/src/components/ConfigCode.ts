import type { GoToken } from './GoCode'

// Lightweight display lexer; source remains plain React text, never HTML.
export function highlightConfig(source: string, dockerfile = false): GoToken[][] {
 return source.split('\n').map(line => {
  const tokens: GoToken[] = []
  const pattern = /"(?:\\.|[^"\\])*"|'[^']*'|#[^\n]*|\$\{[^}]*\}|\$\w+|\b(?:true|false|null|\d+(?:\.\d+)?)\b|[\w.-]+(?=\s*:)|^\s*[A-Za-z]+(?=\s)|[^]/g
  for (const match of line.matchAll(pattern)) {
   const text = match[0]
   let kind = ''
   if (text.startsWith('#')) kind = 'comment'
   else if (/^["']/.test(text)) kind = 'string'
   else if (text.startsWith('$')) kind = 'constant'
   else if (/^(true|false|null)$/.test(text)) kind = 'constant'
   else if (/^\d/.test(text)) kind = 'number'
   else if (dockerfile && match.index === 0 && /^\s*(FROM|RUN|COPY|ADD|WORKDIR|CMD|ENTRYPOINT|ENV|ARG|EXPOSE|VOLUME|USER|LABEL|HEALTHCHECK|SHELL|ONBUILD|STOPSIGNAL)\s*$/i.test(text)) kind = 'keyword'
   else if (!dockerfile && /^\s*:/.test(line.slice(match.index! + text.length))) kind = 'keyword'
   tokens.push({ text, kind })
  }
  return tokens
 })
}

import { expect, it } from 'vitest'
import { highlightConfig } from './ConfigCode'
it('preserves configuration source and highlights keys, instructions and comments', () => {
 for (const docker of [false, true]) {
  const source = docker ? 'FROM alpine:3\nCOPY "src/" /app/\n# comment\n' : 'services:\n  app:\n    image: "alpine:3"\n    enabled: true # note\n'
  const lines = highlightConfig(source, docker)
  expect(lines.map(line => line.map(t => t.text).join('')).join('\n')).toBe(source)
  expect(lines[0].some(t => t.kind === 'keyword')).toBe(true)
  expect(lines.flat().some(t => t.kind === 'comment')).toBe(true)
 }
})

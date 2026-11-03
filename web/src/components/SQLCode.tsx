import { Fragment, useMemo } from 'react'
import { GoTokens, type GoToken } from './GoCode'
const keywords = new Set('CREATE ALTER DROP TABLE INDEX UNIQUE PRIMARY KEY FOREIGN REFERENCES CONSTRAINT CHECK DEFAULT NOT NULL IF EXISTS ON DELETE UPDATE CASCADE SET NO ACTION RESTRICT SELECT FROM WHERE INSERT INTO VALUES RETURNING JOIN LEFT RIGHT INNER OUTER AS AND OR IN IS TRUE FALSE ADD COLUMN BEGIN END COMMIT ROLLBACK DO DECLARE LANGUAGE USING WITH INCLUDE ASC DESC NULLS FIRST LAST SCHEMA EXTENSION REPLACE VIEW FUNCTION RETURNS GENERATED ALWAYS BY IDENTITY SERIAL BIGSERIAL'.split(' '))
const types = new Set('INT INTEGER INT2 INT4 INT8 SMALLINT BIGINT TEXT VARCHAR CHAR CHARACTER VARYING BOOLEAN BOOL UUID JSON JSONB NUMERIC DECIMAL REAL DOUBLE PRECISION FLOAT DATE TIME TIMESTAMP TIMESTAMPTZ INTERVAL BYTEA SERIAL BIGSERIAL'.split(' '))
export function highlightSQL(source: string): GoToken[][] {
 const pattern = /--[^\n]*|\/\*[\s\S]*?(?:\*\/|$)|(\$(?:[a-zA-Z_]\w*)?\$)[\s\S]*?\1|(?:[eE])?'(?:''|\\[^]|[^'\\])*(?:'|$)|"(?:""|[^"])*(?:"|$)|\$\d+|(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?|[\p{L}_][\p{L}\p{N}_$]*|\s+|[^]/guy
 const lines: GoToken[][] = [[]]
 for (const match of source.matchAll(pattern)) {
  const text = match[0], upper = text.toUpperCase()
  const kind = text.startsWith('--') || text.startsWith('/*') ? 'comment' : match[1] || /^(?:[eE])?'/.test(text) ? 'string' : text.startsWith('"') ? 'namespace' : types.has(upper) ? 'type' : keywords.has(upper) ? 'keyword' : /^\$\d/.test(text) ? 'constant' : /^(?:\d|\.\d)/.test(text) ? 'number' : /^[\p{L}_]/u.test(text) && /^\s*\(/.test(source.slice(match.index!+text.length)) ? 'function' : /^[+*/%<>=!|:-]+$/.test(text) ? 'operator' : ''
  text.split('\n').forEach((part,i) => { if(i) lines.push([]); if(part) lines[lines.length-1].push({text:part,kind}) })
 }
 return lines
}
export function SQLCode({text}: {text: string}) {
 const lines=useMemo(()=>highlightSQL(text),[text])
 return <>{lines.map((tokens,i)=><Fragment key={i}>{i ? '\n' : ''}<GoTokens tokens={tokens}/></Fragment>)}</>
}

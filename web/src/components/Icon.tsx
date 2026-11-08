import type { CSSProperties } from 'react'
const paths: Record<string, string> = {
 container: 'M3 5h18v14H3zM7 8v8m5-8v8m5-8v8',
 sun: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5',
 moon: 'M20.5 13A8.5 8.5 0 0 1 11 3.5 8.5 8.5 0 1 0 20.5 13',
 panel: 'M3 4h18v16H3zM15 4v16m-5-11 3 3-3 3',
 api: 'M8 5H4v14h4m8-14h4v14h-4M9 12h6m-3-3 3 3-3 3',
 database: 'M3 6c0-5 18-5 18 0s-18 5-18 0zm0 0v12c0 5 18 5 18 0V6M3 12c0 5 18 5 18 0',

 search: 'm21 21-4.8-4.8M19 10.8a8.2 8.2 0 1 1-16.4 0 8.2 8.2 0 0 1 16.4 0',
 folder: 'M3 8V6a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v1M3 8h16a2 2 0 0 1 1.94 2.49l-2 8A2 2 0 0 1 17 20H5a2 2 0 0 1-2-2V8z',
 package: 'm12 3 9 5v9l-9 5-9-5V8zm0 10 9-5M3 8l9 5v9M7.5 5.5l9 5',
 arrow: 'm9 5 7 7-7 7',
 back: 'm14 5-7 7 7 7',
 code: 'm8 7-5 5 5 5m8-10 5 5-5 5m-3-13-2 16',
 graph: 'M9 4h6v5H9zM2 16h6v5H2zm14 0h6v5h-6zM12 9v4M5 16v-3h14v3',
 close: 'm6 6 12 12M6 18 18 6',
 info: 'M12 11v6m0-10v.1M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0',
 check: 'm5 12 4 4L19 6',
 external: 'M14 3h7v7m0-7L10 14M10 3H3v18h18v-7',
}
export function Icon({ name, size = 16, style }: { name: string; size?: number; style?: CSSProperties }) {
 return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={style}><path d={paths[name] ?? paths.code} /></svg>
}
export function KindBadge({ kind }: { kind: string }) { return <span className={`kind-badge kind-${kind}`}>{kind === 'function' ? 'ƒ' : kind === 'method' ? 'm' : kind === 'class' ? 'C' : kind === 'struct' ? 'S' : kind === 'interface' ? 'I' : kind === 'type' ? 'T' : <Icon name={['container','dockerStage','containerNetwork','containerVolume'].includes(kind) ? 'container' : kind === 'table' || kind === 'index' ? 'database' : kind === 'httpRoute' ? 'api' : kind === 'directory' ? 'folder' : 'package'} size={14}/>}</span> }

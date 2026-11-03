// Mirrors internal/graph DTOs; exercised against the shared JSON contract fixture.
export type Kind = 'module' | 'directory' | 'package' | 'externalPackage' | 'function' | 'method' | 'struct' | 'interface' | 'type' | 'httpRoute' | 'table' | 'index' | 'closure' | 'interfaceMethod' | 'dynamicCall' | 'externalCall'
export interface SourceRange { fileId: string; startLine: number; startColumn: number; endLine: number; endColumn: number }
export interface GraphNode {
 id: string; kind: Kind; label: string; parentId?: string; packageId?: string; source?: SourceRange
 attributes: { resolution?: string; external?: boolean; possible?: boolean; importPath?: string; relativePath?: string; exported: boolean; standard?: boolean; unresolved?: boolean; signature?: string; documentation?: string; receiver?: string; members?: string[]; http?: HTTPRoute; table?: SQLTable; index?: SQLIndex }
}
export interface GraphEdge { sourceColumn?: string; targetColumn?: string; label?: string; resolution?: string; invocation?: string; id: string; kind: string; sourceId: string; targetId: string; evidence?: SourceRange; count?: number; derived?: boolean }
export interface GraphView { flow?: { rootId: string; depth: number; truncated: boolean; omittedExternal: number; omittedPossible: number }; analysisId: string; nodes: GraphNode[]; edges: GraphEdge[]; totalNodes: number; hiddenNodes: number }
export interface ProjectInfo {
 schemaVersion: number; analysisId: string; status: 'complete' | 'partial'
 project: { modulePath: string; rootName: string; buildContext: { goos: string; goarch: string; tags: string[]; cgoEnabled: boolean; configurationKnown: boolean } }
 statistics: Partial<Record<Kind, number>>; diagnosticCount: number
}
export interface Diagnostic { severity: 'info' | 'warning' | 'error'; code: string; message: string; nodeId?: string; source?: SourceRange }
export interface SourceFile { id: string; relativePath: string; content: string }
export type Level = 'packages' | 'directories' | 'symbols' | 'http' | 'database' | 'flow'

export interface TargetRef { source?: SourceRange; name: string; nodeId?: string; resolution: string; factoryId?: string }
export interface HTTPRoute { handlerSource?: SourceRange; handlerId?: string; handlerResolution?: string; handlerFactoryId?: string; middlewareTargets?: TargetRef[]; method: string; path: string; handler: string; middleware: string[]; scope: string }
export interface SQLColumn { name: string; type: string; constraints?: string }
export interface SQLTable { name: string; columns: SQLColumn[]; constraints: string[]; references: { columns: string; table: string; targetColumns: string }[] }
export interface SQLIndex { name: string; table: string; unique: boolean; definition: string }

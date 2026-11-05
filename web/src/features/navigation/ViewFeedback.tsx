import { Icon } from '../../components/Icon'
import type { GraphView } from '../../api/types'
import type { NavigationState } from './model'

export function ViewFeedback({ state, view, partial, clearFilters, firstPage, showDiagnostics }: {
 state: NavigationState; view: GraphView; partial: boolean; clearFilters: () => void; firstPage: () => void; showDiagnostics: () => void
}) {
 const filtered = !!(state.query.trim() || state.kind || state.exported || state.routeMethod)
 const outsidePage = state.offset > 0 && state.offset >= view.totalNodes
 const title = outsidePage ? 'This page is outside the current results' : filtered ? 'No matches for these filters' : state.level === 'flow' ? 'Choose an entry point to explore its calls' : state.level === 'http' ? 'No HTTP registrations found' : state.level === 'database' ? 'No SQL declarations found' : 'No nodes in this view'
 const description = outsidePage ? 'The result set has fewer items than this page. Return to the first page.' : filtered ? `No declarations match${state.query.trim() ? ` “${state.query.trim()}”` : ' the selected filters'} in this view. Explorer searches across the whole project.` : state.level === 'flow' ? 'Select a route above, or open a function in Code and choose Request flow in the inspector.' : state.level === 'http' ? 'Only supported static chi registrations are listed. Dynamic registrations may be omitted; check analysis diagnostics.' : state.level === 'database' ? 'Only CREATE TABLE and CREATE INDEX declarations in SQL files are listed. Migrations are never executed.' : state.level === 'symbols' ? 'This package has no declarations in the current build. Tests and inactive build-tag files are excluded.' : 'This scope has no packages for the current build. Choose another directory or use Explorer.'
 return <div className="canvas-message"><Icon name="search" size={26}/><h3>{title}</h3><p>{description}</p><div className="empty-actions">{outsidePage && <button className="open-button" onClick={firstPage}>First page</button>}{filtered && <button className="open-button" onClick={clearFilters}>Clear view filters</button>}{partial && <button className="open-button" onClick={showDiagnostics}>Show analysis diagnostics</button>}</div></div>
}

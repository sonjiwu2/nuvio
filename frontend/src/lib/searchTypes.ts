/**
 * Mirrors the JSON payloads emitted by the Go backend in app_search.go and
 * internal/search/types.go. Wails only generates TS bindings for bound
 * method signatures, not for event payloads, so these are kept in sync by
 * hand — see the field-level json tags on the Go side.
 */

export interface SearchProgress {
  filesScanned: number
  dirsScanned: number
  matchesFound: number
  currentPath: string
}

export interface SearchMatch {
  path: string
  name: string
  size: number
  modTime: string
}

export interface SearchIssue {
  path: string
  error: string
}

export interface SearchResult {
  root: string
  query: string
  filesScanned: number
  dirsScanned: number
  matchCount: number
  truncated: boolean
  issues: SearchIssue[]
  cancelled: boolean
  durationNs: number
}

export interface SearchProgressEvent extends SearchProgress {
  id: string
}

export interface SearchMatchesEvent {
  id: string
  matches: SearchMatch[]
}

export interface SearchCompletedEvent extends SearchResult {
  id: string
}

export interface SearchFailedEvent {
  id: string
  error: string
}

export const SEARCH_EVENTS = {
  started: 'search.started',
  progress: 'search.progress',
  matches: 'search.matches',
  completed: 'search.completed',
  cancelled: 'search.cancelled',
  failed: 'search.failed',
} as const

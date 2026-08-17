/**
 * Mirrors the JSON payloads emitted by the Go backend in app_rules.go,
 * app_organize.go, and internal/rules/types.go. Wails generates a model
 * for AddRule/ListRules' return type, but its `createdAt: any` (it can't
 * map time.Time) is a worse fit than a plain string here — event payloads
 * aren't generated at all — so, consistent with scanTypes.ts and
 * searchTypes.ts, these are kept in sync by hand.
 */

export interface Rule {
  id: string
  extension: string
  destinationFolder: string
  createdAt: string
}

export interface PreviewEntry {
  sourcePath: string
  name: string
  size: number
  destinationPath: string
  ruleId: string
}

export interface PreviewIssue {
  path: string
  error: string
}

export interface PreviewProgress {
  filesScanned: number
  dirsScanned: number
  matchesFound: number
  currentPath: string
}

export interface PreviewResult {
  root: string
  filesScanned: number
  dirsScanned: number
  matchCount: number
  totalSize: number
  truncated: boolean
  issues: PreviewIssue[]
  cancelled: boolean
  durationNs: number
}

export interface PreviewProgressEvent extends PreviewProgress {
  id: string
}

export interface PreviewEntriesEvent {
  id: string
  entries: PreviewEntry[]
}

export interface PreviewCompletedEvent extends PreviewResult {
  id: string
}

export interface PreviewFailedEvent {
  id: string
  error: string
}

export const ORGANIZE_PREVIEW_EVENTS = {
  started: 'organize.preview.started',
  progress: 'organize.preview.progress',
  entries: 'organize.preview.entries',
  completed: 'organize.preview.completed',
  cancelled: 'organize.preview.cancelled',
  failed: 'organize.preview.failed',
} as const

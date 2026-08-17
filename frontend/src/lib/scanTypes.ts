/**
 * Mirrors the JSON payloads emitted by the Go backend in app.go and
 * internal/scanner/types.go. Wails only generates TS bindings for bound
 * method signatures, not for event payloads, so these are kept in sync by
 * hand — see the field-level json tags on the Go side.
 */

export interface ScanProgress {
  filesScanned: number
  dirsScanned: number
  bytesScanned: number
  currentPath: string
}

export interface ScanFileEntry {
  path: string
  name: string
  size: number
  modTime: string
}

export interface ScanFolderEntry {
  path: string
  name: string
  size: number
}

export interface ScanIssue {
  path: string
  error: string
}

export interface ScanResult {
  root: string
  totalSize: number
  totalFiles: number
  totalDirs: number
  topFiles: ScanFileEntry[]
  topFolders: ScanFolderEntry[]
  rootChildren: ScanFolderEntry[]
  issues: ScanIssue[]
  cancelled: boolean
  durationNs: number
}

export interface ScanProgressEvent extends ScanProgress {
  id: string
}

export interface ScanCompletedEvent extends ScanResult {
  id: string
}

export interface ScanFailedEvent {
  id: string
  error: string
}

export const SCAN_EVENTS = {
  started: 'scan.started',
  progress: 'scan.progress',
  completed: 'scan.completed',
  cancelled: 'scan.cancelled',
  failed: 'scan.failed',
} as const

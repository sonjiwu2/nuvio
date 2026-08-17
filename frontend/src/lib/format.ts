const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'

  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), UNITS.length - 1)
  const value = bytes / 1024 ** exponent
  const precision = exponent === 0 || value >= 100 ? 0 : 1

  return `${value.toFixed(precision)} ${UNITS[exponent]}`
}

export function formatCount(count: number): string {
  return new Intl.NumberFormat('en-US').format(count)
}

export function formatDuration(nanoseconds: number): string {
  const seconds = nanoseconds / 1_000_000_000
  if (seconds < 1) return '<1s'
  if (seconds < 60) return `${Math.round(seconds)}s`
  const minutes = Math.floor(seconds / 60)
  const remainder = Math.round(seconds % 60)
  return `${minutes}m ${remainder}s`
}

/** Shortens an absolute path to fit a single line, keeping the start and end visible. */
export function truncatePath(path: string, maxLength = 64): string {
  if (path.length <= maxLength) return path
  const keep = Math.floor((maxLength - 1) / 2)
  return `${path.slice(0, keep)}…${path.slice(path.length - keep)}`
}

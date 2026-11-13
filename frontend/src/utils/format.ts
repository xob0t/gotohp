const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB']

/** Formats a byte count as a short human readable string, e.g. "12.4 MB". */
export function formatBytes(bytes: number, decimals = 1): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), BYTE_UNITS.length - 1)
  return `${parseFloat((bytes / Math.pow(1024, i)).toFixed(decimals))} ${BYTE_UNITS[i]}`
}

/** Formats a duration in seconds as "42s", "3m 10s" or "1h 05m". */
export function formatDuration(totalSeconds: number): string {
  const seconds = Math.max(0, Math.floor(totalSeconds))
  if (seconds < 60) return `${seconds}s`
  const mins = Math.floor(seconds / 60)
  if (mins < 60) return `${mins}m ${seconds % 60}s`
  return `${Math.floor(mins / 60)}h ${String(mins % 60).padStart(2, '0')}m`
}

/** Returns the last path segment for both POSIX and Windows paths. */
export function baseName(path: string): string {
  return path.split(/[\\/]/).pop() || path
}

/** Formats a timestamp as a local wall clock time with milliseconds. */
export function formatClock(timestamp: number): string {
  const d = new Date(timestamp)
  const pad = (n: number, width = 2) => String(n).padStart(width, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`
}

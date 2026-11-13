import { ref, shallowRef, watch } from 'vue'

export type DebugLevel = 'event' | 'info' | 'warn' | 'error'

export interface DebugEntry {
  id: number
  time: number
  level: DebugLevel
  /** Where the entry came from: "backend" events, "frontend" actions or "window" errors. */
  source: string
  name: string
  data?: unknown
}

const STORAGE_KEY = 'gotohp.debug'
const MAX_ENTRIES = 500
const FLUSH_INTERVAL_MS = 150

function readStoredFlag(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

/** Whether debug mode is on. Persisted per machine; the app works the same without storage. */
export const debugEnabled = ref(readStoredFlag())

watch(debugEnabled, (enabled) => {
  try {
    localStorage.setItem(STORAGE_KEY, enabled ? '1' : '0')
  } catch {
    // Storage can be unavailable in some webviews; the toggle then lasts for this session.
  }
  debugLog('info', 'frontend', enabled ? 'debug mode enabled' : 'debug mode disabled')
})

export function toggleDebug() {
  debugEnabled.value = !debugEnabled.value
}

// Entries are kept in a plain ring buffer and published to the UI on a timer, so
// high-frequency events (ThreadStatus) never trigger a re-render per event.
const buffer: DebugEntry[] = []
const counts: Record<string, number> = {}
let nextId = 1
let flushTimer: ReturnType<typeof setTimeout> | null = null

export const debugEntries = shallowRef<DebugEntry[]>([])
export const debugCounts = shallowRef<Record<string, number>>({})
export const debugPaused = ref(false)

function flush() {
  flushTimer = null
  debugEntries.value = buffer.slice()
  debugCounts.value = { ...counts }
}

function scheduleFlush() {
  if (flushTimer === null) {
    flushTimer = setTimeout(flush, FLUSH_INTERVAL_MS)
  }
}

/**
 * Records an entry in the debug log. The log is always collected (it is small and
 * capped) so that turning debug mode on after a problem still shows what led to it.
 */
export function debugLog(level: DebugLevel, source: string, name: string, data?: unknown) {
  counts[name] = (counts[name] ?? 0) + 1
  if (!debugPaused.value) {
    buffer.push({ id: nextId++, time: Date.now(), level, source, name, data })
    if (buffer.length > MAX_ENTRIES) {
      buffer.splice(0, buffer.length - MAX_ENTRIES)
    }
  }
  scheduleFlush()
}

export function clearDebugLog() {
  buffer.length = 0
  for (const key of Object.keys(counts)) delete counts[key]
  flush()
}

/** Serialises anything for display without throwing on cycles or exotic values. */
export function safeStringify(value: unknown, indent?: number): string {
  if (value === undefined) return ''
  const seen = new WeakSet<object>()
  try {
    return JSON.stringify(value, (_key, v) => {
      if (v instanceof Map) return Object.fromEntries(v)
      if (v instanceof Set) return Array.from(v)
      if (v instanceof Error) return { name: v.name, message: v.message, stack: v.stack }
      if (typeof v === 'object' && v !== null) {
        if (seen.has(v)) return '[circular]'
        seen.add(v)
      }
      return v
    }, indent) ?? String(value)
  } catch {
    return String(value)
  }
}

/** Hides credentials embedded in a proxy URL (user:pass@host). */
export function maskProxy(proxy: string): string {
  return proxy.replace(/\/\/[^/@\s]+@/, '//***@')
}

let globalHandlersInstalled = false

/** Routes uncaught errors into the debug log. Safe to call more than once. */
export function installGlobalDebugHandlers() {
  if (globalHandlersInstalled) return
  globalHandlersInstalled = true

  window.addEventListener('error', (e) => {
    debugLog('error', 'window', 'error', {
      message: e.message,
      file: e.filename,
      line: e.lineno,
      column: e.colno,
    })
  })
  window.addEventListener('unhandledrejection', (e) => {
    const reason = e.reason instanceof Error ? e.reason.message : String(e.reason)
    debugLog('error', 'window', 'unhandledrejection', { reason })
  })
}

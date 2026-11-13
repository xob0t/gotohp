<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { Clipboard, System } from '@wailsio/runtime'
import { toast } from 'vue-sonner'
import { Check, Copy, Pause, Play, RefreshCw, Trash2, X } from '@lucide/vue'
import { ConfigManager } from '../../bindings/app/core'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '@/components/ui/sheet'
import DebugKeyValues from './DebugKeyValues.vue'
import { uploadManager } from '@/utils/UploadManager'
import {
  clearDebugLog,
  debugCounts,
  debugEntries,
  debugLog,
  debugPaused,
  maskProxy,
  safeStringify,
  type DebugEntry,
} from '@/utils/debug'
import { formatBytes, formatClock } from '@/utils/format'
import { thumbnailsActive, thumbnailStats } from '@/utils/thumbnails'

type Row = [string, unknown]
type TabId = 'events' | 'state' | 'config' | 'system'

const MAX_VISIBLE_EVENTS = 200
const TABS: { id: TabId, label: string }[] = [
  { id: 'events', label: 'Events' },
  { id: 'state', label: 'State' },
  { id: 'config', label: 'Config' },
  { id: 'system', label: 'System' },
]

const isOpen = defineModel<boolean>('open', { default: false })
const activeTab = ref<TabId>('events')
const { state } = uploadManager

// --- Events -----------------------------------------------------------------
const filter = ref('')
const showThreadStatus = ref(false)
const expandedId = ref<number | null>(null)

const threadStatusCount = computed(() => debugCounts.value.ThreadStatus ?? 0)

const visibleEntries = computed(() => {
  const needle = filter.value.trim().toLowerCase()
  const matches: DebugEntry[] = []
  // Newest first
  for (let i = debugEntries.value.length - 1; i >= 0 && matches.length < MAX_VISIBLE_EVENTS; i--) {
    const entry = debugEntries.value[i]
    if (!showThreadStatus.value && entry.name === 'ThreadStatus' && entry.level !== 'error') continue
    if (needle && !`${entry.name} ${safeStringify(entry.data)}`.toLowerCase().includes(needle)) continue
    matches.push(entry)
  }
  return matches
})

function entryDetails(entry: DebugEntry): string {
  return `source: ${entry.source}\n${safeStringify(entry.data, 2) || '(no payload)'}`
}

const LEVEL_DOT: Record<DebugEntry['level'], string> = {
  event: 'bg-sky-400',
  info: 'bg-muted-foreground',
  warn: 'bg-amber-400',
  error: 'bg-destructive',
}

// --- State ------------------------------------------------------------------
// The byte accounting lives in plain (non-reactive) fields, so it is polled while
// the panel is open instead of being made reactive just for debugging.
const tick = ref(0)
let tickTimer: ReturnType<typeof setInterval> | null = null

function isoOrDash(timestamp: number): string {
  return timestamp > 0 ? new Date(timestamp).toISOString() : ''
}

const uploadRows = computed<Row[]>(() => [
  ['isUploading', state.isUploading],
  ['isStarting', state.isStarting],
  ['thumbnails', `active=${thumbnailsActive.value} generated=${thumbnailStats.generated} failed=${thumbnailStats.failed}`],
  ['queue', state.queue.map(job => `${job.files.length} path(s)${job.albumAutoMode ? ', auto album' : job.albumName ? `, album ${job.albumName}` : ''}`)],
  ['run', `jobs=${state.runJobs} bytes=${state.runBytes} start=${isoOrDash(state.runStartTime) || '-'}`],
  ['files', `${state.uploadedFiles} / ${state.totalFiles}`],
  ['uploadedBytes', `${state.uploadedBytes} (${formatBytes(state.uploadedBytes)})`],
  ['totalBytes', `${state.totalBytes} (${formatBytes(state.totalBytes)})`],
  ['uploadSpeed', `${Math.round(state.uploadSpeed)} B/s (${formatBytes(state.uploadSpeed)}/s)`],
  ['peakSpeed', `${Math.round(state.peakSpeed)} B/s (${formatBytes(state.peakSpeed)}/s)`],
  ['startTime', isoOrDash(state.startTime)],
  ['endTime', isoOrDash(state.endTime)],
  ['isCreatingAlbum', state.isCreatingAlbum],
  ['albumStatus', state.albumStatus],
])

const resultRows = computed<Row[]>(() => [
  ['success', state.results.success.length],
  ['fail', state.results.fail.length],
  ['skipped', state.results.skipped.length],
  ['warnings', state.results.warnings.length],
  ['preflightWarnings', state.warnings.length],
])

const internalRows = computed<Row[]>(() => {
  void tick.value
  const internals = uploadManager.getInternals()
  return [
    ['completedBytes', internals.completedBytes],
    ['totalBytesAdjustment', internals.totalBytesAdjustment],
    ['trackedFiles', internals.trackedFiles],
    ['speedSamples', internals.speedSamples],
    ['lastSpeedUpdate', isoOrDash(internals.lastSpeedUpdate)],
  ]
})

const threadRows = computed<Row[]>(() =>
  Array.from(state.threads.values())
    .sort((a, b) => a.WorkerID - b.WorkerID)
    .map(thread => [
      `worker ${thread.WorkerID}`,
      `${thread.Status} attempt=${thread.Attempt} bytes=${thread.BytesUploaded}/${thread.BytesTotal} ${thread.FilePath || thread.Message}`.trim(),
    ]),
)

// --- Config -----------------------------------------------------------------
const configRows = ref<Row[]>([])
const accountRows = ref<Row[]>([])
const sessionRows = ref<Row[]>([])
const configError = ref('')
const isLoadingConfig = ref(false)

async function loadConfig() {
  isLoadingConfig.value = true
  configError.value = ''
  try {
    const [settings, accounts, albumName, albumAutoMode, excludePattern] = await Promise.all([
      ConfigManager.GetSettings(),
      ConfigManager.GetAccounts(),
      ConfigManager.GetAlbumName(),
      ConfigManager.GetAlbumAutoMode(),
      ConfigManager.GetExcludePattern(),
    ])
    configRows.value = Object.entries(settings)
      .map(([key, value]): Row => [key, key === 'proxy' && typeof value === 'string' ? maskProxy(value) : value])
    accountRows.value = [
      ['selected', accounts.selected],
      ...accounts.accounts.map((account): Row => [
        account.email,
        account.needsTokenBinding ? 'needs token binding key' : 'ready',
      ]),
    ]
    sessionRows.value = [
      ['albumName', albumName],
      ['albumAutoMode', albumAutoMode],
      ['excludePattern', excludePattern],
    ]
  } catch (error) {
    configError.value = error instanceof Error ? error.message : String(error)
    debugLog('error', 'frontend', 'debug config load failed', configError.value)
  } finally {
    isLoadingConfig.value = false
  }
}

// --- System -----------------------------------------------------------------
const environmentRows = ref<Row[]>([])

async function loadEnvironment() {
  try {
    const env = await System.Environment()
    environmentRows.value = [
      ['os', env.OS],
      ['arch', env.Arch],
      ['debugBuild', env.Debug],
      ['osInfo', env.OSInfo],
      ['platformInfo', env.PlatformInfo],
    ]
  } catch (error) {
    environmentRows.value = [['error', error instanceof Error ? error.message : String(error)]]
  }
}

const webviewRows = computed<Row[]>(() => {
  void tick.value
  return [
    ['userAgent', navigator.userAgent],
    ['language', navigator.language],
    ['online', navigator.onLine],
    ['hardwareConcurrency', navigator.hardwareConcurrency],
    ['viewport', `${window.innerWidth} x ${window.innerHeight}`],
    ['devicePixelRatio', window.devicePixelRatio],
    ['frontendMode', import.meta.env.MODE],
  ]
})

const counterRows = computed<Row[]>(() =>
  Object.entries(debugCounts.value).sort((a, b) => b[1] - a[1]),
)

// --- Lifecycle --------------------------------------------------------------
function stopTicking() {
  if (tickTimer) {
    clearInterval(tickTimer)
    tickTimer = null
  }
}

watch(isOpen, (open) => {
  stopTicking()
  if (open) {
    tickTimer = setInterval(() => tick.value++, 500)
    loadConfig()
    loadEnvironment()
  }
}, { immediate: true })

onUnmounted(stopTicking)

// --- Report -----------------------------------------------------------------
const copied = ref(false)

function toObject(rows: Row[]): Record<string, unknown> {
  return Object.fromEntries(rows)
}

async function copyReport() {
  const report = {
    generatedAt: new Date().toISOString(),
    system: { ...toObject(environmentRows.value), ...toObject(webviewRows.value) },
    config: toObject(configRows.value),
    session: toObject(sessionRows.value),
    accounts: toObject(accountRows.value),
    upload: toObject(uploadRows.value),
    results: toObject(resultRows.value),
    accounting: toObject(internalRows.value),
    threads: Array.from(state.threads.values()),
    eventCounts: debugCounts.value,
    events: debugEntries.value.map(entry => ({ ...entry, time: new Date(entry.time).toISOString() })),
  }
  try {
    await Clipboard.SetText(safeStringify(report, 2))
    copied.value = true
    setTimeout(() => copied.value = false, 1200)
  } catch (error) {
    toast.error('Could not copy debug report', {
      description: error instanceof Error ? error.message : String(error),
    })
  }
}
</script>

<template>
  <Sheet v-model:open="isOpen">
    <SheetContent
      side="bottom"
      class="h-[94vh] gap-0"
      style="--wails-draggable: none"
      @open-auto-focus.prevent
    >
      <!-- Header -->
      <div class="flex items-center gap-2 border-b px-4 py-2.5">
        <div class="min-w-0 flex-1">
          <SheetTitle class="text-sm">
            Debug
          </SheetTitle>
          <SheetDescription class="truncate text-[11px]">
            Reports contain emails and file paths.
          </SheetDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          class="cursor-pointer text-xs"
          @click="copyReport"
        >
          <component
            :is="copied ? Check : Copy"
            class="size-3.5"
          />
          {{ copied ? 'Copied' : 'Copy report' }}
        </Button>
        <SheetClose as-child>
          <Button
            variant="ghost"
            size="icon"
            class="size-8 cursor-pointer"
            aria-label="Close debug panel"
          >
            <X class="size-4" />
          </Button>
        </SheetClose>
      </div>

      <!-- Tabs -->
      <div
        class="flex gap-1 border-b px-3 py-1.5"
        role="tablist"
      >
        <button
          v-for="tab in TABS"
          :key="tab.id"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab.id"
          class="cursor-pointer rounded-md px-2.5 py-1 text-xs font-medium transition-colors select-none"
          :class="activeTab === tab.id
            ? 'bg-accent text-accent-foreground'
            : 'text-muted-foreground hover:text-foreground'"
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Events -->
      <div
        v-if="activeTab === 'events'"
        class="flex min-h-0 flex-1 flex-col"
      >
        <div class="flex items-center gap-1.5 px-3 py-2">
          <Input
            v-model="filter"
            placeholder="Filter events"
            spellcheck="false"
            class="h-8 flex-1 font-mono text-xs"
          />
          <Button
            variant="outline"
            size="icon"
            class="size-8 cursor-pointer"
            :aria-label="debugPaused ? 'Resume log' : 'Pause log'"
            :title="debugPaused ? 'Resume log' : 'Pause log'"
            @click="debugPaused = !debugPaused"
          >
            <component
              :is="debugPaused ? Play : Pause"
              class="size-3.5"
            />
          </Button>
          <Button
            variant="outline"
            size="icon"
            class="size-8 cursor-pointer"
            aria-label="Clear log"
            title="Clear log"
            @click="clearDebugLog"
          >
            <Trash2 class="size-3.5" />
          </Button>
        </div>
        <label class="flex cursor-pointer items-center gap-2 px-4 pb-2 text-[11px] text-muted-foreground select-none">
          <input
            v-model="showThreadStatus"
            type="checkbox"
            class="accent-primary"
          >
          Show ThreadStatus events
          <span class="font-mono tabular-nums">({{ threadStatusCount }})</span>
          <span
            v-if="debugPaused"
            class="ml-auto font-medium text-amber-300"
          >paused</span>
        </label>
        <div class="min-h-0 flex-1 overflow-y-auto border-t font-mono text-[11px]">
          <button
            v-for="entry in visibleEntries"
            :key="entry.id"
            type="button"
            class="block w-full cursor-pointer border-b px-3 py-1.5 text-left hover:bg-accent/40"
            @click="expandedId = expandedId === entry.id ? null : entry.id"
          >
            <span class="flex items-center gap-2">
              <span :class="['size-1.5 shrink-0 rounded-full', LEVEL_DOT[entry.level]]" />
              <span class="shrink-0 text-muted-foreground tabular-nums">{{ formatClock(entry.time) }}</span>
              <span
                class="shrink-0 font-semibold"
                :class="entry.level === 'error' && 'text-destructive'"
              >{{ entry.name }}</span>
              <span class="min-w-0 flex-1 truncate text-muted-foreground">{{ safeStringify(entry.data) }}</span>
            </span>
            <pre
              v-if="expandedId === entry.id"
              class="mt-1.5 max-h-56 overflow-auto rounded-md bg-muted/50 p-2 text-[10px] leading-relaxed break-all whitespace-pre-wrap select-text"
            >{{ entryDetails(entry) }}</pre>
          </button>
          <p
            v-if="visibleEntries.length === 0"
            class="px-3 py-6 text-center font-sans text-xs text-muted-foreground"
          >
            No events yet. Drop some files to see the backend events arrive.
          </p>
        </div>
      </div>

      <!-- State -->
      <div
        v-else-if="activeTab === 'state'"
        class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3"
      >
        <DebugKeyValues
          title="Upload"
          :rows="uploadRows"
        />
        <DebugKeyValues
          title="Results"
          :rows="resultRows"
        />
        <DebugKeyValues
          title="Byte accounting"
          :rows="internalRows"
        />
        <DebugKeyValues
          title="Threads"
          :rows="threadRows"
        />
      </div>

      <!-- Config -->
      <div
        v-else-if="activeTab === 'config'"
        class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3"
      >
        <div class="flex items-center justify-between gap-2">
          <p class="px-1 text-[11px] text-muted-foreground">
            Read from the backend. Proxy credentials are masked.
          </p>
          <Button
            variant="outline"
            size="sm"
            class="cursor-pointer text-xs"
            :disabled="isLoadingConfig"
            @click="loadConfig"
          >
            <RefreshCw
              class="size-3.5"
              :class="isLoadingConfig && 'animate-spin'"
            />
            Refresh
          </Button>
        </div>
        <p
          v-if="configError"
          class="rounded-lg border border-destructive/40 bg-destructive/10 px-2.5 py-2 text-xs text-destructive"
        >
          {{ configError }}
        </p>
        <DebugKeyValues
          title="Preferences"
          :rows="configRows"
        />
        <DebugKeyValues
          title="Session"
          :rows="sessionRows"
        />
        <DebugKeyValues
          title="Accounts"
          :rows="accountRows"
        />
      </div>

      <!-- System -->
      <div
        v-else
        class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-3"
      >
        <DebugKeyValues
          title="Environment"
          :rows="environmentRows"
        />
        <DebugKeyValues
          title="Webview"
          :rows="webviewRows"
        />
        <DebugKeyValues
          title="Event counters"
          :rows="counterRows"
        />
      </div>
    </SheetContent>
  </Sheet>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Button from "./components/ui/button/Button.vue"
import { Progress } from "./components/ui/progress"
import ThreadProgress from "./components/ThreadProgress.vue"
import MediaThumb from "./components/MediaThumb.vue"
import { thumbnailsActive } from './utils/thumbnails'
import { uploadManager } from './utils/UploadManager'
import { X, Check, SkipForward, Clock, Zap, Hourglass, FolderPlus, AlertTriangle, ListPlus } from '@lucide/vue'
import { debugEnabled } from './utils/debug'
import { baseName, formatBytes, formatDuration } from './utils/format'

const { state } = uploadManager

// Elapsed time ticker
const elapsedSeconds = ref(state.startTime > 0 ? Math.floor((Date.now() - state.startTime) / 1000) : 0)
let elapsedInterval: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  elapsedInterval = setInterval(() => {
    if (state.startTime > 0) {
      elapsedSeconds.value = Math.floor((Date.now() - state.startTime) / 1000)
    }
  }, 1000)
})

onUnmounted(() => {
  if (elapsedInterval) {
    clearInterval(elapsedInterval)
  }
})

const threadsList = computed(() => {
  return Array.from(state.threads.values())
    .filter(thread => thread.Status !== 'idle')
    .sort((a, b) => a.WorkerID - b.WorkerID)
})

const progressPercent = computed(() => {
  if (state.totalFiles === 0) return 0
  return Math.round((state.uploadedFiles / state.totalFiles) * 100)
})

// Format speed
const speedDisplay = computed(() => {
  if (state.uploadSpeed <= 0) return '--'
  return formatBytes(state.uploadSpeed) + '/s'
})

const elapsedDisplay = computed(() => formatDuration(elapsedSeconds.value))

// Remaining time, estimated from the bytes left and the smoothed upload speed
const etaDisplay = computed(() => {
  const remaining = state.totalBytes - state.uploadedBytes
  if (state.uploadSpeed <= 0 || state.totalBytes <= 0 || remaining <= 0) return '--'
  return formatDuration(remaining / state.uploadSpeed)
})

// Bytes progress display
const bytesDisplay = computed(() => {
  if (state.totalBytes === 0) return ''
  return `${formatBytes(state.uploadedBytes)} of ${formatBytes(state.totalBytes)}`
})

// Album progress display
const albumProgressPercent = computed(() => {
  if (!state.albumStatus || state.albumStatus.TotalItems === 0) return 0
  return Math.round((state.albumStatus.ItemsAdded / state.albumStatus.TotalItems) * 100)
})

const queuedPathCount = computed(() =>
  state.queue.reduce((sum, job) => sum + job.files.length, 0),
)

function queueLabel(job: { albumName: string, albumAutoMode: boolean }): string {
  if (job.albumAutoMode) return 'auto album'
  return job.albumName ? `album: ${job.albumName}` : 'upload only'
}

function warningFiles(paths: string[]): string {
  return paths.map(baseName).filter(Boolean).join(', ')
}
</script>

<template>
  <div class="flex h-full w-full flex-col">
    <!-- Everything above the cancel button scrolls as one region -->
    <div class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-4 pt-4 pb-3 md:grid md:grid-cols-[minmax(20rem,24rem)_1fr] md:grid-rows-[minmax(0,1fr)] md:gap-4 md:overflow-hidden">
      <!-- Summary column -->
      <div class="flex shrink-0 flex-col gap-3 md:min-h-0 md:overflow-y-auto">
        <!-- Overall progress -->
        <div class="shrink-0 rounded-xl border bg-card p-4">
          <div class="flex items-end justify-between gap-3">
            <div>
              <p class="text-3xl leading-none font-semibold tracking-tight tabular-nums">
                {{ progressPercent }}<span class="text-lg text-muted-foreground">%</span>
              </p>
              <p class="mt-1.5 text-xs text-muted-foreground tabular-nums">
                <span class="font-medium text-foreground">{{ state.uploadedFiles }}</span>
                of {{ state.totalFiles }} items processed
              </p>
            </div>
            <p
              v-if="bytesDisplay"
              class="text-right text-xs text-muted-foreground tabular-nums"
            >
              {{ bytesDisplay }}
            </p>
          </div>
          <Progress
            :model-value="progressPercent"
            class="mt-3 h-2"
          />

          <!-- Stats -->
          <div class="mt-3 grid grid-cols-3 gap-2">
            <div class="rounded-lg bg-muted/50 px-2.5 py-2">
              <p class="flex items-center gap-1 text-[10px] tracking-wide text-muted-foreground uppercase">
                <Zap :size="11" /> Speed
              </p>
              <p class="mt-0.5 truncate text-sm font-medium tabular-nums">
                {{ speedDisplay }}
              </p>
            </div>
            <div class="rounded-lg bg-muted/50 px-2.5 py-2">
              <p class="flex items-center gap-1 text-[10px] tracking-wide text-muted-foreground uppercase">
                <Clock :size="11" /> Elapsed
              </p>
              <p class="mt-0.5 truncate text-sm font-medium tabular-nums">
                {{ elapsedDisplay }}
              </p>
            </div>
            <div class="rounded-lg bg-muted/50 px-2.5 py-2">
              <p class="flex items-center gap-1 text-[10px] tracking-wide text-muted-foreground uppercase">
                <Hourglass :size="11" /> Remaining
              </p>
              <p class="mt-0.5 truncate text-sm font-medium tabular-nums">
                {{ etaDisplay }}
              </p>
            </div>
          </div>

          <!-- Technical details (debug mode) -->
          <p
            v-if="debugEnabled"
            class="mt-3 border-t border-dashed pt-2 font-mono text-[10px] leading-relaxed break-words text-muted-foreground tabular-nums"
          >
            bytes={{ state.uploadedBytes }}/{{ state.totalBytes }}
            speed={{ Math.round(state.uploadSpeed) }}B/s
            peak={{ Math.round(state.peakSpeed) }}B/s
            ok={{ state.results.success.length }}
            skip={{ state.results.skipped.length }}
            fail={{ state.results.fail.length }}
            warn={{ state.results.warnings.length }}
          </p>
        </div>

        <!-- Drops waiting behind this upload -->
        <div
          v-if="state.queue.length"
          class="shrink-0 rounded-xl border bg-card p-3"
        >
          <div class="flex items-center gap-2">
            <ListPlus
              :size="14"
              class="shrink-0 text-primary"
            />
            <span class="min-w-0 flex-1 truncate text-sm font-medium">
              Up next
              <span class="font-normal text-muted-foreground tabular-nums">
                · {{ state.queue.length }} {{ state.queue.length === 1 ? 'drop' : 'drops' }},
                {{ queuedPathCount }} {{ queuedPathCount === 1 ? 'item' : 'items' }}
              </span>
            </span>
            <button
              type="button"
              class="shrink-0 cursor-pointer rounded px-1.5 py-0.5 text-xs text-muted-foreground select-none hover:bg-accent hover:text-foreground"
              @click="uploadManager.clearQueue"
            >
              Clear
            </button>
          </div>
          <ol
            v-if="debugEnabled"
            class="mt-2 space-y-0.5 border-t border-dashed pt-2 font-mono text-[10px] text-muted-foreground"
          >
            <li
              v-for="(job, index) in state.queue"
              :key="index"
              class="truncate"
            >
              {{ index + 1 }}. {{ job.files.length }} path(s), {{ queueLabel(job) }}
            </li>
          </ol>
        </div>

        <!-- Album creation progress -->
        <div
          v-if="state.albumStatus && (state.isCreatingAlbum || state.albumStatus.IsComplete)"
          class="shrink-0 rounded-xl border bg-card p-3"
        >
          <div class="mb-2 flex items-center gap-2">
            <FolderPlus
              :size="14"
              class="shrink-0 text-primary"
            />
            <span class="min-w-0 flex-1 truncate text-sm font-medium">
              {{ state.isCreatingAlbum ? 'Adding to album' : 'Added to album' }}
              <span class="font-normal text-muted-foreground">· {{ state.albumStatus.AlbumName }}</span>
            </span>
            <span class="shrink-0 text-xs text-muted-foreground tabular-nums">
              {{ state.albumStatus.ItemsAdded }}/{{ state.albumStatus.TotalItems }}
            </span>
          </div>
          <Progress
            :model-value="albumProgressPercent"
            class="h-1.5"
          />
          <p
            v-if="debugEnabled && state.albumStatus.AlbumKeys?.length"
            class="mt-2 font-mono text-[10px] break-all text-muted-foreground"
          >
            keys: {{ state.albumStatus.AlbumKeys.join(', ') }}
          </p>
        </div>

        <div
          v-if="state.warnings.length"
          class="shrink-0 space-y-1.5"
        >
          <div
            v-for="(warning, index) in state.warnings"
            :key="`${warning.Code}-${index}`"
            class="flex gap-2 rounded-lg border border-amber-500/20 bg-amber-500/10 px-2.5 py-2 text-amber-300"
          >
            <AlertTriangle
              :size="14"
              class="mt-0.5 shrink-0"
            />
            <div class="min-w-0">
              <p class="text-xs leading-snug">
                {{ warning.Message }}
              </p>
              <p class="truncate text-[10px] opacity-75">
                <span
                  v-if="debugEnabled"
                  class="font-mono"
                >[{{ warning.Code }}] </span>{{ warningFiles(warning.Paths) }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- Activity column -->
      <div class="flex shrink-0 flex-col gap-4 md:min-h-0 md:overflow-y-auto md:pr-1">
        <!-- Thread list -->
        <div class="flex shrink-0 flex-col">
          <p class="mb-1.5 px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
            Active threads · {{ threadsList.length }}
          </p>
          <div class="space-y-1.5">
            <ThreadProgress
              v-for="thread in threadsList"
              :key="thread.WorkerID"
              :thread="thread"
            />
            <p
              v-if="threadsList.length === 0"
              class="rounded-lg border border-dashed px-3 py-4 text-center text-xs text-muted-foreground"
            >
              {{ state.isStarting ? 'Starting next upload…' : 'Preparing files…' }}
            </p>
          </div>
        </div>

        <!-- Recently finished files (wide windows, thumbnails on) -->
        <div
          v-if="thumbnailsActive && state.recent.length"
          class="hidden shrink-0 flex-col md:flex"
        >
          <p class="mb-1.5 px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
            Recently finished · {{ state.recent.length }}
          </p>
          <div class="grid grid-cols-[repeat(auto-fill,minmax(5.5rem,1fr))] gap-2">
            <div
              v-for="item in state.recent"
              :key="item.path"
              class="min-w-0"
              :title="item.path"
            >
              <div class="relative">
                <MediaThumb
                  :path="item.path"
                  class="aspect-square w-full rounded-lg"
                />
                <span
                  class="absolute top-1 right-1 flex size-4 items-center justify-center rounded-full text-white"
                  :class="item.status === 'fail' ? 'bg-destructive' : item.status === 'skipped' ? 'bg-zinc-500' : 'bg-emerald-600'"
                >
                  <component
                    :is="item.status === 'fail' ? X : item.status === 'skipped' ? SkipForward : Check"
                    :size="10"
                    :stroke-width="3"
                  />
                </span>
              </div>
              <p class="mt-1 truncate text-[10px] text-muted-foreground">
                {{ baseName(item.path) }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Cancel button - fixed at bottom -->
    <div class="flex shrink-0 justify-center border-t px-4 py-3">
      <Button
        variant="outline"
        size="sm"
        class="w-full cursor-pointer md:max-w-sm text-destructive hover:bg-destructive/10 hover:text-destructive"
        @click="() => uploadManager.cancelUpload()"
      >
        <X :size="14" />
        {{ state.queue.length ? 'Cancel upload and clear queue' : 'Cancel upload' }}
      </Button>
    </div>
  </div>
</template>

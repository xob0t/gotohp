<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Button from "./components/ui/button/Button.vue"
import { Progress } from "./components/ui/progress"
import { ScrollArea } from "./components/ui/scroll-area"
import ThreadProgress from "./components/ThreadProgress.vue"
import { uploadManager } from './utils/UploadManager'
import { X, Clock, Zap, FolderPlus, Settings } from '@lucide/vue'
import { ConfigManager } from '../bindings/app/backend'

const { state } = uploadManager

// Elapsed time ticker
const elapsedSeconds = ref(0)
let elapsedInterval: ReturnType<typeof setInterval> | null = null

const showSettings = ref(false)
const currentThreads = ref(3)

function toggleSettings() {
  showSettings.value = !showSettings.value
}

async function updateThreads() {
  try {
    await ConfigManager.SetUploadThreads(currentThreads.value)
  } catch (err) {
    console.error('Failed to update upload threads:', err)
  }
}

const smoothedRemainingSeconds = ref<number | null>(null)

function updateSmoothedETA() {
  if (state.totalBytes <= 0 || state.uploadedBytes >= state.totalBytes || state.uploadSpeed <= 0) {
    smoothedRemainingSeconds.value = null
    return
  }

  const remainingBytes = state.totalBytes - state.uploadedBytes
  const instantRemainingSeconds = remainingBytes / state.uploadSpeed

  if (smoothedRemainingSeconds.value === null) {
    smoothedRemainingSeconds.value = instantRemainingSeconds
  } else {
    const alpha = 0.1
    const predictedRemaining = Math.max(0, smoothedRemainingSeconds.value - 1)
    smoothedRemainingSeconds.value = alpha * instantRemainingSeconds + (1 - alpha) * predictedRemaining
  }
}

onMounted(async () => {
  try {
    const config = await ConfigManager.GetConfig()
    currentThreads.value = config.uploadThreads || 3
  } catch (err) {
    console.error('Failed to get current threads config:', err)
  }

  elapsedInterval = setInterval(() => {
    if (state.startTime > 0) {
      elapsedSeconds.value = Math.floor((Date.now() - state.startTime) / 1000)
      updateSmoothedETA()
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

// Format bytes to human readable
function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(decimals)) + ' ' + sizes[i]
}

// Format speed
const speedDisplay = computed(() => {
  if (state.uploadSpeed <= 0) return '--'
  return formatBytes(state.uploadSpeed) + '/s'
})

// Format elapsed time
const elapsedDisplay = computed(() => {
  const seconds = elapsedSeconds.value
  if (seconds < 60) return `${seconds}s`
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (mins < 60) return `${mins}m ${secs}s`
  const hours = Math.floor(mins / 60)
  const remainingMins = mins % 60
  return `${hours}h ${remainingMins}m`
})

// Bytes progress display
const bytesDisplay = computed(() => {
  if (state.totalBytes === 0) return ''
  return `${formatBytes(state.uploadedBytes)} / ${formatBytes(state.totalBytes)}`
})

const etaDisplay = computed(() => {
  const seconds = smoothedRemainingSeconds.value
  if (seconds === null || seconds <= 0) {
    return '--'
  }
  
  const roundedSeconds = Math.round(seconds)
  if (roundedSeconds < 60) return `${roundedSeconds}s`
  const mins = Math.floor(roundedSeconds / 60)
  const secs = roundedSeconds % 60
  if (mins < 60) return `${mins}m ${secs}s`
  const hours = Math.floor(roundedSeconds / 60)
  const remainingMins = mins % 60
  return `${hours}h ${remainingMins}m`
})

// Album progress display
const albumProgressPercent = computed(() => {
  if (!state.albumStatus || state.albumStatus.TotalItems === 0) return 0
  return Math.round((state.albumStatus.ItemsAdded / state.albumStatus.TotalItems) * 100)
})
</script>

<template>
  <div class="flex flex-col h-full w-full px-4 pt-6 pb-4">
    <!-- Check if comparing -->
    <template v-if="state.compareProgress">
      <div class="flex-1 flex flex-col items-center justify-center text-center p-6">
        <!-- Spinner or animated icon -->
        <div class="relative w-16 h-16 mb-6">
          <div class="absolute inset-0 rounded-full border-4 border-primary/20 animate-pulse"></div>
          <div class="absolute inset-0 rounded-full border-4 border-t-primary border-r-transparent border-b-transparent border-l-transparent animate-spin"></div>
        </div>

        <h2 class="text-xl font-semibold mb-2">
          Comparing Photos
        </h2>
        
        <p class="text-sm text-muted-foreground max-w-xs mb-4">
          <span v-if="state.compareProgress.Status === 'loading_cache'">
            Loading local comparison cache...
          </span>
          <span v-else-if="state.compareProgress.Status === 'fetching'">
            Scanning Google Photos library...
          </span>
          <span v-else-if="state.compareProgress.Status === 'comparing'">
            Comparing local files with Google Photos...
          </span>
        </p>

        <!-- Count display if fetching -->
        <div v-if="state.compareProgress.Status === 'fetching'" class="text-2xl font-bold tabular-nums text-primary mb-2">
          {{ state.compareProgress.Count }}
          <span class="text-xs font-normal text-muted-foreground block mt-1">items found</span>
        </div>

        <!-- Progress bar or skeleton loader -->
        <div class="w-full max-w-xs bg-muted rounded-full h-1.5 overflow-hidden">
          <div 
            class="bg-primary h-full transition-all duration-300"
            :class="{
              'w-1/3': state.compareProgress.Status === 'loading_cache',
              'w-2/3': state.compareProgress.Status === 'fetching',
              'w-11/12': state.compareProgress.Status === 'comparing'
            }"
          ></div>
        </div>
      </div>
      
      <!-- Cancel button -->
      <Button
        variant="destructive"
        size="sm"
        class="w-full"
        @click="() => uploadManager.cancelUpload()"
      >
        <X
          :size="14"
          class="mr-1"
        />
        Cancel Upload
      </Button>
    </template>

    <!-- Uploading UI (original with comparison stats) -->
    <template v-else>
      <!-- Header with file count -->
      <div class="text-center mb-3 relative">
        <p class="text-2xl font-bold tabular-nums">
          {{ state.uploadedFiles }}<span class="text-muted-foreground font-normal">/</span><span class="text-muted-foreground">{{ state.totalFiles }}</span>
        </p>
        <p class="text-xs text-muted-foreground">
          files uploaded
        </p>
        <button 
          @click="toggleSettings" 
          class="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
          title="Adjust Upload Threads"
        >
          <Settings :size="16" />
        </button>
      </div>

      <!-- Threads adjustment settings panel -->
      <div v-if="showSettings" class="mb-3 p-3 rounded-lg border bg-muted/40 flex flex-col gap-2 animate-in fade-in slide-in-from-top-2 duration-150">
        <div class="flex items-center justify-between">
          <span class="text-xs font-semibold text-foreground">Upload Threads</span>
          <span class="text-xs font-bold text-primary">{{ currentThreads }}</span>
        </div>
        <div class="flex items-center gap-3">
          <input 
            type="range" 
            v-model.number="currentThreads" 
            min="1" 
            max="10" 
            class="flex-1 h-1 bg-muted-foreground/30 rounded-lg appearance-none cursor-pointer accent-primary"
            @input="updateThreads"
          />
          <button 
            @click="showSettings = false"
            class="text-[10px] text-muted-foreground hover:text-foreground underline px-1"
          >
            Close
          </button>
        </div>
      </div>

      <!-- Compare summary at the top of uploading -->
      <div v-if="state.compareResult" class="text-[11px] text-muted-foreground/90 mb-3 px-3 py-2 rounded-lg bg-muted/40 border border-muted-foreground/10 text-center">
        Compared <span class="font-medium text-foreground">{{ state.compareResult.TotalLocal }}</span> files: 
        <span class="font-medium text-foreground">{{ state.compareResult.TotalLocal - state.compareResult.MissingCount }}</span> already in Google Photos, 
        uploading <span class="font-medium text-primary">{{ state.compareResult.MissingCount }}</span> missing.
      </div>

      <!-- Stats row -->
      <div class="flex gap-3 justify-center mb-3 text-xs text-muted-foreground">
        <div class="flex items-center gap-1.5">
          <Zap :size="12" />
          <span class="tabular-nums text-foreground">{{ speedDisplay }}</span>
        </div>
        <span class="text-border">|</span>
        <div class="flex items-center gap-1.5">
          <Clock :size="12" />
          <span class="tabular-nums text-foreground">{{ elapsedDisplay }}</span>
        </div>
        <span class="text-border">|</span>
        <div class="flex items-center gap-1.5">
          <span class="text-muted-foreground">ETA:</span>
          <span class="tabular-nums text-foreground">{{ etaDisplay }}</span>
        </div>
      </div>

      <!-- Main progress bar -->
      <div class="mb-4">
        <Progress
          :model-value="progressPercent"
          class="h-2.5"
        />
        <div class="flex justify-between mt-1.5 text-xs text-muted-foreground">
          <span v-if="bytesDisplay">{{ bytesDisplay }}</span>
          <span v-else>&nbsp;</span>
          <span class="font-medium">{{ progressPercent }}%</span>
        </div>
      </div>

      <!-- Album creation progress -->
      <div
        v-if="state.albumStatus && (state.isCreatingAlbum || state.albumStatus.IsComplete)"
        class="mb-3 p-3 rounded-lg border bg-muted/30"
      >
        <div class="flex items-center gap-2 mb-2">
          <FolderPlus
            :size="14"
            class="text-primary"
          />
          <span class="text-sm font-medium">
            {{ state.isCreatingAlbum ? 'Adding to album...' : 'Added to album' }}
          </span>
        </div>
        <p class="text-xs text-muted-foreground mb-1.5">
          {{ state.albumStatus.AlbumName }}
        </p>
        <Progress
          :model-value="albumProgressPercent"
          class="h-1.5"
        />
        <p class="text-xs text-muted-foreground mt-1">
          {{ state.albumStatus.ItemsAdded }} / {{ state.albumStatus.TotalItems }} items
        </p>
      </div>

      <!-- Thread list - scrollable -->
      <div class="flex-1 min-h-0 mb-3">
        <p class="text-xs text-muted-foreground mb-1.5">
          Active threads ({{ threadsList.length }})
        </p>
        <ScrollArea class="h-[calc(100%-20px)]">
          <div class="space-y-1.5 pr-3">
            <ThreadProgress
              v-for="thread in threadsList"
              :key="thread.WorkerID"
              :thread="thread"
            />
          </div>
        </ScrollArea>
      </div>

      <!-- Cancel button - fixed at bottom -->
      <Button
        variant="destructive"
        size="sm"
        class="w-full"
        @click="() => uploadManager.cancelUpload()"
      >
        <X
          :size="14"
          class="mr-1"
        />
        Cancel Upload
      </Button>
    </template>
  </div>
</template>

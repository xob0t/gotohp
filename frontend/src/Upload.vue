<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Button from "./components/ui/button/Button.vue"
import { Progress } from "./components/ui/progress"
import { ScrollArea } from "./components/ui/scroll-area"
import ThreadProgress from "./components/ThreadProgress.vue"
import { uploadManager } from './utils/UploadManager'
import { X, Clock, Zap } from 'lucide-vue-next'

const { state } = uploadManager

// Elapsed time ticker
const elapsedSeconds = ref(0)
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
</script>

<template>
  <div class="flex flex-col h-full w-full px-4 pt-6 pb-4">
    <!-- Header with file count -->
    <div class="text-center mb-3">
      <p class="text-2xl font-bold tabular-nums">
        {{ state.uploadedFiles }}<span class="text-muted-foreground font-normal">/</span><span class="text-muted-foreground">{{ state.totalFiles }}</span>
      </p>
      <p class="text-xs text-muted-foreground">files uploaded</p>
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
      <X :size="14" class="mr-1" />
      Cancel Upload
    </Button>
  </div>
</template>

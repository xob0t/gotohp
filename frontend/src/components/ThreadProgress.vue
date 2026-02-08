<script setup lang="ts">
import { computed } from 'vue'
import type { ThreadStatus } from '../utils/UploadManager'
import { Progress } from '@/components/ui/progress'

const props = defineProps<{
  thread: ThreadStatus
}>()

const statusColor = computed(() => {
  switch (props.thread.Status) {
    case 'error':
      return 'text-destructive'
    case 'completed':
      return 'text-primary'
    case 'uploading':
      // Show orange/warning color when retrying
      if (props.thread.Attempt > 1) {
        return 'text-orange-500'
      }
      return 'text-blue-500'
    case 'hashing':
      return 'text-cyan-500'
    case 'checking':
      return 'text-yellow-500'
    case 'finalizing':
      return 'text-purple-500'
    default:
      return 'text-muted-foreground'
  }
})

const statusLabel = computed(() => {
  switch (props.thread.Status) {
    case 'hashing':
      return 'Hashing'
    case 'checking':
      return 'Checking'
    case 'uploading':
      if (props.thread.Attempt > 1) {
        return `Retry #${props.thread.Attempt - 1}`
      }
      return 'Uploading'
    case 'finalizing':
      return 'Finalizing'
    case 'completed':
      return 'Done'
    case 'error':
      return 'Error'
    default:
      return props.thread.Status.charAt(0).toUpperCase() + props.thread.Status.slice(1)
  }
})

const uploadPercent = computed(() => {
  if (props.thread.Status !== 'uploading' || !props.thread.BytesTotal) {
    return null
  }
  return Math.round((props.thread.BytesUploaded / props.thread.BytesTotal) * 100)
})

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const fileSizeDisplay = computed(() => {
  if (props.thread.Status !== 'uploading' || !props.thread.BytesTotal) {
    return null
  }
  return `${formatBytes(props.thread.BytesUploaded)} / ${formatBytes(props.thread.BytesTotal)}`
})
</script>

<template>
  <div class="flex flex-col gap-1 px-3 py-2 bg-card border rounded-md text-xs">
    <div class="flex items-center gap-2">
      <span class="text-muted-foreground font-medium w-5">
        #{{ thread.WorkerID + 1 }}
      </span>

      <span :class="['font-medium w-16', statusColor]">
        {{ statusLabel }}
      </span>

      <span
        v-if="thread.FileName"
        class="flex-1 truncate"
        :title="thread.FilePath"
      >
        {{ thread.FileName }}
      </span>
      <span
        v-else
        class="flex-1 truncate text-muted-foreground"
      >
        {{ thread.Message }}
      </span>

      <span
        v-if="fileSizeDisplay"
        class="text-muted-foreground tabular-nums text-[10px]"
      >
        {{ fileSizeDisplay }}
      </span>

      <span
        v-if="uploadPercent !== null"
        class="text-muted-foreground tabular-nums w-8 text-right"
      >
        {{ uploadPercent }}%
      </span>
    </div>

    <Progress
      v-if="uploadPercent !== null"
      :model-value="uploadPercent"
      class="h-1"
    />
  </div>
</template>

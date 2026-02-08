<script setup lang="ts">
import { computed } from 'vue'
import type { ThreadStatus } from '../utils/UploadManager'
import { Progress } from '@/components/ui/progress'

const props = defineProps<{
  thread: ThreadStatus
}>()

const statusConfig = computed(() => {
  switch (props.thread.Status) {
    case 'error':
      return { color: 'text-destructive', bg: 'bg-destructive/10', label: 'Error' }
    case 'completed':
      return { color: 'text-primary', bg: 'bg-primary/10', label: 'Done' }
    case 'uploading':
      if (props.thread.Attempt > 1) {
        return { color: 'text-orange-500', bg: 'bg-orange-500/10', label: `Retry ${props.thread.Attempt - 1}` }
      }
      return { color: 'text-blue-500', bg: 'bg-blue-500/10', label: 'Uploading' }
    case 'hashing':
      return { color: 'text-cyan-500', bg: 'bg-cyan-500/10', label: 'Hashing' }
    case 'checking':
      return { color: 'text-yellow-500', bg: 'bg-yellow-500/10', label: 'Checking' }
    case 'finalizing':
      return { color: 'text-purple-500', bg: 'bg-purple-500/10', label: 'Finalizing' }
    default:
      return { color: 'text-muted-foreground', bg: 'bg-muted', label: props.thread.Status }
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
  return `${formatBytes(props.thread.BytesUploaded)}/${formatBytes(props.thread.BytesTotal)}`
})
</script>

<template>
  <div class="px-2.5 py-2 bg-card border rounded-md">
    <!-- Top row: status badge + filename -->
    <div class="flex items-center gap-2 mb-1">
      <span 
        :class="[
          'text-[10px] font-medium px-1.5 py-0.5 rounded',
          statusConfig.color,
          statusConfig.bg
        ]"
      >
        {{ statusConfig.label }}
      </span>
      <span
        v-if="thread.FileName"
        class="flex-1 text-xs truncate"
        :title="thread.FilePath"
      >
        {{ thread.FileName }}
      </span>
      <span
        v-else
        class="flex-1 text-xs truncate text-muted-foreground"
      >
        {{ thread.Message }}
      </span>
    </div>

    <!-- Progress bar (only when uploading) -->
    <template v-if="uploadPercent !== null">
      <Progress
        :model-value="uploadPercent"
        class="h-1 mb-1"
      />
      <div class="flex justify-between text-[10px] text-muted-foreground tabular-nums">
        <span>{{ fileSizeDisplay }}</span>
        <span>{{ uploadPercent }}%</span>
      </div>
    </template>
  </div>
</template>

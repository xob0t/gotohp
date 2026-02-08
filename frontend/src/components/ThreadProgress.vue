<script setup lang="ts">
import { computed } from 'vue'
import type { ThreadStatus } from '../utils/UploadManager'

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
</script>

<template>
  <div class="flex items-center gap-2 px-3 py-2 bg-card border rounded-md text-xs">
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
  </div>
</template>

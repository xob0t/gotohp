<script setup lang="ts">
import { computed } from 'vue'
import type { ThreadStatus } from '../utils/UploadManager'

const props = defineProps<{
  thread: ThreadStatus
}>()

// Compute status color
const statusColor = computed(() => {
  switch (props.thread.Status) {
    case 'error':
      return 'text-red-500'
    case 'completed':
      return 'text-green-500'
    case 'uploading':
      return 'text-blue-600'
    case 'hashing':
      return 'text-blue-400'
    case 'checking':
      return 'text-yellow-500'
    case 'finalizing':
      return 'text-purple-500'
    default:
      return 'text-muted-foreground'
  }
})

// Compute status label
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
      return 'Completed'
    case 'error':
      return 'Error'
    default:
      return props.thread.Status.charAt(0).toUpperCase() + props.thread.Status.slice(1)
  }
})
</script>

<template>
  <div class="flex items-center gap-1.5 px-2 py-1 border rounded bg-card text-[10px]">
    <!-- Thread ID -->
    <span class="font-semibold text-muted-foreground">
      #{{ thread.WorkerID + 1 }}
    </span>

    <!-- Status label -->
    <span :class="['font-medium min-w-[3.5rem]', statusColor]">
      {{ statusLabel }}
    </span>

    <!-- File name -->
    <div class="flex-1 min-w-0">
      <span v-if="thread.FileName" class="truncate block" :title="thread.FilePath">
        {{ thread.FileName }}
      </span>
      <span v-else class="text-muted-foreground truncate block">
        {{ thread.Message }}
      </span>
    </div>
  </div>
</template>

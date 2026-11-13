<script setup lang="ts">
import { computed } from 'vue'
import type { ThreadStatus } from '../utils/UploadManager'
import { Progress } from '@/components/ui/progress'
import { debugEnabled } from '@/utils/debug'
import { formatBytes } from '@/utils/format'
import { thumbnailsActive } from '@/utils/thumbnails'
import MediaThumb from './MediaThumb.vue'

const props = defineProps<{
  thread: ThreadStatus
}>()

const statusConfig = computed(() => {
  switch (props.thread.Status) {
    case 'error':
      return { color: 'text-destructive', dot: 'bg-destructive', label: 'Error', active: false }
    case 'completed':
      return { color: 'text-primary', dot: 'bg-primary', label: 'Done', active: false }
    case 'skipped':
      return { color: 'text-amber-300', dot: 'bg-amber-400', label: 'Skipped', active: false }
    case 'uploading':
      if (props.thread.Attempt > 1) {
        return { color: 'text-orange-400', dot: 'bg-orange-400', label: `Retry ${props.thread.Attempt - 1}`, active: true }
      }
      return { color: 'text-sky-400', dot: 'bg-sky-400', label: 'Uploading', active: true }
    case 'hashing':
      return { color: 'text-cyan-400', dot: 'bg-cyan-400', label: 'Hashing', active: true }
    case 'checking':
      return { color: 'text-yellow-400', dot: 'bg-yellow-400', label: 'Checking', active: true }
    case 'finalizing':
      return { color: 'text-violet-400', dot: 'bg-violet-400', label: 'Finalizing', active: true }
    default:
      return { color: 'text-muted-foreground', dot: 'bg-muted-foreground', label: props.thread.Status, active: false }
  }
})

const uploadPercent = computed(() => {
  if (props.thread.Status !== 'uploading' || !props.thread.BytesTotal) {
    return null
  }
  return Math.round((props.thread.BytesUploaded / props.thread.BytesTotal) * 100)
})

const fileSizeDisplay = computed(() => {
  if (props.thread.Status !== 'uploading' || !props.thread.BytesTotal) {
    return null
  }
  return `${formatBytes(props.thread.BytesUploaded)} of ${formatBytes(props.thread.BytesTotal)}`
})

const statusMessage = computed(() => props.thread.Message.replace(/^Error:\s*/, ''))
</script>

<template>
  <div class="flex gap-3 rounded-lg border bg-card px-3 py-2">
    <MediaThumb
      v-if="thumbnailsActive && thread.FilePath"
      :path="thread.FilePath"
      class="size-12 self-start"
    />
    <div class="min-w-0 flex-1 self-center">
      <!-- Top row: status dot + filename + status -->
      <div class="flex items-center gap-2">
        <span class="relative flex size-2 shrink-0">
          <span
            v-if="statusConfig.active"
            :class="['absolute inline-flex size-full animate-ping rounded-full opacity-60', statusConfig.dot]"
          />
          <span :class="['relative inline-flex size-2 rounded-full', statusConfig.dot]" />
        </span>
        <span
          v-if="thread.FileName"
          class="min-w-0 flex-1 truncate text-xs font-medium"
          :title="thread.FilePath"
        >
          {{ thread.FileName }}
        </span>
        <span
          v-else
          class="min-w-0 flex-1 truncate text-xs text-muted-foreground"
        >
          {{ thread.Message }}
        </span>
        <span :class="['shrink-0 text-[11px] font-medium tabular-nums', statusConfig.color]">
          {{ uploadPercent !== null && thread.Attempt <= 1 ? `${uploadPercent}%` : statusConfig.label }}
        </span>
      </div>

      <!-- Progress bar (only when uploading) -->
      <template v-if="uploadPercent !== null">
        <Progress
          :model-value="uploadPercent"
          class="mt-1.5 h-1"
        />
        <div class="mt-1 flex justify-between text-[10px] text-muted-foreground tabular-nums">
          <span>{{ fileSizeDisplay }}</span>
          <span v-if="thread.Attempt > 1">{{ uploadPercent }}%</span>
        </div>
      </template>
      <p
        v-else-if="thread.Status === 'error' || thread.Status === 'skipped'"
        :class="[
          'mt-1 text-[11px] leading-snug break-words',
          thread.Status === 'error' ? 'text-destructive' : 'text-muted-foreground'
        ]"
      >
        {{ statusMessage }}
      </p>

      <!-- Technical details (debug mode) -->
      <div
        v-if="debugEnabled"
        class="mt-1.5 border-t border-dashed pt-1.5 font-mono text-[10px] leading-relaxed text-muted-foreground"
      >
        <p class="tabular-nums">
          worker={{ thread.WorkerID }} status={{ thread.Status }} attempt={{ thread.Attempt }}
          bytes={{ thread.BytesUploaded }}/{{ thread.BytesTotal }}
        </p>
        <p
          v-if="thread.FilePath"
          class="break-all"
        >
          {{ thread.FilePath }}
        </p>
        <p
          v-if="thread.Message && thread.FileName && thread.Status !== 'error' && thread.Status !== 'skipped'"
          class="break-words"
        >
          msg: {{ thread.Message }}
        </p>
      </div>
    </div>
  </div>
</template>

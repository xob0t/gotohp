<script setup lang="ts">
import { computed } from 'vue'
import Button from "./components/ui/button/Button.vue"
import ThreadProgress from "./components/ThreadProgress.vue"
import { uploadManager } from './utils/UploadManager'

// Access the reactive state from the upload manager
const { state } = uploadManager

// Convert threads Map to array and filter out idle threads
const threadsList = computed(() => {
  return Array.from(state.threads.values())
    .filter(thread => thread.Status !== 'idle')
    .sort((a, b) => a.WorkerID - b.WorkerID)
})
</script>

<template>
  <div class="flex flex-col items-center gap-3 w-full mt-4 px-4">
    <!-- Overall progress -->
    <div class="flex flex-col items-center text-xs w-full">
      <span class="text-muted-foreground">Uploading...</span>
      <span class="text-muted-foreground">
        {{ state.uploadedFiles }} / {{ state.totalFiles }}
      </span>
    </div>
    <div class="relative h-2 w-full overflow-hidden rounded-full bg-secondary">
      <div
        class="h-full bg-primary transition-all"
        :style="{ width: state.totalFiles > 0 ? `${(state.uploadedFiles / state.totalFiles) * 100}%` : '0%' }"
      />
    </div>

    <!-- Cancel button -->
    <Button
      variant="destructive"
      class="cursor-pointer w-full"
      @click="() => uploadManager.cancelUpload()"
    >
      Cancel
    </Button>

    <!-- Thread progress list -->
    <div
      v-if="threadsList.length > 0"
      class="w-full flex flex-col gap-1"
    >
      <ThreadProgress
        v-for="thread in threadsList"
        :key="thread.WorkerID"
        :thread="thread"
      />
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import Button from "./components/ui/button/Button.vue"
import { Progress } from "./components/ui/progress"
import ThreadProgress from "./components/ThreadProgress.vue"
import { uploadManager } from './utils/UploadManager'
import { X } from 'lucide-vue-next'

const { state } = uploadManager

const threadsList = computed(() => {
  return Array.from(state.threads.values())
    .filter(thread => thread.Status !== 'idle')
    .sort((a, b) => a.WorkerID - b.WorkerID)
})

const progressPercent = computed(() => {
  if (state.totalFiles === 0) return 0
  return Math.round((state.uploadedFiles / state.totalFiles) * 100)
})
</script>

<template>
  <div class="flex flex-col items-center gap-4 w-full max-w-md mx-auto pt-8 px-4">
    <!-- Header -->
    <div class="text-center space-y-1">
      <h2 class="text-lg font-semibold">
        Uploading...
      </h2>
      <p class="text-sm text-muted-foreground">
        {{ state.uploadedFiles }} of {{ state.totalFiles }} files
      </p>
    </div>

    <!-- Progress -->
    <div class="w-full space-y-2">
      <div class="flex justify-between text-xs text-muted-foreground">
        <span>Progress</span>
        <span>{{ progressPercent }}%</span>
      </div>
      <Progress
        :model-value="progressPercent"
        class="h-2"
      />
    </div>

    <!-- Thread list -->
    <div
      v-if="threadsList.length > 0"
      class="w-full space-y-1.5"
    >
      <p class="text-xs text-muted-foreground">
        Active threads
      </p>
      <div class="space-y-1">
        <ThreadProgress
          v-for="thread in threadsList"
          :key="thread.WorkerID"
          :thread="thread"
        />
      </div>
    </div>

    <!-- Cancel -->
    <Button
      variant="destructive"
      class="w-full"
      @click="() => uploadManager.cancelUpload()"
    >
      <X
        class="mr-1"
        :size="16"
      />
      Cancel
    </Button>
  </div>
</template>

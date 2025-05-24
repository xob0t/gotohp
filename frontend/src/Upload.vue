<script setup lang="ts">
import Button from "./components/ui/button/Button.vue"
import { uploadManager } from './utils/UploadManager'

// Access the reactive state from the upload manager
const { state } = uploadManager
</script>

<template>
  <div class="flex flex-col items-center gap-5 w-full mt-30 px-20">
    <div class="flex flex-col items-center text-sm">
      <span class="text-muted-foreground">Uploading...</span>
      <span class="text-muted-foreground">
        {{ state.uploadedFiles }} of {{ state.totalFiles }}
      </span>
    </div>
    <div class="relative h-2 w-full overflow-hidden rounded-full bg-secondary">
      <div class="h-full bg-primary transition-all"
        :style="{ width: state.totalFiles > 0 ? `${(state.uploadedFiles / state.totalFiles) * 100}%` : '0%' }" />
    </div>
    <Button variant="destructive" class="cursor-pointer" @click="() => uploadManager.cancelUpload()">
      Cancel
    </Button>
  </div>
</template>
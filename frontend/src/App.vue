<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet'
import { useColorMode } from '@vueuse/core'
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { ConfigManager } from '../bindings/app/backend'
import { Events } from '@wailsio/runtime'
import Button from "./components/ui/button/Button.vue"
import EditableSelect from "./components/ui/EditableSelect.vue"
import './index.css'
import SettingsPanel from "./SettingsPanel.vue"
import Upload from './Upload.vue'
import { uploadManager } from './utils/UploadManager'

import { toast, Toaster } from "vue-sonner"

useColorMode().value = "dark"

const { state: uploadState } = uploadManager
const copyButtonText = ref('Copy as JSON');

// Drag state for dual drop zones
const isDraggingFiles = ref(false)
let dragEnterCount = 0

// Album upload flow state
const showAlbumInput = ref(false)
const pendingFiles = ref<string[]>([])
const pendingFileCount = ref(0)

const selectedOption = ref('')
const options = ref<string[]>([])
const credentialMap = ref<Record<string, string>>({})
const albumNameOrKey = ref('')

function extractEmailFromCredential(credential: string): string | null {
  try {
    const params = new URLSearchParams(credential)
    return params.get('Email') || null
  } catch (error) {
    console.error('Failed to parse credential:', error)
    return null
  }
}

watch(selectedOption, async (newValue) => {
  if (newValue) {
    try {
      await ConfigManager.SetSelected(newValue)
      console.log('Successfully updated selected value:', newValue)
    } catch (error) {
      console.error('Failed to update selected value:', error)
      toast.error('Failed to update selected account.')
    }
  }
})

async function addCredentials(authString: string) {
  try {
    await ConfigManager.AddCredentials(authString)

    const email = extractEmailFromCredential(authString)
    if (email) {
      credentialMap.value[email] = authString
      if (!options.value.includes(email)) {
        options.value = [...options.value, email]
      }
      selectedOption.value = email
    }
    toast.success('Credentials added successfully!')
    return true
  } catch (error: unknown) {
    console.error('Failed to add credentials:', error)
    toast.error('Failed to add credentials', {
      description: error instanceof Error ? error.message : String(error),
    })
    return false
  }
}

async function removeCredentials(email: string) {
  try {
    await ConfigManager.RemoveCredentials(email)

    if (credentialMap.value[email]) {
      delete credentialMap.value[email]
      options.value = options.value.filter(opt => opt !== email)
      if (selectedOption.value === email) {
        selectedOption.value = ''
      }
    }
    toast.success('Credentials removed.')
    return true
  } catch (error) {
    console.error('Failed to remove credentials:', error)
    toast.error('Failed to remove credentials.')
    return false
  }
}

onMounted(async () => {
  const config = await ConfigManager.GetConfig()
  if (config.credentials?.length) {
    credentialMap.value = {}
    options.value = []

    config.credentials.forEach(credential => {
      const email = extractEmailFromCredential(credential)
      if (email) {
        credentialMap.value[email] = credential
        options.value.push(email)
      }
    })

    if (config.selected) {
      selectedOption.value = config.selected
    }
  }

})

const handleCopyClick = () => {
  uploadManager.copyResultsAsJson();
  copyButtonText.value = 'Copied!';
  setTimeout(() => copyButtonText.value = 'Copy as JSON', 1000);
};



// Global drag event handlers to detect file dragging
let dragLeaveTimeout: ReturnType<typeof setTimeout> | null = null

const onDragEnter = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  // Clear any pending drag leave timeout
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  isDraggingFiles.value = true
}

const onDragOver = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  e.preventDefault()
  
  // Clear any pending drag leave timeout - we're still dragging
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
}

const onDragLeave = (e: DragEvent) => {
  if (!e.dataTransfer?.types.includes('Files')) return
  
  // Use timeout to detect if we've truly left the window
  // dragover will cancel this if we're still in the window
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
  dragLeaveTimeout = setTimeout(() => {
    isDraggingFiles.value = false
    dragLeaveTimeout = null
  }, 50)
}

const onDrop = () => {
  // Clear any pending timeout
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
    dragLeaveTimeout = null
  }
  
  // Delay resetting isDraggingFiles to allow Wails to process the drop target
  // before Vue re-renders and hides the drop zones
  setTimeout(() => {
    isDraggingFiles.value = false
  }, 100)
}

// Album upload confirmation
const confirmAlbumUpload = async () => {
  // Set album name in backend (not persisted to disk)
  await ConfigManager.SetAlbumName(albumNameOrKey.value)
  await ConfigManager.SetAlbumAutoMode(false)
  // Start upload with pending files
  Events.Emit('startUpload', { files: pendingFiles.value })
  showAlbumInput.value = false
  pendingFiles.value = []
  pendingFileCount.value = 0
  albumNameOrKey.value = '' // Reset for next upload
}

const cancelAlbumUpload = () => {
  showAlbumInput.value = false
  pendingFiles.value = []
  pendingFileCount.value = 0
  albumNameOrKey.value = '' // Reset on cancel too
}

// Handle album error event
const onAlbumError = (event: CustomEvent<{ AlbumName: string; Error: string }>) => {
  const { AlbumName, Error } = event.detail
  // Check if it's a 404 error (album key not found)
  if (Error.includes('404')) {
    toast.error('Album not found', {
      description: `The album key "${AlbumName}" does not exist or is invalid.`,
    })
  } else {
    toast.error('Failed to create album', {
      description: `Album "${AlbumName}": ${Error}`,
    })
  }
}

onMounted(() => {
  document.addEventListener('dragenter', onDragEnter)
  document.addEventListener('dragleave', onDragLeave)
  document.addEventListener('dragover', onDragOver)
  document.addEventListener('drop', onDrop)
  window.addEventListener('albumError', onAlbumError as EventListener)

  // Listen for files-dropped event from backend
  Events.On('files-dropped', (event: { data: { files: string[]; dropZone: string } }) => {
    const { files, dropZone } = event.data

    if (dropZone === 'album') {
      // Show album input screen
      pendingFiles.value = files
      pendingFileCount.value = files.length
      showAlbumInput.value = true
    } else if (dropZone === 'auto-album') {
      // Auto album mode - create albums based on folder names
      ConfigManager.SetAlbumName('')
      ConfigManager.SetAlbumAutoMode(true)
      Events.Emit('startUpload', { files })
    } else {
      // Regular upload (no album)
      ConfigManager.SetAlbumName('')
      ConfigManager.SetAlbumAutoMode(false)
      Events.Emit('startUpload', { files })
    }
  })
})

onUnmounted(() => {
  document.removeEventListener('dragenter', onDragEnter)
  document.removeEventListener('dragleave', onDragLeave)
  document.removeEventListener('dragover', onDragOver)
  document.removeEventListener('drop', onDrop)
  window.removeEventListener('albumError', onAlbumError as EventListener)
  if (dragLeaveTimeout) {
    clearTimeout(dragLeaveTimeout)
  }
})
</script>

<template>
  <main
    class="w-screen h-screen flex flex-col items-center"
    style="--wails-draggable: drag"
  >
    <!-- Drop zones shown when dragging files -->
    <div
      v-if="!uploadState.isUploading && isDraggingFiles && options.length > 0"
      class="w-screen h-screen flex flex-col gap-3 p-6"
      style="--wails-draggable: none"
    >
      <div
        data-file-drop-target
        data-drop-zone="regular"
        class="flex-1 flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/50 rounded-xl transition-all duration-200 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">Upload Only</h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">Upload files without adding to any album</p>
      </div>
      <div
        data-file-drop-target
        data-drop-zone="album"
        class="flex-1 flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/50 rounded-xl transition-all duration-200 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">Upload to Album</h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">Upload and add to a specific album (you'll enter the name)</p>
      </div>
      <div
        data-file-drop-target
        data-drop-zone="auto-album"
        class="flex-1 flex flex-col items-center justify-center border-2 border-dashed border-muted-foreground/50 rounded-xl transition-all duration-200 drop-zone"
      >
        <h2 class="text-xl font-semibold select-none text-muted-foreground">Auto Album</h2>
        <p class="text-sm text-muted-foreground/70 mt-2 select-none text-center px-4">Upload and create albums automatically based on folder names</p>
      </div>
    </div>

    <!-- Normal UI (not dragging) -->
    <div
      v-else-if="!uploadState.isUploading"
      class="w-screen h-screen flex flex-col items-center gap-4 max-w-md pt-30"
      data-file-drop-target
    >
      <template v-if="options.length === 0">
        <EditableSelect
          v-model="selectedOption"
          :options="options"
          @update:options="(newOptions) => options = newOptions"
          @item-added="addCredentials"
          @item-removed="removeCredentials"
        />
      </template>

      <template v-else>
        <!-- Show album input screen when files dropped on album zone -->
        <template v-if="showAlbumInput">
          <div class="flex flex-col items-center justify-center gap-6 p-8" style="--wails-draggable: none">
            <h1 class="text-xl font-semibold select-none">Upload to Album</h1>
            <p class="text-muted-foreground select-none">{{ pendingFileCount }} file(s) ready to upload</p>
            
            <div class="flex flex-col gap-2 w-full max-w-xs">
              <Label for="album-input" class="text-muted-foreground text-sm">Album name or key</Label>
              <Input
                id="album-input"
                v-model="albumNameOrKey"
                placeholder="Album name or AF1Qip... key"
                autofocus
              />
            </div>

            <div class="flex gap-4">
              <Button
                variant="outline"
                class="cursor-pointer select-none"
                @click="cancelAlbumUpload"
              >
                Cancel
              </Button>
              <Button
                class="cursor-pointer select-none"
                :disabled="!albumNameOrKey.trim()"
                @click="confirmAlbumUpload"
              >
                Upload
              </Button>
            </div>
          </div>
        </template>

        <!-- Normal UI when not dragging -->
        <template v-else>
          <h1 class="text-xl font-semibold select-none">
            Drop files to upload
          </h1>
          <EditableSelect
            v-model="selectedOption"
            :options="options"
            @update:options="(newOptions) => options = newOptions"
            @item-added="addCredentials"
            @item-removed="removeCredentials"
          />

          <Sheet>
            <SheetTrigger>
              <Button
                variant="outline"
                class="cursor-pointer select-none"
              >
                Settings
              </Button>
            </SheetTrigger>
            <SheetContent side="bottom">
              <SettingsPanel />
            </SheetContent>
          </Sheet>

          <div
            v-if="uploadState.uploadedFiles > 0"
            class="flex flex-col items-center gap-2 border rounded-lg p-5 mt-5"
          >
            <h2 class="text-l font-semibold select-none ">
              Upload Results
            </h2>
            <Label class="text-muted-foreground">Successful: {{ uploadState.results.success.length }}</Label>
            <Label class="text-muted-foreground">Failed: {{ uploadState.results.fail.length }}</Label>
            <Button
              variant="outline"
              class="cursor-pointer select-none min-w-[125px]"
              @click="handleCopyClick"
            >
              {{ copyButtonText }}
            </Button>
          </div>
        </template>
      </template>
    </div>
    <div
      v-if="uploadState.isUploading"
      class="w-full h-full"
    >
      <Upload />
    </div>
    <Toaster position="bottom-center" />
  </main>
</template>

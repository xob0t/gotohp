<script setup lang="ts">
import './index.css'
import { ref, onMounted, watch } from 'vue'
import EditableSelect from "./components/ui/EditableSelect.vue"
import SettingsPanel from "./SettingsPanel.vue";
import { Events, Clipboard } from "@wailsio/runtime";

import { Label } from '@/components/ui/label'
import { ConfigManager } from '../bindings/app/backend'
import Button from "./components/ui/button/Button.vue";
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet'

import { useColorMode } from '@vueuse/core'

useColorMode().value = "dark"

const totalFiles = ref(0)
const uploadedFiles = ref(0)
const isUploading = ref(false)

const uploadResults = ref<{
  success: { path: string; mediaKey: string }[];
  fail: string[];
}>({
  success: [],
  fail: []
})

const selectedOption = ref('')
const options = ref<string[]>([]) // Only emails here
const credentialMap = ref<Record<string, string>>({}) // Maps email to full credential

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
    }
  }
})


async function addCredentials(authString: string) {
  try {
    // First validate and add via backend
    await ConfigManager.AddCredentials(authString)

    // If successful, update frontend state
    const email = extractEmailFromCredential(authString)
    if (email) {
      credentialMap.value[email] = authString
      if (!options.value.includes(email)) {
        options.value = [...options.value, email]
      }
      selectedOption.value = email
    }
    return true
  } catch (error) {
    console.error('Failed to add credentials:', error)
    return false
  }
}

async function removeCredentials(email: string) {
  try {
    // First remove via backend
    await ConfigManager.RemoveCredentials(email)

    // If successful, update frontend state
    if (credentialMap.value[email]) {
      delete credentialMap.value[email]
      options.value = options.value.filter(opt => opt !== email)
      if (selectedOption.value === email) {
        selectedOption.value = ''
      }
    }
    return true
  } catch (error) {
    console.error('Failed to remove credentials:', error)
    return false
  }
}


// Create a function to reset upload results
function resetUploadResults() {
  uploadResults.value = {
    success: [],
    fail: []
  }
}

// Function to copy upload results as JSON
function copyResultsAsJson() {
  const resultsJson = JSON.stringify(uploadResults.value, null, 2)
  Clipboard.SetText(resultsJson)
    .then(() => {
      console.log('Upload results copied to clipboard')
    })
    .catch((error) => {
      console.error('Failed to copy results:', error)
    })
}

onMounted(() => {
  Events.On('FileStatus', (event: { data: Array<{ IsError: boolean, Path: string, MediaKey: string }> }) => {
    const { IsError, Path, MediaKey } = event.data[0]

    if (!IsError) {
      uploadedFiles.value += 1
      uploadResults.value.success.push({ path: Path, mediaKey: MediaKey })
    } else {
      uploadResults.value.fail.push(Path)
    }
  });
})

onMounted(() => {
  Events.On('uploadStart', (event: { data: Array<{ Total: number }> }) => {
    totalFiles.value = event.data[0].Total
    uploadedFiles.value = 0
    isUploading.value = true
    resetUploadResults()
  });
})

onMounted(() => {
  Events.On('uploadStop', () => {
    isUploading.value = false
  });
})


onMounted(async () => {
  const config = await ConfigManager.GetConfig()
  if (config.credentials?.length) {
    // Initialize both the dropdown options and credential map
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

function CancelUpload() {
  const event = new Events.WailsEvent("uploadCancel");
  Events.Emit(event);
}

</script>

<template>
  <main class="p-20 size-full flex flex-col">
    <div v-if="!isUploading" class="flex-1 flex flex-col items-center justify-center gap-4">
      <template v-if="options.length === 0">
        <EditableSelect v-model="selectedOption" :options="options"
          @update:options="(newOptions) => options = newOptions" @item-added="addCredentials"
          @item-removed="removeCredentials" />
      </template>

      <template v-else>
        <h1 class="text-xl font-semibold select-none">
          Drop files to upload
        </h1>
        <EditableSelect v-model="selectedOption" :options="options"
          @update:options="(newOptions) => options = newOptions" @item-added="addCredentials"
          @item-removed="removeCredentials" />

        <Sheet>
          <SheetTrigger>
            <Button variant="outline" class="cursor-pointer select-none">
              Settings
            </Button>
          </SheetTrigger>
          <SheetContent side="bottom">
            <SettingsPanel />
          </SheetContent>
        </Sheet>

        <div v-if="uploadedFiles > 0" class="flex flex-col items-center gap-2 border rounded-lg p-5 mt-10">
          <h2 class="text-l font-semibold select-none ">Upload Results</h2>
          <Label class="text-muted-foreground">Successful ({{ uploadResults.success.length }})</Label>
          <Label class="text-muted-foreground">Failed ({{ uploadResults.fail.length }})</Label>
          <Button variant="outline" class="cursor-pointer select-none" @click="copyResultsAsJson">
            Copy as JSON
          </Button>
        </div>
      </template>
    </div>
    <div v-if="isUploading" class="w-full mt-6 space-y-2 flex flex-col items-center gap-5">
      <div class="flex flex-col items-center text-sm">
        <span class="text-muted-foreground">Uploading...</span>
        <span class="text-muted-foreground">
          {{ uploadedFiles }} of {{ totalFiles }}
        </span>
      </div>
      <div class="relative h-2 w-full overflow-hidden rounded-full bg-secondary">
        <div class="h-full w-full flex-1 bg-primary transition-all"
          :style="{ width: totalFiles > 0 ? `${(uploadedFiles / totalFiles) * 100}%` : '0%' }" />
      </div>
      <Button variant="destructive" class="cursor-pointer" @click="CancelUpload">
        Cancel
      </Button>
    </div>
  </main>
</template>
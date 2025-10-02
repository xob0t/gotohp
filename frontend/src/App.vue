<script setup lang="ts">
import './index.css'
import { ref, onMounted, watch } from 'vue'
import EditableSelect from "./components/ui/EditableSelect.vue"
import Upload from './Upload.vue'
import SettingsPanel from "./SettingsPanel.vue"
import { uploadManager } from './utils/UploadManager'

import { Label } from '@/components/ui/label'
import { ConfigManager } from '../bindings/app/backend'
import Button from "./components/ui/button/Button.vue"
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet'

import { useColorMode } from '@vueuse/core'

useColorMode().value = "dark"

// Access upload state from manager
const { state: uploadState } = uploadManager
const copyButtonText = ref('Copy as JSON');

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

const handleCopyClick = () => {
  uploadManager.copyResultsAsJson();
  copyButtonText.value = 'Copied!';
  setTimeout(() => copyButtonText.value = 'Copy as JSON', 1000);
};
</script>

<template>
  <main class="w-screen h-screen flex flex-col items-center" style="--wails-draggable: drag">
    <div v-if="!uploadState.isUploading" class="w-screen h-screen flex flex-col items-center gap-4 max-w-md pt-30"
      data-wails-dropzone>
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

        <div v-if="uploadState.uploadedFiles > 0" class="flex flex-col items-center gap-2 border rounded-lg p-5 mt-5">
          <h2 class="text-l font-semibold select-none ">Upload Results</h2>
          <Label class="text-muted-foreground">Successful: {{ uploadState.results.success.length }}</Label>
          <Label class="text-muted-foreground">Failed: {{ uploadState.results.fail.length }}</Label>
          <Button variant="outline" class="cursor-pointer select-none min-w-[125px]" @click="handleCopyClick">
            {{ copyButtonText }}
          </Button>
        </div>
      </template>
    </div>
    <div v-if="uploadState.isUploading" class="w-full">
      <Upload />
    </div>
  </main>
</template>
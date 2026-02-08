<script setup lang="ts">
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetTrigger,
} from '@/components/ui/sheet'
import { useColorMode } from '@vueuse/core'
import { onMounted, ref, watch } from 'vue'
import { ConfigManager } from '../bindings/app/backend'
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

const selectedOption = ref('')
const options = ref<string[]>([])
const credentialMap = ref<Record<string, string>>({})

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
</script>

<template>
  <main
    class="w-screen h-screen flex flex-col items-center"
    style="--wails-draggable: drag"
  >
    <div
      v-if="!uploadState.isUploading"
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
    </div>
    <div
      v-if="uploadState.isUploading"
      class="w-full"
    >
      <Upload />
    </div>
    <Toaster position="bottom-center" />
  </main>
</template>

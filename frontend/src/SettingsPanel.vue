<script setup lang="ts">
import { nextTick, ref, onMounted, watch } from 'vue'
import { ConfigManager } from '../bindings/app/core'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import SettingRow from '@/components/SettingRow.vue'
import { debugEnabled } from '@/utils/debug'
import { thumbnailsEnabled } from '@/utils/thumbnails'

import {
    NumberField,
    NumberFieldContent,
    NumberFieldDecrement,
    NumberFieldIncrement,
    NumberFieldInput,
} from '@/components/ui/number-field'

interface Settings {
    proxy: string
    useQuota: boolean
    saver: boolean
    recursive: boolean
    forceUpload: boolean
    pairLivePhotos: boolean
    skipIncompleteLivePhotos: boolean
    updateExistingPhotosToLive: boolean
    deleteFromHost: boolean
    disableUnsupportedFilesFilter: boolean
    setDateFromFilename: boolean
    uploadThreads: number
}

const settings = ref<Settings>({
    proxy: '',
    useQuota: false,
    saver: false,
    recursive: false,
    forceUpload: false,
    pairLivePhotos: false,
    skipIncompleteLivePhotos: true,
    updateExistingPhotosToLive: false,
    deleteFromHost: false,
    disableUnsupportedFilesFilter: false,
    setDateFromFilename: false,
    uploadThreads: 0
})
const isHydrating = ref(true)

onMounted(async () => {
    try {
        const config = await ConfigManager.GetSettings()
        settings.value = {
            proxy: config.proxy || '',
            useQuota: config.useQuota || false,
            saver: config.saver || false,
            recursive: config.recursive || false,
            forceUpload: config.forceUpload || false,
            pairLivePhotos: config.pairLivePhotos || false,
            skipIncompleteLivePhotos: config.skipIncompleteLivePhotos ?? true,
            updateExistingPhotosToLive: config.updateExistingPhotosToLive || false,
            deleteFromHost: config.deleteFromHost || false,
            disableUnsupportedFilesFilter: config.disableUnsupportedFilesFilter || false,
            setDateFromFilename: config.setDateFromFilename || false,
            uploadThreads: config.uploadThreads || 1
        }
    } finally {
        await nextTick()
        isHydrating.value = false
    }
})

// Watch for changes to proxy value and update backend
watch(() => settings.value.proxy, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetProxy(newValue)
})

// Create individual watchers for each boolean setting
watch(() => settings.value.useQuota, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetUseQuota(newValue)
})

watch(() => settings.value.saver, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSaver(newValue)
})

watch(() => settings.value.recursive, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetRecursive(newValue)
})

watch(() => settings.value.forceUpload, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetForceUpload(newValue)
})

watch(() => settings.value.pairLivePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetPairLivePhotos(newValue)
    if (newValue && !settings.value.skipIncompleteLivePhotos) {
        settings.value.skipIncompleteLivePhotos = true
        await ConfigManager.SetSkipIncompleteLivePhotos(true)
    }
})

watch(() => settings.value.skipIncompleteLivePhotos, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSkipIncompleteLivePhotos(newValue)
})

watch(() => settings.value.updateExistingPhotosToLive, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetUpdateExistingPhotosToLive(newValue)
})

watch(() => settings.value.deleteFromHost, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetDeleteFromHost(newValue)
})

watch(() => settings.value.disableUnsupportedFilesFilter, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetDisableUnsupportedFilesFilter(newValue)
})

watch(() => settings.value.setDateFromFilename, async (newValue) => {
    if (isHydrating.value) return
    await ConfigManager.SetSetDateFromFilename(newValue)
})

watch(() => settings.value.uploadThreads, async (newValue) => {
    if (isHydrating.value) return
    if (newValue < 1) {
        settings.value.uploadThreads = 1
    } else {
        await ConfigManager.SetUploadThreads(newValue)
    }
})
</script>

<template>
  <div class="mx-auto flex w-full max-w-2xl flex-col gap-4 px-4 pb-5">
    <section class="flex flex-col gap-1.5">
      <h3 class="px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
        Upload
      </h3>
      <div class="divide-y overflow-hidden rounded-xl border bg-card">
        <SettingRow
          control-id="upload-threads"
          title="Upload threads"
          description="Number of parallel upload threads."
          config-key="uploadThreads"
        >
          <NumberField
            id="upload-threads"
            v-model="settings.uploadThreads"
            :min="1"
            class="w-28 shrink-0"
          >
            <NumberFieldContent>
              <NumberFieldDecrement
                class="cursor-pointer"
                :disabled="settings.uploadThreads <= 1"
              />
              <NumberFieldInput />
              <NumberFieldIncrement class="cursor-pointer" />
            </NumberFieldContent>
          </NumberField>
        </SettingRow>
        <SettingRow
          control-id="saver-mode"
          title="Storage saver quality"
          description="Upload in storage saver quality instead of original quality."
          config-key="saver"
        >
          <Switch
            id="saver-mode"
            v-model="settings.saver"
          />
        </SettingRow>
        <SettingRow
          control-id="use-quota"
          title="Use quota"
          description="Count uploads against the account's storage quota."
          config-key="useQuota"
        >
          <Switch
            id="use-quota"
            v-model="settings.useQuota"
          />
        </SettingRow>
        <SettingRow
          control-id="force-upload"
          title="Force upload"
          description="Upload even if the file already exists in the library. Applies to single files; Live Photo pairs always check both components for duplicates."
          config-key="forceUpload"
        >
          <Switch
            id="force-upload"
            v-model="settings.forceUpload"
          />
        </SettingRow>
      </div>
    </section>

    <section class="flex flex-col gap-1.5">
      <h3 class="px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
        Files
      </h3>
      <div class="divide-y overflow-hidden rounded-xl border bg-card">
        <SettingRow
          control-id="recursive"
          title="Recursive directory upload"
          description="Include subdirectories of dropped folders."
          config-key="recursive"
        >
          <Switch
            id="recursive"
            v-model="settings.recursive"
          />
        </SettingRow>
        <SettingRow
          control-id="filter-unsupported"
          title="Disable unsupported files filter"
          description="Turn off file type filtering and try every file."
          config-key="disableUnsupportedFilesFilter"
        >
          <Switch
            id="filter-unsupported"
            v-model="settings.disableUnsupportedFilesFilter"
          />
        </SettingRow>
        <SettingRow
          control-id="set-date-from-filename"
          title="Set upload date from filename"
          description="Set the media date from the filename, e.g. 20240709_182027.jpg."
          config-key="setDateFromFilename"
        >
          <Switch
            id="set-date-from-filename"
            v-model="settings.setDateFromFilename"
          />
        </SettingRow>
      </div>
    </section>

    <section class="flex flex-col gap-1.5">
      <h3 class="px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
        Live Photos
      </h3>
      <div class="divide-y overflow-hidden rounded-xl border bg-card">
        <SettingRow
          control-id="pair-live-photos"
          title="Pair Apple Live Photos"
          description="Identify Apple Live Photo image and MOV pairs via embedded metadata and upload them as one item."
          config-key="pairLivePhotos"
        >
          <Switch
            id="pair-live-photos"
            v-model="settings.pairLivePhotos"
          />
        </SettingRow>
        <SettingRow
          control-id="skip-incomplete-live-photos"
          title="Skip incomplete Live Photos"
          description="Only upload Live Photos when both photo and video are available. If either is missing, don't upload the remaining file separately."
          config-key="skipIncompleteLivePhotos"
          nested
          :disabled="!settings.pairLivePhotos"
        >
          <Switch
            id="skip-incomplete-live-photos"
            v-model="settings.skipIncompleteLivePhotos"
            :disabled="!settings.pairLivePhotos"
          />
        </SettingRow>
        <SettingRow
          control-id="update-existing-photos-to-live"
          title="Update existing photos to Live Photos"
          description="If a static photo was already uploaded, re-upload it alongside its MOV companion to upgrade it to a Live Photo. Both local photo and video files are required."
          config-key="updateExistingPhotosToLive"
          nested
          :disabled="!settings.pairLivePhotos"
        >
          <Switch
            id="update-existing-photos-to-live"
            v-model="settings.updateExistingPhotosToLive"
            :disabled="!settings.pairLivePhotos"
          />
        </SettingRow>
      </div>
    </section>

    <section class="flex flex-col gap-1.5">
      <h3 class="px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
        Network
      </h3>
      <div class="rounded-xl border bg-card p-3">
        <label
          for="proxy-url"
          class="mb-1.5 block text-sm leading-tight font-medium select-none"
        >
          Proxy
          <code
            v-if="debugEnabled"
            class="ml-1 rounded bg-muted px-1 py-px font-mono text-[10px] font-normal text-muted-foreground"
          >proxy</code>
        </label>
        <Input
          id="proxy-url"
          v-model="settings.proxy"
          type="text"
          spellcheck="false"
          autocomplete="off"
          placeholder="Proxy URL (optional)"
          class="font-mono text-xs"
        />
      </div>
    </section>

    <section class="flex flex-col gap-1.5">
      <h3 class="px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
        Danger zone
      </h3>
      <div class="overflow-hidden rounded-xl border border-destructive/40 bg-destructive/5">
        <SettingRow
          control-id="delete-host"
          title="Delete from host after upload"
          description="Deletes local files once they are uploaded."
          config-key="deleteFromHost"
          danger
        >
          <Switch
            id="delete-host"
            v-model="settings.deleteFromHost"
            variant="destructive"
          />
        </SettingRow>
      </div>
    </section>

    <section class="flex flex-col gap-1.5">
      <h3 class="px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase select-none">
        Advanced
      </h3>
      <div class="divide-y overflow-hidden rounded-xl border bg-card">
        <SettingRow
          control-id="show-thumbnails"
          title="Show thumbnails"
          description="Preview photos and videos while they upload. Shown only when the window is wide enough; turn off to save memory and disk reads."
        >
          <Switch
            id="show-thumbnails"
            v-model="thumbnailsEnabled"
          />
        </SettingRow>
        <SettingRow
          control-id="debug-mode"
          title="Debug mode"
          description="Show technical details and the debug panel: live event log, state, config and system info. Shortcut: Ctrl/Cmd + Shift + D."
        >
          <Switch
            id="debug-mode"
            v-model="debugEnabled"
          />
        </SettingRow>
      </div>
    </section>
  </div>
</template>

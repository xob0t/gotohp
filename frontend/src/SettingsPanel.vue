<script setup lang="ts">
import { nextTick, ref, onMounted, watch } from 'vue'
import { ConfigManager } from '../bindings/app/backend'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

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
        const config = await ConfigManager.GetConfig()
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
  <div class="flex flex-col gap-2.5 m-4">
    <NumberField
      v-model="settings.uploadThreads"
      class="flex items-center justify-between"
    >
      <Label
        for="upload-threads"
        class="size-full"
      >Upload Threads</Label>
      <NumberFieldContent>
        <NumberFieldDecrement
          class="cursor-pointer"
          :disabled="settings.uploadThreads <= 1"
        />
        <NumberFieldInput />
        <NumberFieldIncrement class="cursor-pointer" />
      </NumberFieldContent>
    </NumberField>
    <div class="flex items-center justify-between">
      <Label
        for="use-quota"
        class="size-full cursor-pointer"
      >Use Quota</Label>
      <Switch
        id="use-quota"
        v-model="settings.useQuota"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="saver-mode"
        class="size-full cursor-pointer"
      >Storage Saver Quality</Label>
      <Switch
        id="saver-mode"
        v-model="settings.saver"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="recursive"
        class="size-full cursor-pointer"
      >Recursive Directory Upload</Label>
      <Switch
        id="recursive"
        v-model="settings.recursive"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="force-upload"
        class="size-full cursor-pointer"
      >Force Upload</Label>
      <Switch
        id="force-upload"
        v-model="settings.forceUpload"
      />
    </div>
    <p class="-mt-1 text-[11px] leading-snug text-muted-foreground">
      Applies to single files. Live Photo pairs always check both components for duplicates.
    </p>
    <div class="flex items-center justify-between">
      <div class="pr-4">
        <Label
          for="pair-live-photos"
          class="cursor-pointer"
        >Pair Apple Live Photos</Label>
        <p class="mt-0.5 text-[11px] leading-snug text-muted-foreground">
          Identify Apple Live Photo image and MOV pairs via embedded metadata and upload them as one item.
        </p>
      </div>
      <Switch
        id="pair-live-photos"
        v-model="settings.pairLivePhotos"
      />
    </div>
    <div
      class="flex items-center justify-between pl-4 transition-opacity"
      :class="settings.pairLivePhotos ? 'opacity-100' : 'opacity-45'"
    >
      <div class="pr-4">
        <Label
          for="skip-incomplete-live-photos"
          class="cursor-pointer"
        >Skip Incomplete Live Photos</Label>
        <p class="mt-0.5 text-[11px] leading-snug text-muted-foreground">
          Only upload Live Photos when both photo and video are available. If either is missing,
          don’t upload the remaining file separately.
        </p>
      </div>
      <Switch
        id="skip-incomplete-live-photos"
        v-model="settings.skipIncompleteLivePhotos"
        :disabled="!settings.pairLivePhotos"
      />
    </div>
    <div
      class="flex items-center justify-between pl-4 transition-opacity"
      :class="settings.pairLivePhotos ? 'opacity-100' : 'opacity-45'"
    >
      <div class="pr-4">
        <Label
          for="update-existing-photos-to-live"
          class="cursor-pointer"
        >Update Existing Photos to Live Photos</Label>
        <p class="mt-0.5 text-[11px] leading-snug text-muted-foreground">
          If a static photo was already uploaded, re-upload it alongside its MOV companion to upgrade it to a Live Photo. Both local photo and video files are required.
        </p>
      </div>
      <Switch
        id="update-existing-photos-to-live"
        v-model="settings.updateExistingPhotosToLive"
        :disabled="!settings.pairLivePhotos"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="filter-unsupported"
        class="size-full cursor-pointer"
      >Disable Unsupported Files Filter</Label>
      <Switch
        id="filter-unsupported"
        v-model="settings.disableUnsupportedFilesFilter"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="set-date-from-filename"
        class="size-full cursor-pointer"
      >Set Upload Date from Filename</Label>
      <Switch
        id="set-date-from-filename"
        v-model="settings.setDateFromFilename"
      />
    </div>
    <div class="flex items-center justify-between">
      <Label
        for="delete-host"
        class="size-full cursor-pointer"
      >Delete From Host After Upload</Label>
      <Switch
        id="delete-host"
        v-model="settings.deleteFromHost"
        variant="destructive"
      />
    </div>
    <div>
      <Input
        v-model="settings.proxy"
        type="text"
        placeholder="Proxy URL (optional)"
      />
    </div>
  </div>
</template>

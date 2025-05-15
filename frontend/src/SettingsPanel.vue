<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ConfigService } from '../bindings/backend'
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
    deleteFromHost: boolean
    disableUnsupportedFilesFilter: boolean
    uploadThreads: number
}

const settings = ref<Settings>({
    proxy: '',
    useQuota: false,
    saver: false,
    recursive: false,
    forceUpload: false,
    deleteFromHost: false,
    disableUnsupportedFilesFilter: false,
    uploadThreads: 0
})

onMounted(async () => {
    const config = await ConfigService.GetConfig()
    settings.value = {
        proxy: config.proxy || '',
        useQuota: config.useQuota || false,
        saver: config.saver || false,
        recursive: config.recursive || false,
        forceUpload: config.forceUpload || false,
        deleteFromHost: config.deleteFromHost || false,
        disableUnsupportedFilesFilter: config.disableUnsupportedFilesFilter || false,
        uploadThreads: config.uploadThreads || 1
    }
})

// Watch for changes to proxy value and update backend
watch(() => settings.value.proxy, async (newValue) => {
    await ConfigService.SetProxy(newValue)
})

// Create individual watchers for each boolean setting
watch(() => settings.value.useQuota, async (newValue) => {
    await ConfigService.SetUseQuota(newValue)
})

watch(() => settings.value.saver, async (newValue) => {
    await ConfigService.SetSaver(newValue)
})

watch(() => settings.value.recursive, async (newValue) => {
    await ConfigService.SetRecursive(newValue)
})

watch(() => settings.value.forceUpload, async (newValue) => {
    await ConfigService.SetForceUpload(newValue)
})

watch(() => settings.value.deleteFromHost, async (newValue) => {
    await ConfigService.SetDeleteFromHost(newValue)
})

watch(() => settings.value.disableUnsupportedFilesFilter, async (newValue) => {
    await ConfigService.SetDisableUnsupportedFilesFilter(newValue)
})

watch(() => settings.value.uploadThreads, async (newValue) => {
    if (newValue < 1) {
        settings.value.uploadThreads = 1
    } else {
        await ConfigService.SetUploadThreads(newValue)
    }
})
</script>

<template>
    <div class="flex flex-col gap-2 m-4">
        <NumberField v-model="settings.uploadThreads" class="flex items-center justify-between">
            <Label for="upload-threads" class="size-full">Upload Threads</Label>
            <NumberFieldContent>
                <NumberFieldDecrement />
                <NumberFieldInput />
                <NumberFieldIncrement />
            </NumberFieldContent>
        </NumberField>
        <div class="flex items-center justify-between">
            <Label for="use-quota" class="size-full">Use Quota</Label>
            <Switch id="use-quota" v-model="settings.useQuota" />
        </div>
        <div class="flex items-center justify-between">
            <Label for="saver-mode" class="size-full">Storage Saver Quality</Label>
            <Switch id="saver-mode" v-model="settings.saver" />
        </div>
        <div class="flex items-center justify-between">
            <Label for="recursive" class="size-full">Recursive Directory Upload</Label>
            <Switch id="recursive" v-model="settings.recursive" />
        </div>
        <div class="flex items-center justify-between">
            <Label for="force-upload" class="size-full">Force Upload</Label>
            <Switch id="force-upload" v-model="settings.forceUpload" />
        </div>
        <div class="flex items-center justify-between">
            <Label for="filter-unsupported" class="size-full">Disable Unsupported Files Filter</Label>
            <Switch id="filter-unsupported" v-model="settings.disableUnsupportedFilesFilter" />
        </div>
        <div class="flex items-center justify-between">
            <Label for="delete-host" class="size-full">Delete From Host After Upload</Label>
            <Switch id="delete-host" v-model="settings.deleteFromHost" />
        </div>
        <div class="flex items-center space-x-2 mt-4">
            <Input v-model="settings.proxy" type="text" placeholder="Proxy URL (optional)" class="flex-1" />
        </div>
    </div>

</template>
<!-- src/components/wails-controls/WindowControls.vue -->
<script setup lang="ts">
import { twMerge } from "tailwind-merge"
import { onMounted } from "vue"
import Windows from "./controls/Windows.vue"
import type { WindowControlsProps } from "./types"
import { getOsType } from "./utils/os"

defineOptions({
  inheritAttrs: false,
})

const props = withDefaults(defineProps<WindowControlsProps>(), {
  justify: false,
  hide: false,
  hideMethod: "display",
  className: "",
})

let platform = props.platform
onMounted(() => {
  getOsType().then((type) => {
    if (!platform) {
      switch (type) {
        case "darwin":
          platform = "macos"
          break
        case "linux":
          platform = "gnome"
          break
        default:
          platform = "windows"
      }
    }
  })
})

const customClass = twMerge(
  "flex",
  props.className,
  props.hide && (props.hideMethod === "display" ? "hidden" : "invisible")
)
</script>

<template>
  <Windows v-if="platform === 'windows'" :class="twMerge(customClass, props.justify && 'ml-auto')" />
  <Windows v-else :class="twMerge(customClass, props.justify && 'ml-auto')" />
</template>
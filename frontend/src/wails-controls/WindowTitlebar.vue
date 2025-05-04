<!-- src/components/wails-controls/WindowTitlebar.vue -->
<script setup lang="ts">
import { twMerge } from "tailwind-merge"
import { onMounted, ref } from "vue"
import type { WindowControlsProps, WindowTitlebarProps } from "./types"
import { getOsType } from "./utils/os"
import WindowControls from "./WindowControls.vue"

const { windowControlsProps } = withDefaults(
  defineProps<WindowTitlebarProps>(),
  {
    controlsOrder: "system",
  }
)

const osType = ref<string | undefined>(undefined)

onMounted(() => {
  getOsType().then((type) => {
    osType.value = type
  })
})


const customProps = (ml: string) => {
  if (windowControlsProps?.justify !== undefined) return windowControlsProps

  const {
    justify: windowControlsJustify,
    className: windowControlsClassName,
    ...restProps
  } = windowControlsProps || {}
  return {
    justify: false,
    className: twMerge(windowControlsClassName, ml),
    ...restProps,
  } as WindowControlsProps
}
</script>

<template>
  <div :class="twMerge(
    'bg-background flex select-none flex-row items-center justify-between overflow-hidden',
    $attrs.class as string
  )" style="--wails-draggable: drag">
    <slot />
    <div style="--wails-draggable: none">
      <WindowControls :="customProps('ml-auto')" />
    </div>
  </div>
</template>
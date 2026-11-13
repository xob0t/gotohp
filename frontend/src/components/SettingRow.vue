<script setup lang="ts">
import { debugEnabled } from '@/utils/debug'

defineProps<{
  /** id of the control this row labels; clicking anywhere on the row activates it. */
  controlId: string
  title: string
  description?: string
  /** Config key shown next to the title in debug mode. */
  configKey?: string
  disabled?: boolean
  nested?: boolean
  danger?: boolean
}>()
</script>

<template>
  <label
    :for="controlId"
    class="flex items-center gap-3 px-3 py-2.5 transition-opacity select-none"
    :class="[
      disabled ? 'cursor-not-allowed opacity-45' : 'cursor-pointer hover:bg-accent/40',
      nested && 'pl-7',
    ]"
  >
    <span class="flex min-w-0 flex-1 flex-col gap-0.5">
      <span
        class="text-sm leading-tight font-medium"
        :class="danger && 'text-destructive'"
      >
        {{ title }}
        <code
          v-if="debugEnabled && configKey"
          class="ml-1 rounded bg-muted px-1 py-px font-mono text-[10px] font-normal text-muted-foreground"
        >{{ configKey }}</code>
      </span>
      <span
        v-if="description"
        class="text-xs leading-snug text-muted-foreground"
      >{{ description }}</span>
    </span>
    <slot />
  </label>
</template>

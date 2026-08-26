<script setup lang="ts">
import { ref, watch } from 'vue'
import { Check, Plus, X } from '@lucide/vue'
import {
  SelectItem as SelectItemPrimitive,
  SelectItemIndicator,
  SelectItemText,
} from 'reka-ui'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface Props {
  modelValue?: string
  options?: string[]
  removingAccount?: string
}

interface Emits {
  (event: 'update:modelValue', value: string): void
  (event: 'item-removed', value: string): void
  (event: 'add'): void
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: '',
  options: () => [],
  removingAccount: '',
})

const emit = defineEmits<Emits>()
const isOpen = ref(false)
const selectedValue = ref(props.modelValue)

watch(() => props.modelValue, (newValue) => {
  if (newValue !== selectedValue.value) {
    selectedValue.value = newValue || ''
  }
})

watch(selectedValue, (newValue) => {
  emit('update:modelValue', newValue)
})

</script>

<template>
  <Select
    v-model="selectedValue"
    v-model:open="isOpen"
  >
    <SelectTrigger class="max-w-60 select-none">
      <SelectValue placeholder="Select account" />
    </SelectTrigger>
    <SelectContent
      align="center"
      class="w-max min-w-[var(--reka-select-trigger-width)] max-w-[min(15rem,calc(100vw-3rem))]"
    >
      <SelectGroup>
        <div
          v-for="option in options"
          :key="option"
          class="relative flex items-center"
        >
          <SelectItemPrimitive
            :value="option"
            class="focus:bg-accent focus:text-accent-foreground relative flex min-w-0 w-full cursor-default items-center rounded-sm py-1.5 pr-9 pl-8 text-sm outline-hidden select-none"
          >
            <span class="absolute left-2 flex size-3.5 items-center justify-center">
              <SelectItemIndicator>
                <Check class="size-4" />
              </SelectItemIndicator>
            </span>
            <SelectItemText class="min-w-0 flex-1">
              <span class="block max-w-44 truncate">{{ option }}</span>
            </SelectItemText>
          </SelectItemPrimitive>
          <button
            type="button"
            class="absolute right-1.5 top-1/2 z-10 flex size-6 -translate-y-1/2 items-center justify-center rounded text-muted-foreground hover:bg-destructive hover:text-white disabled:pointer-events-none disabled:opacity-50"
            :aria-label="`Remove ${option}`"
            :title="`Remove ${option}`"
            :disabled="removingAccount === option"
            @pointerdown.stop.prevent
            @click.stop.prevent="emit('item-removed', option)"
          >
            <X class="size-3.5" />
          </button>
        </div>
      </SelectGroup>
      <SelectSeparator v-if="options.length" />
      <button
        type="button"
        class="focus:bg-accent focus:text-accent-foreground relative flex w-full cursor-default items-center gap-2 rounded-sm py-1.5 pr-2 pl-8 text-sm text-muted-foreground outline-hidden select-none hover:bg-accent"
        @pointerdown.stop.prevent
        @click.stop.prevent="isOpen = false; emit('add')"
      >
        <Plus class="size-4" />
        Add account
      </button>
    </SelectContent>
  </Select>
</template>

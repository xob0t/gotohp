<script setup lang="ts">
import { ref, watch } from 'vue'
import { X } from 'lucide-vue-next'
import { Select, SelectContent, SelectTrigger, SelectValue, SelectGroup } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import SelectItem from '@/components/ui/CustomSelectItem.vue'
import { Input } from '@/components/ui/input'

interface Props {
    modelValue?: string
    options?: string[]
}

interface Emits {
    (e: 'update:modelValue', value: string): void
    (e: 'update:options', value: string[]): void
    (e: 'item-added', value: string): void
    (e: 'item-removed', value: string): void
}

const props = withDefaults(defineProps<Props>(), {
    modelValue: '',
    options: () => []
})

const emit = defineEmits<Emits>()

const internalOptions = ref([...props.options])
const selectedValue = ref(props.modelValue)
const newOption = ref('')

// Watchers for syncing props with internal state
watch(() => props.modelValue, (newVal) => {
    if (newVal !== selectedValue.value) {
        selectedValue.value = newVal || ''
    }
})

watch(() => props.options, (newOptions) => {
    if (JSON.stringify(newOptions) !== JSON.stringify(internalOptions.value)) {
        internalOptions.value = [...newOptions]
    }
}, { deep: true })

// Emit selected value changes
watch(selectedValue, (newVal) => {
    emit('update:modelValue', newVal)
})

const addOption = () => {
    const trimmed = newOption.value.trim()
    if (trimmed && !internalOptions.value.includes(trimmed)) {
        // Instead of adding directly, emit the event first
        // The parent component will handle validation and then update options if valid
        emit('item-added', trimmed)
        newOption.value = ''
        // Don't auto-select yet - wait for parent to confirm
    }
}

const removeOption = (index: number) => {
    const removedItem = internalOptions.value[index]
    const newOptions = [...internalOptions.value]
    newOptions.splice(index, 1)
    internalOptions.value = newOptions
    emit('update:options', newOptions)
    emit('item-removed', removedItem)

    if (selectedValue.value === removedItem) {
        selectedValue.value = ''
    }
}
</script>

<template>
  <!-- Show only the Add input when there are no options -->
  <div
    v-if="internalOptions.length === 0"
    class="flex gap-2"
  >
    <Input
      v-model="newOption"
      placeholder="Add credentials"
      @keydown.enter="addOption"
    />
    <Button
      type="button"
      :disabled="!newOption.trim()"
      @click="addOption"
    >
      Add
    </Button>
  </div>

  <!-- Show the Select dropdown when there are options -->
  <Select
    v-else
    v-model="selectedValue"
  >
    <SelectTrigger class="select-none">
      <SelectValue placeholder="Select Account" />
    </SelectTrigger>
    <SelectContent align="center">
      <SelectGroup>
        <template
          v-for="(option, index) in internalOptions"
          :key="index"
        >
          <div class="relative flex items-center">
            <SelectItem :value="option">
              <div class="flex w-full items-center justify-between">
                <span class="truncate">{{ option }}</span>
              </div>
            </SelectItem>

            <button
              type="button"
              class="absolute right-2 p-1 rounded hover:bg-destructive z-10 group cursor-pointer"
              :title="`Remove ${option}`"
              @click.stop.prevent="() => removeOption(index)"
            >
              <X class="h-3 w-3 text-muted-foreground group-hover:text-black" />
            </button>
          </div>
        </template>
      </SelectGroup>
      <div class="p-2 flex gap-2">
        <Input
          v-model="newOption"
          class="h-8 px-1"
          placeholder="Credentials"
          @keydown.enter="addOption"
        />
        <Button
          size="sm"
          type="button"
          :disabled="!newOption.trim()"
          @click="addOption"
        >
          Add
        </Button>
      </div>
    </SelectContent>
  </Select>
</template>
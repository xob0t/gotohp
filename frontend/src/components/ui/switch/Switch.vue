<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
import {
  SwitchRoot,
  type SwitchRootEmits,
  type SwitchRootProps,
  SwitchThumb,
  useForwardPropsEmits,
} from 'reka-ui'
import { computed } from 'vue'
import { type SwitchVariants, type SwitchThumbVariants, switchVariants, switchThumbVariants } from '.'

interface Props extends SwitchRootProps {
  variant?: SwitchVariants['variant']
  size?: SwitchVariants['size']
  thumbVariant?: SwitchThumbVariants['variant']
  thumbSize?: SwitchThumbVariants['size']
  class?: HTMLAttributes['class']
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'default',
  size: 'default',
})

const emits = defineEmits<SwitchRootEmits>()

const delegatedProps = computed(() => {
  const { class: _, variant: __, size: ___, thumbVariant: ____, thumbSize: _____, ...delegated } = props

  return delegated
})

const forwarded = useForwardPropsEmits(delegatedProps, emits)

// If thumbVariant or thumbSize not specified, they will default to matching the switch variant and size
const effectiveThumbVariant = computed(() => props.thumbVariant || props.variant)
const effectiveThumbSize = computed(() => props.thumbSize || props.size)
</script>

<template>
  <SwitchRoot data-slot="switch" v-bind="forwarded" :class="cn(
    switchVariants({ variant, size }),
    props.class,
  )">
    <SwitchThumb data-slot="switch-thumb" :class="cn(
      switchThumbVariants({ variant: effectiveThumbVariant, size: effectiveThumbSize })
    )">
      <slot name="thumb" />
    </SwitchThumb>
  </SwitchRoot>
</template>
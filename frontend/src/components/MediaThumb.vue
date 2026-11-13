<script setup lang="ts">
import { computed, watch } from 'vue'
import { File, Film, Image } from '@lucide/vue'
import {
  cachedThumbnail,
  mediaKind,
  requestThumbnail,
  thumbnailsActive,
} from '@/utils/thumbnails'

const props = defineProps<{
  path: string
  /** Show a thumbnail only if it already exists; never start generating one. */
  cachedOnly?: boolean
}>()

const kind = computed(() => mediaKind(props.path))
const thumbnail = computed(() => thumbnailsActive.value ? cachedThumbnail(props.path) : null)

watch(
  () => [props.path, thumbnailsActive.value] as const,
  ([path, active]) => {
    if (active && !props.cachedOnly) requestThumbnail(path)
  },
  { immediate: true },
)
</script>

<template>
  <span class="relative flex shrink-0 items-center justify-center overflow-hidden rounded-md bg-muted text-muted-foreground">
    <img
      v-if="thumbnail"
      :src="thumbnail"
      alt=""
      class="size-full object-cover"
      draggable="false"
    >
    <component
      :is="kind === 'video' ? Film : kind === 'image' ? Image : File"
      v-else
      class="size-2/5"
    />
    <Film
      v-if="thumbnail && kind === 'video'"
      class="absolute right-0.5 bottom-0.5 size-3 rounded-sm bg-black/60 p-px text-white"
    />
  </span>
</template>

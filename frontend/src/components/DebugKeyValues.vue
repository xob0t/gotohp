<script setup lang="ts">
import { safeStringify } from '@/utils/debug'

defineProps<{
  title: string
  rows: [string, unknown][]
}>()

function display(value: unknown): string {
  if (value === '' || value === null || value === undefined) return '—'
  if (typeof value === 'string') return value
  return safeStringify(value)
}
</script>

<template>
  <section>
    <h4 class="mb-1 px-1 text-[11px] font-semibold tracking-wider text-muted-foreground uppercase">
      {{ title }}
    </h4>
    <dl class="divide-y overflow-hidden rounded-lg border bg-card font-mono text-[11px]">
      <div
        v-for="[key, value] in rows"
        :key="key"
        class="flex gap-3 px-2.5 py-1.5"
      >
        <dt class="w-2/5 shrink-0 break-all text-muted-foreground">
          {{ key }}
        </dt>
        <dd class="min-w-0 flex-1 break-all select-text">
          {{ display(value) }}
        </dd>
      </div>
      <p
        v-if="rows.length === 0"
        class="px-2.5 py-1.5 text-muted-foreground"
      >
        —
      </p>
    </dl>
  </section>
</template>

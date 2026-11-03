<script setup lang="ts">
import { computed } from 'vue'
import type { Label } from '@/types/work'

// Labels as colored chips; with max, the rest go into «+N» (spec 007, RF-42, RF-43).
const props = withDefaults(defineProps<{ labels: Label[]; max?: number }>(), { max: Infinity })
const shown = computed(() => props.labels.slice(0, props.max))
const rest = computed(() => props.labels.length - shown.value.length)
</script>

<template>
  <span class="flex min-w-0 flex-wrap items-center gap-1" data-testid="label-chips">
    <span
      v-for="l in shown"
      :key="l.id"
      class="inline-flex max-w-40 items-center gap-1 truncate rounded-full border px-2 py-0.5 text-xs"
      :class="l.retired && 'text-muted-foreground line-through'"
      data-testid="label-chip"
    >
      <span class="size-2 shrink-0 rounded-full" :style="{ background: l.color }" />
      <span class="truncate">{{ l.name }}</span>
    </span>
    <span v-if="rest > 0" class="text-xs text-muted-foreground">+{{ rest }}</span>
    <slot v-if="labels.length === 0" name="empty" />
  </span>
</template>

<script setup lang="ts">
import type { WorkItemType } from '@/types/work'

// The line on top of a kanban card in the colour of its type, which names
// the type on hover (spec 013, RF-14a). The hover area is taller than the line.
defineProps<{ type?: WorkItemType }>()
</script>

<template>
  <span
    v-if="type"
    class="absolute -inset-x-px -top-px h-2"
    :title="type.name"
    role="img"
    :aria-label="type.name"
    data-testid="type-stripe"
  >
    <span class="block h-1 rounded-t-lg" :style="{ background: type.color }" />
  </span>
</template>

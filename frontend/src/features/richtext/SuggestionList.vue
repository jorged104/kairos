<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { SuggestionItem } from './suggestionRender'

// The list of a suggestion (/ menu, @ and # searches), driven by the
// keyboard through onKeyDown (RF-8).
const props = defineProps<{
  items: SuggestionItem[]
  command: (item: SuggestionItem) => void
  title: string
  empty: string
}>()

const active = ref(0)
watch(
  () => props.items,
  () => (active.value = 0),
)

const rows = computed(() =>
  props.items.map((item, index) => ({ item, index, heading: item.group && item.group !== props.items[index - 1]?.group ? item.group : null })),
)

function pick(index: number) {
  const item = props.items[index]
  if (item) props.command(item)
}

function onKeyDown(event: KeyboardEvent): boolean {
  const n = props.items.length
  if (!n) return false
  if (event.key === 'ArrowDown') active.value = (active.value + 1) % n
  else if (event.key === 'ArrowUp') active.value = (active.value + n - 1) % n
  else if (event.key === 'Enter' || event.key === 'Tab') pick(active.value)
  else return false
  return true
}

defineExpose({ onKeyDown })
</script>

<template>
  <div
    role="listbox"
    :aria-label="title"
    class="max-h-80 w-72 overflow-y-auto rounded-lg border bg-popover p-1 text-sm text-popover-foreground shadow-lg"
    data-testid="suggestions"
  >
    <template v-for="row in rows" :key="row.item.id">
      <div v-if="row.heading" class="px-2 pb-1 pt-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{{ row.heading }}</div>
      <button
        type="button"
        role="option"
        :aria-selected="row.index === active"
        class="flex w-full flex-col items-start rounded-md px-2 py-1.5 text-left"
        :class="row.index === active ? 'bg-accent text-accent-foreground' : 'hover:bg-muted'"
        @mousedown.prevent="pick(row.index)"
        @mouseenter="active = row.index"
      >
        <span class="font-medium">{{ row.item.label }}</span>
        <span v-if="row.item.hint" class="text-xs text-muted-foreground">{{ row.item.hint }}</span>
      </button>
    </template>
    <p v-if="!items.length" class="px-2 py-1.5 text-muted-foreground">{{ empty }}</p>
  </div>
</template>

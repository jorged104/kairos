<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Ref as Person } from '@/types/work'

// The assignees of a work item as initials: up to max, then «+N», with every
// name on hover (spec 006, RF-18).
const props = withDefaults(defineProps<{ people: Person[]; max?: number; names?: boolean; bare?: boolean }>(), {
  max: 3,
  names: false,
  bare: false,
})
const { t } = useI18n()

const shown = computed(() => props.people.slice(0, props.max))
const rest = computed(() => props.people.length - shown.value.length)
const all = computed(() => props.people.map((p) => p.name).join(', '))
const initials = (name: string) =>
  name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join('') || '?'
</script>

<template>
  <span v-if="people.length" class="flex min-w-0 items-center gap-1.5" :title="all" data-testid="avatars">
    <span class="flex -space-x-1.5">
      <span
        v-for="p in shown"
        :key="p.id"
        class="flex size-6 shrink-0 items-center justify-center rounded-full border-2 border-background bg-primary/15 text-[10px] font-semibold text-primary"
        aria-hidden="true"
      >
        {{ initials(p.name) }}
      </span>
      <span
        v-if="rest > 0"
        class="flex size-6 shrink-0 items-center justify-center rounded-full border-2 border-background bg-muted text-[10px] font-semibold text-muted-foreground"
        aria-hidden="true"
      >
        +{{ rest }}
      </span>
    </span>
    <span v-if="!bare && (names || people.length === 1)" class="truncate">{{ people.length === 1 ? people[0]!.name : all }}</span>
    <span class="sr-only">{{ all }}</span>
  </span>
  <span v-else class="text-muted-foreground">{{ t('workItems.unassigned') }}</span>
</template>

<script setup lang="ts">
import { keepPreviousData, useMutation, useQuery } from '@tanstack/vue-query'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { linkMilestoneWorkItems, listWorkItems } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useCatalog } from '@/features/work/useCatalog'
import type { MilestoneItem } from '@/types/work'

// Searching the project's work items and linking several to a milestone at
// once (spec 004, RF-162 to RF-167).
const props = defineProps<{ milestoneId: string; projectId: string; items: MilestoneItem[] }>()
const emit = defineEmits<{ done: []; close: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()
const { statuses } = useCatalog()

const PAGE = 50
const text = ref('')
const q = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(text, (v) => {
  clearTimeout(timer)
  timer = setTimeout(() => (q.value = v.trim()), 250)
})
onBeforeUnmount(() => clearTimeout(timer))
const onlyFree = ref(true)

const { data: results, isFetching } = useQuery({
  queryKey: computed(() => ['milestone-search', props.projectId, q.value, onlyFree.value]),
  queryFn: () => listWorkItems(props.projectId, { q: q.value || undefined, milestone_id: onlyFree.value ? 'none' : undefined, page_size: PAGE }),
  placeholderData: keepPreviousData,
})

// Already in the milestone, directly or by a parent (RF-167).
const inMilestone = computed(() => new Map(props.items.map((i) => [i.id, i.inherited ? 'byParent' : 'inMilestone'])))
const selected = ref(new Set<string>())
const selectable = computed(() => (results.value?.items ?? []).filter((n) => !inMilestone.value.has(n.id)))
const allMarked = computed(() => selectable.value.length > 0 && selectable.value.every((n) => selected.value.has(n.id)))
function toggle(id: string) {
  const next = new Set(selected.value)
  if (!next.delete(id)) next.add(id)
  selected.value = next
}
function toggleAll() {
  const next = new Set(selected.value)
  for (const n of selectable.value) {
    if (allMarked.value) next.delete(n.id)
    else next.add(n.id)
  }
  selected.value = next
}

const link = useMutation({
  mutationFn: () => linkMilestoneWorkItems(props.milestoneId, [...selected.value]),
  onSuccess: () => {
    selected.value = new Set()
    emit('done')
  },
})
</script>

<template>
  <section class="flex flex-col gap-3 rounded-lg border bg-background p-3" :aria-label="t('milestones.addTasks')" data-testid="link-work-items">
    <div class="flex flex-wrap items-center gap-3">
      <Input v-model="text" type="search" class="min-w-56 flex-1" :placeholder="t('milestones.search')" :aria-label="t('milestones.search')" data-testid="link-search" />
      <label class="flex items-center gap-2 text-sm">
        <input v-model="onlyFree" type="checkbox" class="size-4 accent-primary" data-testid="link-only-free" />
        {{ t('milestones.onlyUnlinked') }}
      </label>
      <Button size="sm" variant="ghost" @click="emit('close')">{{ t('milestones.closeSearch') }}</Button>
    </div>
    <Alert v-if="link.error.value" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(link.error.value) }}</AlertDescription>
    </Alert>

    <div class="flex items-center gap-2 text-xs text-muted-foreground">
      <label v-if="selectable.length" class="flex items-center gap-2">
        <input type="checkbox" class="size-4 accent-primary" :checked="allMarked" data-testid="link-all" @change="toggleAll" />
        {{ t('milestones.selectAll') }}
      </label>
      <span v-if="results && results.total > results.items.length" class="ml-auto">{{ t('milestones.showing', { shown: results.items.length, total: results.total }) }}</span>
    </div>
    <ul class="max-h-80 divide-y overflow-y-auto rounded-md border text-sm" :aria-busy="isFetching">
      <li v-for="n in results?.items ?? []" :key="n.id" class="flex items-center gap-2 px-2 py-1.5" data-testid="link-result">
        <input
          type="checkbox"
          class="size-4 accent-primary"
          :checked="inMilestone.has(n.id) || selected.has(n.id)"
          :disabled="inMilestone.has(n.id)"
          :aria-label="`${n.key} ${n.title}`"
          data-testid="link-check"
          @change="toggle(n.id)"
        />
        <span class="font-mono text-xs text-muted-foreground">{{ n.key }}</span>
        <span class="truncate">{{ n.title }}</span>
        <Badge v-if="inMilestone.has(n.id)" variant="outline">{{ t(`milestones.${inMilestone.get(n.id)}`) }}</Badge>
        <span class="ml-auto shrink-0 text-xs text-muted-foreground">{{ statuses.get(n.status_id)?.name }}</span>
      </li>
      <li v-if="results && !results.items.length" class="p-3 text-muted-foreground">{{ t('milestones.noResults') }}</li>
    </ul>

    <div class="flex justify-end">
      <Button size="sm" :disabled="!selected.size || link.isPending.value" data-testid="link-add" @click="link.mutate()">
        {{ t('milestones.addN', { count: selected.size }, selected.size) }}
      </Button>
    </div>
  </section>
</template>

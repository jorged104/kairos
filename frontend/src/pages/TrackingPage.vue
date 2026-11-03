<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { getPortfolio } from '@/api/portfolio'
import { getTracking, type TrackingFilters } from '@/api/tracking'
import { listProjects } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import MilestoneGantt from '@/features/tracking/MilestoneGantt.vue'
import TaskColumns from '@/features/tracking/TaskColumns.vue'
import { usePeople } from '@/features/work/usePeople'

// Several projects at once: figures, calendar of milestones and work items
// that need attention, read only (spec 011). The filters live in the URL so
// a copied link opens the same selection (RF-8).
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const errorMessage = useErrorMessage()
const { users } = usePeople()

const list = (v: unknown) => (Array.isArray(v) ? v : v ? [v] : []).map(String)
const filters = computed<TrackingFilters>(() => ({
  program_id: typeof route.query.program_id === 'string' ? route.query.program_id : undefined,
  project_id: list(route.query.project_id),
  owner_id: list(route.query.owner_id),
}))
const any = computed(() => !!filters.value.program_id || !!filters.value.project_id?.length || !!filters.value.owner_id?.length)
function set(next: TrackingFilters) {
  const f = { ...filters.value, ...next }
  void router.replace({ query: { program_id: f.program_id || undefined, project_id: f.project_id?.length ? f.project_id : undefined, owner_id: f.owner_id?.length ? f.owner_id : undefined } })
}
const clear = () => router.replace({ query: {} })

const { data, error } = useQuery({
  queryKey: computed(() => ['tracking', filters.value]),
  queryFn: () => getTracking(filters.value),
  placeholderData: keepPreviousData,
})

// Choices of the filters (RF-2 to RF-4).
const { data: portfolio } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio, staleTime: 60_000 })
const programs = computed(() => (portfolio.value?.programs ?? []).flatMap((c) => (c.program ? [c.program] : [])))
const search = ref('')
const q = ref('')
let timer: ReturnType<typeof setTimeout> | undefined
watch(search, (v) => {
  clearTimeout(timer)
  timer = setTimeout(() => (q.value = v.trim()), 250)
})
const { data: found } = useQuery({
  queryKey: computed(() => ['projects', 'tracking-search', q.value]),
  queryFn: () => listProjects({ q: q.value, lifecycle: 'activo', page_size: 20 }),
  enabled: computed(() => q.value.length > 0),
})
const names = ref(new Map<string, string>())
watch(data, (d) => d?.projects.forEach((p) => names.value.set(p.id, p.name)), { immediate: true })
function addProject(id: string, name: string) {
  names.value.set(id, name)
  search.value = ''
  q.value = ''
  if (!filters.value.project_id?.includes(id)) set({ project_id: [...(filters.value.project_id ?? []), id] })
}
const ownerName = (id: string) => users.value.find((u) => u.id === id)?.name ?? id
const ownerChoice = ref('')
watch(ownerChoice, (id) => {
  if (id && !filters.value.owner_id?.includes(id)) set({ owner_id: [...(filters.value.owner_id ?? []), id] })
  ownerChoice.value = ''
})

const today = computed(() => (data.value ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'long' }).format(new Date(data.value.today + 'T00:00:00')) : ''))
const pct = (n: number) => `${n.toLocaleString('es', { maximumFractionDigits: 1 })} %`
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="tracking">
    <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1">
      <h1 class="font-heading text-xl font-semibold">{{ t('tracking.title') }}</h1>
      <p v-if="data" class="text-sm text-muted-foreground">{{ t('portfolio.today', { date: today }) }}</p>
    </div>

    <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" :aria-label="t('tracking.filters')">
      <div class="grid gap-3 md:grid-cols-3">
        <label class="flex flex-col gap-1 text-sm">
          {{ t('tracking.program') }}
          <NativeSelect :model-value="filters.program_id ?? ''" data-testid="tracking-program" @update:model-value="(v) => set({ program_id: v || undefined })">
            <option value="">{{ t('tracking.anyProgram') }}</option>
            <option v-for="p in programs" :key="p.id" :value="p.id">{{ p.name }}</option>
          </NativeSelect>
        </label>
        <div class="relative flex flex-col gap-1 text-sm">
          <label for="tracking-search">{{ t('tracking.projects') }}</label>
          <Input id="tracking-search" v-model="search" type="search" :placeholder="t('tracking.searchProject')" data-testid="tracking-search" />
          <ul v-if="q && found?.items.length" class="absolute top-full z-10 mt-1 w-full rounded-md border bg-background shadow" role="listbox">
            <li v-for="p in found.items" :key="p.id">
              <button type="button" class="w-full px-2 py-1.5 text-left hover:bg-muted" data-testid="tracking-found" @click="addProject(p.id, p.name)">
                <span class="font-mono text-xs text-muted-foreground">{{ p.identifier }}</span> {{ p.name }}
              </button>
            </li>
          </ul>
        </div>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('tracking.owners') }}
          <NativeSelect v-model="ownerChoice" data-testid="tracking-owner">
            <option value="">{{ t('tracking.addOwner') }}</option>
            <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
          </NativeSelect>
        </label>
      </div>
      <div v-if="any" class="flex flex-wrap items-center gap-2">
        <span v-for="id in filters.project_id" :key="id" class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs" data-testid="tracking-chip">
          {{ names.get(id) ?? id }}
          <button type="button" :aria-label="t('tracking.remove', { name: names.get(id) ?? id })" @click="set({ project_id: filters.project_id!.filter((x) => x !== id) })">
            <X class="size-3" aria-hidden="true" />
          </button>
        </span>
        <span v-for="id in filters.owner_id" :key="id" class="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs" data-testid="tracking-chip">
          {{ t('tracking.ownerChip', { name: ownerName(id) }) }}
          <button type="button" :aria-label="t('tracking.remove', { name: ownerName(id) })" @click="set({ owner_id: filters.owner_id!.filter((x) => x !== id) })">
            <X class="size-3" aria-hidden="true" />
          </button>
        </span>
        <Button size="sm" variant="ghost" data-testid="tracking-clear" @click="clear">{{ t('tracking.clear') }}</Button>
      </div>
      <p v-else-if="data" class="text-xs text-muted-foreground" data-testid="tracking-default">{{ data.mine ? t('tracking.mine') : t('tracking.all') }}</p>
    </section>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <template v-if="data">
      <Alert v-if="data.matched > data.projects.length" role="status" data-testid="tracking-limit">
        <AlertDescription>{{ t('tracking.limit', { shown: data.projects.length, left: data.matched - data.projects.length }) }}</AlertDescription>
      </Alert>
      <div v-if="!data.projects.length" class="flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-sm text-muted-foreground" data-testid="tracking-empty">
        {{ t('tracking.empty') }}
        <Button v-if="any" size="sm" variant="outline" @click="clear">{{ t('tracking.clear') }}</Button>
      </div>
      <template v-else>
        <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('overview.indicators')" data-testid="tracking-summary">
          <article class="rounded-xl border bg-background p-4">
            <h2 class="text-sm text-muted-foreground">{{ t('tracking.projectsCount') }}</h2>
            <p class="text-3xl font-bold tabular-nums">{{ data.summary.projects }}</p>
          </article>
          <article class="rounded-xl border bg-background p-4">
            <h2 class="text-sm text-muted-foreground">{{ t('tracking.milestonesCount') }}</h2>
            <p class="text-3xl font-bold tabular-nums">{{ data.summary.milestones }}</p>
            <p class="text-xs text-muted-foreground">{{ t('tracking.met', { n: data.summary.met }) }}</p>
          </article>
          <article class="rounded-xl border bg-background p-4">
            <h2 class="text-sm text-muted-foreground">{{ t('tracking.real') }}</h2>
            <p class="text-3xl font-bold tabular-nums">{{ pct(data.summary.figures.real) }}</p>
          </article>
          <article class="flex flex-col gap-1 rounded-xl border bg-background p-4">
            <h2 class="text-sm text-muted-foreground">{{ t('tracking.plan') }}</h2>
            <p class="text-3xl font-bold tabular-nums">{{ data.summary.figures.has_plan ? pct(data.summary.figures.plan) : t('schedule.noPlan') }}</p>
            <LevelBadge :figures="data.summary.figures" compact />
          </article>
        </section>
        <MilestoneGantt :projects="data.projects" :today="data.today" />
        <TaskColumns :data="data" />
      </template>
    </template>
  </div>
</template>

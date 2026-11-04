<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { CalendarRange, Columns3, Table2 } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { getBoard, type ListViewConfig, type Mode } from '@/api/projectBoard'
import { LEVELS } from '@/api/schedule'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import ListViewsBar from '@/features/projectviews/ListViewsBar.vue'
import ProjectBoard from '@/features/projectviews/ProjectBoard.vue'
import ProjectGantt from '@/features/projectviews/ProjectGantt.vue'
import ProjectTable from '@/features/projectviews/ProjectTable.vue'
import { COLUMNS, END_VALUES, FIELDS, GROUPS, filter, kindOf, sort, type Columns, type Group } from '@/features/projectviews/filters'
import { useListState } from '@/features/projectviews/useListState'
import { useLevels } from '@/features/schedule/useLevels'
import FilterBar from '@/features/views/FilterBar.vue'
import type { FilterOption } from '@/features/views/useFilterOptions'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'
import { HEALTHS } from '@/types/work'

// Every project in a Kanban by status, a Timeline or a Table, with the same
// filters and saved views (spec 012, RF-27 to RF-32, RF-62 to RF-79).
const { t, locale } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const levels = useLevels()
const { catalog } = useCatalog()
const { users, teams } = usePeople()
const state = useListState()

const { data, error } = useQuery({ queryKey: ['project-board'], queryFn: getBoard, placeholderData: keepPreviousData })
const statuses = computed(() => catalog.value.project_statuses)
const statusOrder = computed(() => statuses.value.map((s) => s.id))
const columns = computed<Columns>(() => (state.config.value.columns as Columns) || 'status')
const group = computed<Group>(() => (state.config.value.group as Group) || 'program')

// What each filter can take and the name of each value (RF-62 to RF-64, RF-78).
const options = computed<Record<string, FilterOption[]>>(() => ({
  status_id: statuses.value.filter((s) => !s.retired).map((s) => ({ value: s.id, label: s.name, color: s.color })),
  level: [...LEVELS.map((l) => ({ value: l, label: levels.name(l) })), { value: 'none', label: t('schedule.noPlan') }],
  health: HEALTHS.map((h) => ({ value: h, label: t(`projects.healths.${h}`) })),
  program_id: [...(data.value?.programs ?? []).map((p) => ({ value: p.id, label: p.name })), { value: 'none', label: t('projectViews.noProgram') }],
  team_id: teams.value.map(({ team, depth }) => ({ value: team.id, label: '  '.repeat(depth) + team.name })),
  owner_id: [{ value: 'me', label: t('projectViews.me') }, ...users.value.map((u) => ({ value: u.id, label: u.name }))],
  end: END_VALUES.map((v) => ({ value: v, label: t(`projectViews.end.${v}`) })),
}))
const loaded = computed(() => !!data.value && users.value.length > 0 && catalog.value.project_statuses.length > 0)
function available(field: string, value: string) {
  if (!loaded.value) return true
  switch (field) {
    case 'status_id':
    case 'program_id':
    case 'team_id':
    case 'owner_id':
      return (options.value[field] ?? []).some((o) => o.value === value)
    default:
      return true
  }
}
const fmt = (d: string) => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z'))
function label(field: string, value: string) {
  if (kindOf(field) === 'date') return fmt(value)
  if (field === 'q') return value
  return options.value[field]?.find((o) => o.value === value)?.label.trim() ?? value
}

const ctx = computed(() => ({ me: auth.me?.id ?? '', today: data.value?.today ?? '' }))
const all = computed(() => data.value?.projects ?? [])
const count = (c: ListViewConfig | null) => (c ? filter(all.value, c.filters, ctx.value, available).length : all.value.length)
const rows = computed(() => sort(filter(all.value, state.filters.value, ctx.value, available), state.config.value.sort, statusOrder.value))

const modes: { key: Mode; icon: typeof Columns3 }[] = [
  { key: 'kanban', icon: Columns3 },
  { key: 'timeline', icon: CalendarRange },
  { key: 'table', icon: Table2 },
]
const canCreate = computed(() => auth.can('projects:create:all', 'projects:create:team'))
const sorts = ['gap', 'end', 'name', '-real']
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="projects">
    <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <h1 class="font-heading text-xl font-semibold">{{ t('projects.title') }}</h1>
      <nav :aria-label="t('projectViews.mode')" class="flex gap-0.5 rounded-lg bg-muted p-0.5">
        <button
          v-for="m in modes"
          :key="m.key"
          type="button"
          class="inline-flex h-8 items-center gap-1.5 rounded-md px-3 text-sm"
          :class="state.mode.value === m.key ? 'bg-background font-semibold shadow-xs' : 'text-muted-foreground hover:text-foreground'"
          :aria-pressed="state.mode.value === m.key"
          :data-testid="`mode-${m.key}`"
          @click="state.apply({ mode: m.key })"
        >
          <component :is="m.icon" class="size-4" aria-hidden="true" />{{ t(`projectViews.modes.${m.key}`) }}
        </button>
      </nav>
      <span class="flex-1" />
      <Button v-if="canCreate" size="sm" variant="outline" as-child>
        <RouterLink to="/projects/new">{{ auth.can('templates:use') ? t('projects.fromTemplate') : t('projects.new') }}</RouterLink>
      </Button>
    </div>

    <ListViewsBar
      :view-id="state.viewId.value"
      :config="state.config.value"
      :count="count"
      :can-share="auth.can('project_views:share')"
      @open="state.openView"
    />

    <FilterBar
      :filters="state.filters.value"
      :options="options"
      :label="label"
      :available="available"
      :fields="FIELDS"
      :kind-of="kindOf"
      labels="projectViews.fields"
      @set="state.setValues"
      @remove="state.removeValue"
    >
      <span class="flex-1" />
      <label v-if="state.mode.value === 'kanban'" class="flex items-center gap-2 text-sm text-muted-foreground">
        {{ t('projectViews.columnsBy') }}
        <NativeSelect :model-value="columns" class="h-8 w-44" data-testid="board-columns" @update:model-value="(v) => state.apply({ columns: v === 'status' ? '' : v })">
          <option v-for="c in COLUMNS" :key="c" :value="c">{{ t(`projectViews.columns.${c}`) }}</option>
        </NativeSelect>
      </label>
      <label v-if="state.mode.value === 'timeline'" class="flex items-center gap-2 text-sm text-muted-foreground">
        {{ t('projectViews.groupBy') }}
        <NativeSelect :model-value="group" class="h-8 w-44" data-testid="gantt-group-by" @update:model-value="(v) => state.apply({ group: v === 'program' ? '' : v })">
          <option v-for="g in GROUPS" :key="g" :value="g">{{ t(`projectViews.groups.${g}`) }}</option>
        </NativeSelect>
      </label>
      <label v-if="state.mode.value !== 'table'" class="flex items-center gap-2 text-sm text-muted-foreground">
        {{ t('projectViews.order') }}
        <NativeSelect
          :model-value="state.config.value.sort || 'gap'"
          class="h-8 w-56"
          data-testid="board-sort"
          @update:model-value="(v) => state.apply({ sort: v === 'gap' ? '' : v })"
        >
          <option v-for="s in sorts" :key="s" :value="s">{{ t(`projectViews.sorts.${s}`) }}</option>
        </NativeSelect>
      </label>
    </FilterBar>
    <p v-if="!Object.keys(state.filters.value).length" class="-mt-2 text-xs text-muted-foreground">{{ t('projectViews.noFilters') }}</p>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <template v-if="data">
      <ProjectBoard v-if="state.mode.value === 'kanban'" :rows="rows" :columns="columns" :statuses="statuses" />
      <ProjectGantt v-else-if="state.mode.value === 'timeline'" :rows="rows" :today="data.today" :group="group" :programs="data.programs" :statuses="statuses" />
      <ProjectTable v-else :rows="rows" :statuses="statuses" :sort="state.config.value.sort" @sort="(s) => state.apply({ sort: s === 'gap' ? '' : s })" />
    </template>
  </div>
</template>

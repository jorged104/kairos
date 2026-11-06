<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import { getProject } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import ProjectKanban from '@/features/kanban/ProjectKanban.vue'
import MeetingsTab from '@/features/meetings/MeetingsTab.vue'
import ProjectHistory from '@/features/projects/ProjectHistory.vue'
import ProjectMilestones from '@/features/projects/ProjectMilestones.vue'
import ProjectOverview from '@/features/projects/ProjectOverview.vue'
import ProjectPhases from '@/features/projects/ProjectPhases.vue'
import ProjectSummary from '@/features/projects/ProjectSummary.vue'
import ProjectTeam from '@/features/projects/ProjectTeam.vue'
import ProjectWorkItems from '@/features/projects/ProjectWorkItems.vue'
import ReportsTab from '@/features/reports/ReportsTab.vue'
import ProjectTimeline from '@/features/timeline/ProjectTimeline.vue'
import FilterBar from '@/features/views/FilterBar.vue'
import { useFilterOptions } from '@/features/views/useFilterOptions'
import { useProjectFilters } from '@/features/views/useProjectFilters'
import ViewsBar from '@/features/views/ViewsBar.vue'
import { healthVariant, lifecycleVariant } from '@/features/work/Badges'

// Project page (spec 008, RF-1 to RF-5): overview, table, kanban, timeline
// and team, then the milestones, phases and data of spec 004 (RF-136); the
// project's page in History and its data in Config.
const props = defineProps<{ id: string }>()
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const { data: project, error: loadError } = useQuery({
  queryKey: computed(() => ['project', props.id]),
  queryFn: () => getProject(props.id),
})

const ALL_TABS = ['overview', 'table', 'kanban', 'timeline', 'team', 'milestones', 'meetings', 'reports', 'phases', 'history', 'config'] as const
type Tab = (typeof ALL_TABS)[number]
// The team only for who manages it (RF-4).
const TABS = computed(() => ALL_TABS.filter((x) => x !== 'team' || project.value?.can.manage_team))
// The tabs before spec 008 keep their links.
const OLD: Record<string, Tab> = { summary: 'config', data: 'config', 'work-items': 'table' }
const tab = computed<Tab>(() => {
  const q = route.query.tab as string
  const x = (OLD[q] ?? q) as Tab
  return TABS.value.includes(x) ? x : 'overview'
})
const tabLabel = (x: Tab) => t(`projects.tabs.${x}`)
const working = computed(() => tab.value === 'table' || tab.value === 'kanban' || tab.value === 'timeline')

// Filters and views of the table, the kanban and the timeline (RF-18 to RF-32).
const views = useProjectFilters(() => props.id)
const filterOptions = useFilterOptions(() => project.value)
// A value no longer available stays as a chip but doesn't filter (RF-31).
const effective = computed(() =>
  Object.fromEntries(
    Object.entries(views.filters.value)
      .map(([k, vs]) => [k, vs.filter((v) => filterOptions.available(k, v))] as const)
      .filter(([, vs]) => vs.length),
  ),
)

const error = ref<unknown>(null)
watch(tab, () => (error.value = null))
const conflict = computed(() => error.value instanceof ApiError && error.value.code === 'version_conflict')
function reload() {
  error.value = null
  return qc.invalidateQueries({ queryKey: ['project', props.id] })
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>
    <template v-if="project">
      <div class="flex flex-wrap items-center gap-2">
        <span class="font-mono text-sm text-muted-foreground">{{ project.identifier }}</span>
        <h1 class="font-heading text-xl font-semibold">{{ project.name }}</h1>
        <Badge :variant="lifecycleVariant[project.lifecycle]">{{ t(`projects.lifecycles.${project.lifecycle}`) }}</Badge>
        <Badge :variant="healthVariant[project.health]">{{ t(`projects.healths.${project.health}`) }}</Badge>
        <Badge v-if="project.is_template" variant="outline">{{ t('projects.isTemplate') }}</Badge>
        <RouterLink v-if="project.parent" :to="`/projects/${project.parent.id}`" class="text-sm text-muted-foreground hover:underline">
          ↑ {{ project.parent.identifier }} · {{ project.parent.name }}
        </RouterLink>
      </div>

      <nav class="flex gap-1 border-b text-sm" role="tablist">
        <button
          v-for="x in TABS"
          :key="x"
          type="button"
          role="tab"
          :aria-selected="tab === x"
          class="-mb-px border-b-2 px-3 py-2"
          :class="tab === x ? 'border-primary font-medium' : 'border-transparent text-muted-foreground hover:text-foreground'"
          :data-testid="`tab-${x}`"
          @click="router.replace({ query: { ...(working && ['table', 'kanban', 'timeline'].includes(x) ? route.query : {}), tab: x, item: undefined, meeting: undefined, report: undefined, edit: undefined } })"
        >
          {{ tabLabel(x) }}
        </button>
      </nav>

      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription class="flex flex-wrap items-center gap-2">
          {{ errorMessage(error) }}
          <Button v-if="conflict" size="sm" variant="outline" @click="reload">{{ t('work.reload') }}</Button>
        </AlertDescription>
      </Alert>

      <template v-if="working">
        <ViewsBar
          :project-id="project.id"
          :view-id="views.viewId.value"
          :config="views.config.value"
          :can-share="!!project.can.manage_team"
          @open="views.openView"
        />
        <FilterBar
          :filters="views.filters.value"
          :options="filterOptions.options.value"
          :label="filterOptions.label"
          :available="filterOptions.available"
          @set="views.setValues"
          @remove="views.removeValue"
        />
      </template>

      <ProjectOverview v-if="tab === 'overview'" :project="project" />
      <ProjectWorkItems
        v-else-if="tab === 'table'"
        :project="project"
        :filters="effective"
        :sort="views.sort.value"
        :group="views.group.value"
        @arrange="views.apply"
      />
      <ProjectKanban v-else-if="tab === 'kanban'" :project="project" :filters="effective" />
      <ProjectTimeline v-else-if="tab === 'timeline'" :project="project" :filters="effective" />
      <ProjectTeam v-else-if="tab === 'team'" :project="project" />
      <ProjectMilestones v-else-if="tab === 'milestones'" :project="project" />
      <MeetingsTab v-else-if="tab === 'meetings'" :project="project" />
      <ReportsTab v-else-if="tab === 'reports'" :project="project" />
      <ProjectPhases v-else-if="tab === 'phases'" :project="project" @error="error = $event" />
      <ProjectHistory v-else-if="tab === 'history'" :project="project" />
      <ProjectSummary v-else :project="project" @error="error = $event" />
    </template>
  </div>
</template>

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
import ProjectMilestones from '@/features/projects/ProjectMilestones.vue'
import ProjectPhases from '@/features/projects/ProjectPhases.vue'
import ProjectSummary from '@/features/projects/ProjectSummary.vue'
import ProjectWorkItems from '@/features/projects/ProjectWorkItems.vue'
import { healthVariant, lifecycleVariant } from '@/features/work/Badges'

// Project page: summary, phases, milestones and work items (RF-136).
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

const TABS = ['summary', 'phases', 'milestones', 'work-items'] as const
type Tab = (typeof TABS)[number]
const tab = computed<Tab>(() => (TABS.includes(route.query.tab as Tab) ? (route.query.tab as Tab) : 'summary'))
const tabLabel = (x: Tab) => t(`projects.tabs.${x}`)

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
          @click="router.replace({ query: { tab: x } })"
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

      <ProjectSummary v-if="tab === 'summary'" :project="project" @error="error = $event" />
      <ProjectPhases v-else-if="tab === 'phases'" :project="project" @error="error = $event" />
      <ProjectMilestones v-else-if="tab === 'milestones'" :project="project" @error="error = $event" />
      <ProjectWorkItems v-else :project="project" @error="error = $event" />
    </template>
  </div>
</template>

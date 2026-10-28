<script setup lang="ts">
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/vue-query'
import { refDebounced } from '@vueuse/core'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { createProject, getProject, listProjects, updateProject } from '@/api/work'
import { EditableCell, GridTable, QuickAddRow, type CellOption } from '@/components/data-table'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { healthVariant, lifecycleVariant } from '@/features/work/Badges'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'
import { HEALTHS, LIFECYCLES, type ProjectSummary } from '@/types/work'

// Project list with search and filters (RF-128).
const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const { teams, users, ownTeams } = usePeople()

const search = ref('')
const debounced = refDebounced(search, 300)
const teamId = ref('')
const lifecycle = ref('')
const health = ref('')
const page = ref(1)
watch([debounced, teamId, lifecycle, health], () => (page.value = 1))

const query = computed(() => ({
  q: debounced.value,
  team_id: teamId.value,
  lifecycle: lifecycle.value,
  health: health.value,
  page: page.value,
}))
const { data, error } = useQuery({
  queryKey: ['projects', query],
  queryFn: () => listProjects(query.value),
  placeholderData: keepPreviousData,
})
const pages = computed(() => (data.value ? Math.max(1, Math.ceil(data.value.total / data.value.page_size)) : 1))

// Rows are edited in place and new projects come from the last row (RF-139, RF-140).
const qc = useQueryClient()
const { catalog } = useCatalog()
const canCreate = computed(() => auth.can('projects:create:all', 'projects:create:team'))
const refresh = () => qc.invalidateQueries({ queryKey: ['projects'] })

const draft = ref({ identifier: '', teamId: '', methodologyId: '' })
watch(ownTeams, (list) => {
  if (!draft.value.teamId && list.length) draft.value.teamId = list[0]!.team.id
}, { immediate: true })
async function create(name: string) {
  const d = draft.value
  await createProject({ identifier: d.identifier.toUpperCase(), name, team_id: d.teamId, methodology_id: d.methodologyId || undefined })
  draft.value = { ...d, identifier: '' }
  await refresh()
}

// The PATCH replaces every editable field, so the rest is read fresh.
type Field = 'name' | 'owner_id' | 'health' | 'start_date' | 'end_date'
function saver(p: ProjectSummary, field: Field) {
  return async (value: string) => {
    const cur = await getProject(p.id)
    await updateProject(p.id, {
      version: cur.version, name: cur.name, description: cur.description, team_id: cur.team.id, owner_id: cur.owner.id,
      parent_id: cur.parent?.id ?? null, start_date: cur.start_date, end_date: cur.end_date, health: cur.health,
      health_explanation: cur.health_explanation, custom_values: cur.custom_values,
      [field]: field === 'start_date' || field === 'end_date' ? value || null : value,
    })
    qc.invalidateQueries({ queryKey: ['project', p.id] })
    await refresh()
  }
}
const editable = (p: ProjectSummary) => p.can.manage && p.lifecycle === 'activo'
const userOptions = computed<CellOption[]>(() => users.value.map((u) => ({ value: u.id, label: u.name })))
const healthOptions = computed<CellOption[]>(() => HEALTHS.map((h) => ({ value: h, label: t(`projects.healths.${h}`) })))
const methodologies = computed(() => catalog.value.methodologies.filter((m) => !m.retired))
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2">
      <h1 class="font-heading text-xl font-semibold">{{ t('projects.title') }}</h1>
      <Button v-if="canCreate && auth.can('templates:use')" variant="outline" as-child>
        <RouterLink to="/projects/new">{{ t('projects.fromTemplate') }}</RouterLink>
      </Button>
    </div>

    <div class="grid gap-3 rounded-lg border bg-background p-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="flex flex-col gap-1.5">
        <Label for="project-search">{{ t('common.search') }}</Label>
        <Input id="project-search" v-model="search" :placeholder="t('projects.searchPlaceholder')" />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="project-team">{{ t('projects.team') }}</Label>
        <NativeSelect id="project-team" v-model="teamId">
          <option value="">{{ t('common.all') }}</option>
          <option v-for="{ team, depth } in teams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="project-lifecycle">{{ t('projects.lifecycle') }}</Label>
        <NativeSelect id="project-lifecycle" v-model="lifecycle">
          <option value="">{{ t('projects.lifecycleDefault') }}</option>
          <option v-for="l in LIFECYCLES" :key="l" :value="l">{{ t(`projects.lifecycles.${l}`) }}</option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="project-health">{{ t('projects.health') }}</Label>
        <NativeSelect id="project-health" v-model="health">
          <option value="">{{ t('common.all') }}</option>
          <option v-for="h in HEALTHS" :key="h" :value="h">{{ t(`projects.healths.${h}`) }}</option>
        </NativeSelect>
      </div>
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <GridTable :empty="!canCreate && data?.items.length === 0">
      <thead>
        <tr>
          <th class="w-24">{{ t('projects.identifier') }}</th>
          <th>{{ t('projects.name') }}</th>
          <th class="w-36">{{ t('projects.team') }}</th>
          <th class="w-44">{{ t('projects.owner') }}</th>
          <th class="w-36">{{ t('projects.health') }}</th>
          <th class="w-28">{{ t('projects.lifecycle') }}</th>
          <th class="w-36">{{ t('projects.startDate') }}</th>
          <th class="w-36">{{ t('projects.endDate') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in data?.items ?? []" :key="p.id" class="hover:bg-muted/30" data-testid="project-row">
          <td class="px-2 font-mono text-xs text-muted-foreground">{{ p.identifier }}</td>
          <EditableCell :value="p.name" :label="t('projects.name')" :save="saver(p, 'name')" :editable="editable(p)" required>
            <RouterLink :to="`/projects/${p.id}`" class="font-medium hover:underline" @click.stop>{{ p.name }}</RouterLink>
            <span v-if="p.methodology" class="text-xs text-muted-foreground">{{ p.methodology.name }}</span>
          </EditableCell>
          <td class="px-2">{{ p.team.name }}</td>
          <EditableCell
            kind="select"
            :value="p.owner.id"
            :label="t('projects.owner')"
            :options="userOptions"
            :save="saver(p, 'owner_id')"
            :editable="editable(p)"
            required
          >
            {{ p.owner.name }}
          </EditableCell>
          <EditableCell
            kind="select"
            :value="p.health"
            :label="t('projects.health')"
            :options="healthOptions"
            :save="saver(p, 'health')"
            :editable="editable(p)"
            required
          >
            <Badge :variant="healthVariant[p.health]">{{ t(`projects.healths.${p.health}`) }}</Badge>
          </EditableCell>
          <td class="px-2"><Badge :variant="lifecycleVariant[p.lifecycle]">{{ t(`projects.lifecycles.${p.lifecycle}`) }}</Badge></td>
          <EditableCell kind="date" :value="p.start_date ?? ''" :label="t('projects.startDate')" :save="saver(p, 'start_date')" :editable="editable(p)" />
          <EditableCell kind="date" :value="p.end_date ?? ''" :label="t('projects.endDate')" :save="saver(p, 'end_date')" :editable="editable(p)" />
        </tr>
        <QuickAddRow v-if="canCreate" :colspan="8" :label="t('projects.new')" :placeholder="t('projects.newPlaceholder')" :submit="create">
          <Input
            v-model="draft.identifier"
            class="h-8 w-28 font-mono uppercase"
            :placeholder="t('projects.identifier')"
            :aria-label="t('projects.identifier')"
            :title="t('projects.identifierHelp')"
            maxlength="10"
            pattern="[A-Za-z][A-Za-z0-9]{1,9}"
            required
          />
          <NativeSelect v-model="draft.teamId" class="w-40" :aria-label="t('projects.team')" required>
            <option v-for="{ team, depth } in ownTeams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
          </NativeSelect>
          <NativeSelect v-model="draft.methodologyId" class="w-40" :aria-label="t('projects.methodology')">
            <option value="">{{ t('projects.noMethodology') }}</option>
            <option v-for="m in methodologies" :key="m.id" :value="m.id">{{ m.name }}</option>
          </NativeSelect>
        </QuickAddRow>
      </tbody>
    </GridTable>

    <div v-if="data" class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pageOf', { page: data.page, pages, total: data.total }) }}</span>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" :disabled="page <= 1" @click="page--">{{ t('common.previous') }}</Button>
        <Button variant="outline" size="sm" :disabled="page >= pages" @click="page++">{{ t('common.next') }}</Button>
      </div>
    </div>
  </div>
</template>

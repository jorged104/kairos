<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { ArrowDown, ArrowUp } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { BoardProject } from '@/api/projectBoard'
import { createProject, getProject, updateProject } from '@/api/work'
import { EditableCell, GridTable, QuickAddRow, type CellOption } from '@/components/data-table'
import NativeSelect from '@/components/NativeSelect.vue'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import { healthVariant } from '@/features/work/Badges'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'
import { HEALTHS, type ProjectStatus } from '@/types/work'
import type { TABLE_SORTS } from './filters'

// The table of projects: edited in place, a quick row to add, and the
// status, program and figures of each (spec 012, RF-60, RF-61; spec 004,
// RF-139, RF-140).
const props = defineProps<{ rows: BoardProject[]; statuses: ProjectStatus[]; sort: string }>()
const emit = defineEmits<{ sort: [string] }>()
const { t } = useI18n()
const qc = useQueryClient()
const auth = useAuthStore()
const { users, ownTeams } = usePeople()
const { catalog } = useCatalog()

const refresh = () => qc.invalidateQueries({ queryKey: ['project-board'] })
const canCreate = computed(() => auth.can('projects:create:all', 'projects:create:team'))
const draft = ref({ identifier: '', teamId: '', methodologyId: '' })
watch(
  ownTeams,
  (list) => {
    if (!draft.value.teamId && list.length) draft.value.teamId = list[0]!.team.id
  },
  { immediate: true },
)
async function create(name: string) {
  const d = draft.value
  await createProject({ identifier: d.identifier.toUpperCase(), name, team_id: d.teamId, methodology_id: d.methodologyId || undefined })
  draft.value = { ...d, identifier: '' }
  await refresh()
}

// The PATCH replaces every editable field, so the rest is read fresh.
type Field = 'name' | 'owner_id' | 'health' | 'start_date' | 'end_date'
function saver(p: BoardProject, field: Field) {
  return async (value: string) => {
    const cur = await getProject(p.id)
    await updateProject(p.id, {
      version: cur.version, name: cur.name, team_id: cur.team.id, owner_id: cur.owner.id,
      parent_id: cur.parent?.id ?? null, start_date: cur.start_date, end_date: cur.end_date, health: cur.health,
      health_explanation: cur.health_explanation, custom_values: cur.custom_values,
      [field]: field === 'start_date' || field === 'end_date' ? value || null : value,
    })
    qc.invalidateQueries({ queryKey: ['project', p.id] })
    await refresh()
  }
}
const editable = (p: BoardProject) => p.can_manage && p.lifecycle === 'activo'
const userOptions = computed<CellOption[]>(() => users.value.map((u) => ({ value: u.id, label: u.name })))
const healthOptions = computed<CellOption[]>(() => HEALTHS.map((h) => ({ value: h, label: t(`projects.healths.${h}`) })))
const methodologies = computed(() => catalog.value.methodologies.filter((m) => !m.retired))
const statusOf = (p: BoardProject) => props.statuses.find((s) => s.id === p.status_id)

type Key = (typeof TABLE_SORTS)[number]
const key = computed(() => (props.sort.replace(/^-/, '') || 'gap') as Key)
const desc = computed(() => props.sort.startsWith('-'))
const by = (k: Key) => emit('sort', key.value === k && !desc.value ? `-${k}` : k)
const aria = (k: Key) => (key.value === k ? (desc.value ? 'descending' : 'ascending') : 'none')
const columns: { key: Key; label: string; class: string }[] = [
  { key: 'identifier', label: 'projects.identifier', class: 'w-24' },
  { key: 'name', label: 'projects.name', class: '' },
  { key: 'status', label: 'projects.status', class: 'w-36' },
  { key: 'program', label: 'portfolio.program', class: 'w-40' },
  { key: 'owner', label: 'projects.owner', class: 'w-44' },
  { key: 'health', label: 'projects.health', class: 'w-36' },
  { key: 'real', label: 'portfolio.columns.real', class: 'w-20 text-right' },
  { key: 'plan', label: 'portfolio.columns.plan', class: 'w-20 text-right' },
  { key: 'gap', label: 'portfolio.columns.level', class: 'w-40' },
  { key: 'start', label: 'projects.startDate', class: 'w-36' },
  { key: 'end', label: 'projects.endDate', class: 'w-36' },
]
const pct = (n: number) => `${n.toLocaleString('es', { maximumFractionDigits: 1 })} %`
</script>

<template>
  <GridTable :empty="!canCreate && rows.length === 0" data-testid="project-table">
    <thead>
      <tr>
        <th v-for="c in columns" :key="c.key" :class="c.class" :aria-sort="aria(c.key)">
          <button type="button" class="inline-flex items-center gap-1 hover:text-foreground" :data-testid="`table-sort-${c.key}`" @click="by(c.key)">
            {{ t(c.label) }}
            <component :is="desc ? ArrowDown : ArrowUp" v-if="key === c.key" class="size-3" aria-hidden="true" />
          </button>
        </th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="p in rows" :key="p.id" class="hover:bg-muted/30" data-testid="project-row">
        <td class="px-2 font-mono text-xs text-muted-foreground">{{ p.identifier }}</td>
        <EditableCell :value="p.name" :label="t('projects.name')" :save="saver(p, 'name')" :editable="editable(p)" required>
          <RouterLink :to="`/projects/${p.id}`" class="font-medium hover:underline" @click.stop>{{ p.name }}</RouterLink>
        </EditableCell>
        <td class="px-2">
          <span class="inline-flex items-center gap-1.5">
            <span class="size-2 rounded-full" :style="{ background: statusOf(p)?.color }" aria-hidden="true" />{{ statusOf(p)?.name }}
          </span>
        </td>
        <td class="truncate px-2 text-muted-foreground">{{ p.program?.name ?? t('common.none') }}</td>
        <EditableCell kind="select" :value="p.owner.id" :label="t('projects.owner')" :options="userOptions" :save="saver(p, 'owner_id')" :editable="editable(p)" required>
          {{ p.owner.name }}
        </EditableCell>
        <EditableCell kind="select" :value="p.health" :label="t('projects.health')" :options="healthOptions" :save="saver(p, 'health')" :editable="editable(p)" required>
          <Badge :variant="healthVariant[p.health]">{{ t(`projects.healths.${p.health}`) }}</Badge>
        </EditableCell>
        <td class="px-2 text-right tabular-nums">{{ pct(p.figures.real) }}</td>
        <td class="px-2 text-right text-muted-foreground tabular-nums">{{ p.figures.has_plan ? pct(p.figures.plan) : '—' }}</td>
        <td class="px-2">
          <LevelBadge v-if="p.figures.has_plan" :figures="p.figures" compact />
          <span v-else class="text-xs text-muted-foreground">{{ t('schedule.noPlan') }}</span>
        </td>
        <EditableCell kind="date" :value="p.start_date ?? ''" :label="t('projects.startDate')" :save="saver(p, 'start_date')" :editable="editable(p)" />
        <EditableCell kind="date" :value="p.end_date ?? ''" :label="t('projects.endDate')" :save="saver(p, 'end_date')" :editable="editable(p)" />
      </tr>
      <QuickAddRow v-if="canCreate" :colspan="columns.length" :label="t('projects.new')" :placeholder="t('projects.newPlaceholder')" :submit="create">
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
</template>

<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { createMilestone, getMilestone, listMilestones, listPhases, updateMilestone, type MilestoneInput } from '@/api/work'
import { EditableCell, GridTable, QuickAddRow, type CellOption } from '@/components/data-table'
import NativeSelect from '@/components/NativeSelect.vue'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { milestoneVariant } from '@/features/work/Badges'
import ProgressBar from '@/features/work/ProgressBar.vue'
import type { Milestone, Project } from '@/types/work'

// Milestones of a project with their progress (RF-44 to RF-56), edited in
// the table and created from its last row (RF-139, RF-140).
const props = defineProps<{ project: Project }>()
const { t } = useI18n()
const qc = useQueryClient()

const { data: milestones } = useQuery({ queryKey: ['milestones', props.project.id], queryFn: () => listMilestones(props.project.id) })
const { data: phases } = useQuery({ queryKey: ['phases', props.project.id], queryFn: () => listPhases(props.project.id) })
const phaseOptions = computed<CellOption[]>(() => (phases.value ?? []).map((ph) => ({ value: ph.id, label: ph.name })))

const active = computed(() => props.project.lifecycle === 'activo')
const canCreate = computed(() => props.project.can.manage && active.value)
const refresh = () => qc.invalidateQueries({ queryKey: ['milestones', props.project.id] })

const draft = ref({ due: '', phase: '' })
async function create(name: string) {
  await createMilestone(props.project.id, { name, due_date: draft.value.due || null, phase_id: draft.value.phase || null })
  draft.value = { due: '', phase: '' }
  await refresh()
}

// The PATCH replaces every editable field, so the rest is read fresh.
type Field = 'name' | 'due_date' | 'phase_id'
function saver(m: Milestone, field: Field) {
  return async (value: string) => {
    const cur = await getMilestone(m.id)
    const input: MilestoneInput = {
      version: cur.version, name: cur.name, due_date: cur.due_date,
      phase_id: cur.phase?.id ?? null, position: cur.position,
    }
    await updateMilestone(m.id, { ...input, [field]: field === 'name' ? value : value || null })
    qc.invalidateQueries({ queryKey: ['milestone', m.id] })
    await refresh()
  }
}
const editable = (m: Milestone) => m.can.manage && active.value
</script>

<template>
  <GridTable :empty="!canCreate && milestones?.length === 0">
    <thead>
      <tr>
        <th>{{ t('milestones.name') }}</th>
        <th class="w-32">{{ t('milestones.status') }}</th>
        <th class="w-44">{{ t('milestones.phase') }}</th>
        <th class="w-40">{{ t('milestones.due') }}</th>
        <th class="w-56">{{ t('milestones.progress') }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="m in milestones ?? []" :key="m.id" class="hover:bg-muted/30" data-testid="milestone-row">
        <EditableCell :value="m.name" :label="t('milestones.name')" :save="saver(m, 'name')" :editable="editable(m)" required>
          <RouterLink :to="`/milestones/${m.id}`" class="font-medium hover:underline" @click.stop>{{ m.name }}</RouterLink>
        </EditableCell>
        <td class="bg-muted/20 px-2">
          <span class="flex flex-wrap gap-1">
            <Badge :variant="milestoneVariant[m.status]">{{ t(`milestones.statuses.${m.status}`) }}</Badge>
            <Badge v-if="m.overdue" variant="destructive">{{ t('milestones.overdue') }}</Badge>
          </span>
        </td>
        <EditableCell
          kind="select"
          :value="m.phase?.id ?? ''"
          :label="t('milestones.phase')"
          :options="phaseOptions"
          :save="saver(m, 'phase_id')"
          :editable="editable(m) && phaseOptions.length > 0"
        >
          <span :class="!m.phase && 'text-muted-foreground'">{{ m.phase?.name ?? t('milestones.noPhase') }}</span>
        </EditableCell>
        <EditableCell kind="date" :value="m.due_date ?? ''" :label="t('milestones.due')" :save="saver(m, 'due_date')" :editable="editable(m)">
          <span v-if="m.completed_on">{{ t('milestones.completedOn') }} {{ m.completed_on }}</span>
          <span v-else :class="!m.due_date && 'text-muted-foreground'">{{ m.due_date ?? t('common.none') }}</span>
        </EditableCell>
        <td class="bg-muted/20 px-2 py-1">
          <div class="flex flex-col gap-0.5 text-xs text-muted-foreground">
            <ProgressBar :value="m.progress.percent" :label="t('milestones.progress')" />
            {{ t('milestones.counts', { closed: m.progress.closed, total: m.progress.total }) }}
          </div>
        </td>
      </tr>
      <QuickAddRow v-if="canCreate" :colspan="5" :label="t('milestones.new')" :placeholder="t('milestones.name')" :submit="create">
        <NativeSelect v-if="phases?.length" v-model="draft.phase" class="w-44" :aria-label="t('milestones.phase')">
          <option value="">{{ t('milestones.noPhase') }}</option>
          <option v-for="ph in phases" :key="ph.id" :value="ph.id">{{ ph.name }}</option>
        </NativeSelect>
        <Input v-model="draft.due" type="date" class="h-8 w-40" :aria-label="t('milestones.due')" />
      </QuickAddRow>
    </tbody>
  </GridTable>
</template>

<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { createMilestone, listMilestones, listPhases } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { milestoneVariant } from '@/features/work/Badges'
import ProgressBar from '@/features/work/ProgressBar.vue'
import type { Project } from '@/types/work'

// Milestones of a project with their progress (RF-44 to RF-56).
const props = defineProps<{ project: Project }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()

const { data: milestones } = useQuery({ queryKey: ['milestones', props.project.id], queryFn: () => listMilestones(props.project.id) })
const { data: phases } = useQuery({ queryKey: ['phases', props.project.id], queryFn: () => listPhases(props.project.id) })

const form = ref({ name: '', due: '', phase: '' })
const create = useMutation({
  mutationFn: () =>
    createMilestone(props.project.id, { name: form.value.name, due_date: form.value.due || null, phase_id: form.value.phase || null }),
  onSuccess: () => {
    form.value = { name: '', due: '', phase: '' }
    return qc.invalidateQueries({ queryKey: ['milestones', props.project.id] })
  },
  onError: (e) => emit('error', e),
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <form
      v-if="project.can.manage && project.lifecycle === 'activo'"
      class="flex flex-wrap items-center gap-2 rounded-lg border bg-background p-3"
      @submit.prevent="create.mutate()"
    >
      <Input v-model="form.name" class="w-64" :placeholder="t('milestones.name')" :aria-label="t('milestones.name')" required />
      <Input v-model="form.due" type="date" class="w-40" :aria-label="t('milestones.due')" />
      <NativeSelect v-if="phases?.length" v-model="form.phase" class="w-48" :aria-label="t('milestones.phase')">
        <option value="">{{ t('milestones.noPhase') }}</option>
        <option v-for="ph in phases" :key="ph.id" :value="ph.id">{{ ph.name }}</option>
      </NativeSelect>
      <Button type="submit" size="sm" :disabled="create.isPending.value">{{ t('common.create') }}</Button>
    </form>

    <ul class="flex flex-col gap-2">
      <li v-for="m in milestones ?? []" :key="m.id" class="grid gap-2 rounded-lg border bg-background p-3 sm:grid-cols-[1fr_12rem_10rem]" data-testid="milestone-row">
        <div class="flex flex-wrap items-center gap-2">
          <RouterLink :to="`/milestones/${m.id}`" class="font-medium hover:underline">{{ m.name }}</RouterLink>
          <Badge :variant="milestoneVariant[m.status]">{{ t(`milestones.statuses.${m.status}`) }}</Badge>
          <Badge v-if="m.overdue" variant="destructive">{{ t('milestones.overdue') }}</Badge>
          <span v-if="m.phase" class="text-xs text-muted-foreground">{{ m.phase.name }}</span>
        </div>
        <div class="flex flex-col gap-1 text-xs text-muted-foreground">
          <ProgressBar :value="m.progress.percent" :label="t('milestones.progress')" />
          {{ t('milestones.counts', { closed: m.progress.closed, total: m.progress.total }) }}
        </div>
        <div class="text-sm">{{ m.completed_on ?? m.due_date ?? t('common.none') }}</div>
      </li>
    </ul>
    <p v-if="milestones && milestones.length === 0" class="text-sm text-muted-foreground">{{ t('common.empty') }}</p>
  </div>
</template>

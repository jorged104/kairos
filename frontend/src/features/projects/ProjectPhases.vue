<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { useI18n } from 'vue-i18n'
import { listPhases, setGate, updatePhase } from '@/api/work'
import { Input } from '@/components/ui/input'
import type { Project, ProjectPhase } from '@/types/work'

// Phases of a project with their dates and informative gates (RF-40 to RF-43).
const props = defineProps<{ project: Project }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t, d } = useI18n()
const qc = useQueryClient()

const { data: phases } = useQuery({ queryKey: ['phases', props.project.id], queryFn: () => listPhases(props.project.id) })
const refresh = () => qc.invalidateQueries({ queryKey: ['phases', props.project.id] })
const dates = useMutation({
  mutationFn: (x: { phase: ProjectPhase; start: string; finish: string }) => updatePhase(x.phase.id, x.start || null, x.finish || null),
  onSuccess: refresh,
  onError: (e) => {
    emit('error', e)
    return refresh()
  },
})
const gate = useMutation({
  mutationFn: (x: { phase: ProjectPhase; gate: 'start' | 'finish'; passed: boolean }) => setGate(x.phase.id, x.gate, x.passed),
  onSuccess: refresh,
  onError: (e) => emit('error', e),
})
const editable = () => props.project.can.manage && props.project.lifecycle === 'activo'
</script>

<template>
  <div class="rounded-lg border bg-background">
    <p v-if="phases && phases.length === 0" class="p-4 text-sm text-muted-foreground">{{ t('projects.noPhases') }}</p>
    <ol class="divide-y">
      <li v-for="ph in phases ?? []" :key="ph.id" class="flex flex-wrap items-center gap-3 p-3 text-sm" data-testid="phase-row">
        <span class="w-48 font-medium">{{ ph.position }}. {{ ph.name }}</span>
        <label class="flex items-center gap-1">
          {{ t('projects.phaseStart') }}
          <Input
            type="date"
            class="w-40"
            :model-value="ph.start_date ?? ''"
            :disabled="!editable()"
            @change="dates.mutate({ phase: ph, start: ($event.target as HTMLInputElement).value, finish: ph.finish_date ?? '' })"
          />
        </label>
        <label class="flex items-center gap-1">
          {{ t('projects.phaseFinish') }}
          <Input
            type="date"
            class="w-40"
            :model-value="ph.finish_date ?? ''"
            :disabled="!editable()"
            @change="dates.mutate({ phase: ph, start: ph.start_date ?? '', finish: ($event.target as HTMLInputElement).value })"
          />
        </label>
        <label v-if="ph.has_start_gate" class="flex items-center gap-1">
          <input
            type="checkbox"
            :checked="!!ph.start_gate_passed_at"
            :disabled="!editable()"
            @change="gate.mutate({ phase: ph, gate: 'start', passed: !ph.start_gate_passed_at })"
          />
          {{ t('catalog.startGate') }}
          <span v-if="ph.start_gate_passed_at" class="text-xs text-muted-foreground">{{ d(new Date(ph.start_gate_passed_at), 'long') }}</span>
        </label>
        <label v-if="ph.has_finish_gate" class="flex items-center gap-1">
          <input
            type="checkbox"
            :checked="!!ph.finish_gate_passed_at"
            :disabled="!editable()"
            @change="gate.mutate({ phase: ph, gate: 'finish', passed: !ph.finish_gate_passed_at })"
          />
          {{ t('catalog.finishGate') }}
          <span v-if="ph.finish_gate_passed_at" class="text-xs text-muted-foreground">{{ d(new Date(ph.finish_gate_passed_at), 'long') }}</span>
        </label>
      </li>
    </ol>
  </div>
</template>

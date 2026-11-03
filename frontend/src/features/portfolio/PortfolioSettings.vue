<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { setPortfolio } from '@/api/portfolio'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Project } from '@/types/work'

// Whether the project is a program and its weight in the programs above it,
// for the administrator (spec 010, RF-1 to RF-8).
const props = defineProps<{ project: Project }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()

const program = ref(!!props.project.is_program)
const weight = ref(props.project.portfolio_weight?.toString() ?? '')
watch(
  () => props.project,
  (p) => {
    program.value = !!p.is_program
    weight.value = p.portfolio_weight?.toString() ?? ''
  },
)

const save = useMutation({
  mutationFn: () =>
    setPortfolio(props.project.id, { version: props.project.version, is_program: program.value, weight: String(weight.value).trim() === '' ? null : Number(weight.value) }),
  onSuccess: (p) => {
    qc.setQueryData(['project', props.project.id], p)
    qc.invalidateQueries({ queryKey: ['portfolio'] })
    qc.invalidateQueries({ queryKey: ['overview'] })
  },
  onError: (e) => emit('error', e),
})
</script>

<template>
  <section class="rounded-lg border bg-background p-4 text-sm" data-testid="portfolio-settings">
    <h2 class="mb-2 font-medium">{{ t('portfolio.settings.title') }}</h2>
    <form class="flex flex-wrap items-end gap-4" @submit.prevent="save.mutate()">
      <label class="flex items-center gap-2">
        <input v-model="program" type="checkbox" class="size-4 accent-primary" :disabled="project.is_template" data-testid="portfolio-is-program" />
        {{ t('portfolio.settings.isProgram') }}
      </label>
      <div class="flex flex-col gap-1">
        <Label for="portfolio-weight">{{ t('portfolio.settings.weight') }}</Label>
        <Input
          id="portfolio-weight"
          v-model="weight"
          type="number"
          min="0"
          max="100000"
          step="1"
          class="w-36"
          :placeholder="String(project.effort ?? 0)"
          data-testid="portfolio-weight"
        />
      </div>
      <p class="text-xs text-muted-foreground" data-testid="portfolio-effort">
        {{ project.portfolio_weight == null ? t('portfolio.settings.byEffort', { n: project.effort ?? 0 }) : t('portfolio.settings.manual', { n: project.effort ?? 0 }) }}
      </p>
      <Button type="submit" size="sm" class="ml-auto" :disabled="save.isPending.value || project.lifecycle !== 'activo'" data-testid="portfolio-save">
        {{ t('common.save') }}
      </Button>
    </form>
  </section>
</template>

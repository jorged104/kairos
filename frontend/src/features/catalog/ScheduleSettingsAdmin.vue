<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { LEVELS, PALETTE, updateScheduleSettings, type ScheduleSettings } from '@/api/schedule'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { COLOR_CLASSES, useLevels } from '@/features/schedule/useLevels'

// The traffic light: thresholds, names and colors of the levels, for the
// administrator (spec 009, RF-36 to RF-41).
const emit = defineEmits<{ error: [unknown]; saved: [] }>()
const { t } = useI18n()
const qc = useQueryClient()
const { settings } = useLevels()

const copy = (s: ScheduleSettings): ScheduleSettings => JSON.parse(JSON.stringify(s))
const form = ref<ScheduleSettings>(copy(settings.value))
watch(settings, (s) => (form.value = copy(s)))
const ordered = computed(() => form.value.upper > form.value.risk && form.value.risk > form.value.late) // RF-37

const save = useMutation({
  mutationFn: () => updateScheduleSettings(form.value),
  onSuccess: (s) => {
    qc.setQueryData(['schedule-settings'], s)
    qc.invalidateQueries({ queryKey: ['overview'] })
    qc.invalidateQueries({ queryKey: ['schedule'] })
    emit('saved')
  },
  onError: (e) => emit('error', e),
})
</script>

<template>
  <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="schedule-settings" data-testid="schedule-settings">
    <div>
      <h2 id="schedule-settings" class="font-medium">{{ t('schedule.settings.title') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('schedule.settings.help') }}</p>
    </div>
    <form class="flex flex-col gap-4" @submit.prevent="save.mutate()">
      <div class="grid gap-3 sm:grid-cols-3">
        <div v-for="k in ['upper', 'risk', 'late'] as const" :key="k" class="flex flex-col gap-1">
          <Label :for="`threshold-${k}`">{{ t(`schedule.settings.${k}`) }}</Label>
          <Input :id="`threshold-${k}`" v-model.number="form[k]" type="number" min="-100" max="100" step="0.1" required :data-testid="`threshold-${k}`" />
        </div>
      </div>
      <p v-if="!ordered" class="text-sm text-destructive" role="alert" data-testid="threshold-order">{{ t('schedule.settings.order') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <div v-for="l in LEVELS" :key="l" class="flex items-end gap-2">
          <span class="mb-2 size-4 shrink-0 rounded-full" :class="COLOR_CLASSES[form.levels[l].color].solid" aria-hidden="true" />
          <div class="flex min-w-0 flex-1 flex-col gap-1">
            <Label :for="`level-name-${l}`">{{ t('schedule.settings.name', { level: t(`schedule.levels.${l}`) }) }}</Label>
            <Input
              :id="`level-name-${l}`"
              :model-value="form.levels[l].name ?? ''"
              maxlength="30"
              :placeholder="t(`schedule.levels.${l}`)"
              :data-testid="`level-name-${l}`"
              @update:model-value="(v) => (form.levels[l].name = String(v).trim() || null)"
            />
          </div>
          <select
            v-model="form.levels[l].color"
            class="h-9 rounded-md border bg-background px-2 text-sm"
            :aria-label="t('schedule.settings.color', { level: t(`schedule.levels.${l}`) })"
            :data-testid="`level-color-${l}`"
          >
            <option v-for="c in PALETTE" :key="c" :value="c">{{ t(`schedule.settings.colors.${c}`) }}</option>
          </select>
        </div>
      </div>
      <div class="flex justify-end">
        <Button type="submit" size="sm" :disabled="!ordered || save.isPending.value" data-testid="schedule-save">{{ t('schedule.settings.save') }}</Button>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Figures } from '@/api/schedule'
import { signed, useLevels } from './useLevels'

// The traffic light of something with its plan, real and gap (spec 009, RF-35).
defineProps<{ figures: Figures; compact?: boolean }>()
const { t } = useI18n()
const { name, classes } = useLevels()
const pct = (n: number) => n.toLocaleString('es', { maximumFractionDigits: 1 })
</script>

<template>
  <span v-if="figures.has_plan && figures.level" class="inline-flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs" data-testid="level-badge">
    <span class="rounded-full px-2 py-0.5 font-semibold" :class="classes(figures.level).badge" :data-level="figures.level">{{ name(figures.level) }}</span>
    <span v-if="!compact" class="text-muted-foreground">{{ t('schedule.figures', { plan: pct(figures.plan), real: pct(figures.real), gap: signed(figures.gap) }) }}</span>
    <span v-else class="text-muted-foreground">{{ t('schedule.gap', { gap: signed(figures.gap) }) }}</span>
  </span>
</template>

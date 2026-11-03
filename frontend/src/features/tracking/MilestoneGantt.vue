<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { MILESTONE_STATES, type MilestoneState, type TrackingProject } from '@/api/tracking'
import { useLevels } from '@/features/schedule/useLevels'

// The milestones of the projects followed on a line of months: the bar from
// their work items with the real progress, the target date and today
// (spec 011, RF-15 to RF-29).
const props = defineProps<{ projects: TrackingProject[]; today: string }>()
const { t, locale } = useI18n()
const levels = useLevels()

const DAY = 86_400_000
const at = (d: string) => new Date(d + 'T00:00:00Z').getTime()
const range = computed(() => {
  const days = [at(props.today)]
  for (const p of props.projects)
    for (const m of p.milestones) for (const d of [m.bar_start, m.bar_end, m.due_date]) if (d) days.push(at(d))
  const a = new Date(Math.min(...days))
  const b = new Date(Math.max(...days))
  const from = Date.UTC(a.getUTCFullYear(), a.getUTCMonth(), 1)
  const to = Date.UTC(b.getUTCFullYear(), b.getUTCMonth() + 1, 1)
  return { from, to, span: to - from }
})
const pos = (d: string) => (100 * (at(d) - range.value.from)) / range.value.span
const months = computed(() => {
  const out: { left: number; label: string }[] = []
  const fmt = new Intl.DateTimeFormat(locale.value, { month: 'short', year: '2-digit', timeZone: 'UTC' })
  for (let d = new Date(range.value.from); d.getTime() < range.value.to; d.setUTCMonth(d.getUTCMonth() + 1))
    out.push({ left: (100 * (d.getTime() - range.value.from)) / range.value.span, label: fmt.format(d) })
  return out
})
// Each state with the color of its level where it has one (RF-27).
function color(s: MilestoneState) {
  if (s === 'running') return levels.classes('on_track').solid
  if (s === 'at_risk' || s === 'late') return levels.classes(s).solid
  if (s === 'done') return 'bg-foreground'
  return s === 'future' ? 'bg-slate-400 dark:bg-slate-500' : 'bg-muted-foreground/40'
}
const date = (d: string | null) => (d ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(at(d)) : '')
const pct = (n: number) => `${n.toLocaleString('es', { maximumFractionDigits: 1 })} %`
</script>

<template>
  <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="tr-gantt" data-testid="tracking-gantt">
    <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <h2 id="tr-gantt" class="text-base font-semibold">{{ t('tracking.milestones') }}</h2>
      <ul class="flex flex-wrap gap-3 text-xs text-muted-foreground" data-testid="tracking-legend">
        <li v-for="s in MILESTONE_STATES" :key="s" class="inline-flex items-center gap-1.5">
          <span class="size-2.5 rounded-full" :class="color(s)" aria-hidden="true" />{{ t(`tracking.states.${s}`) }}
        </li>
      </ul>
    </div>
    <div class="overflow-x-auto">
      <div class="min-w-[720px]">
        <div class="flex text-xs text-muted-foreground">
          <div class="w-64 shrink-0" />
          <div class="relative h-5 flex-1">
            <span v-for="m in months" :key="m.left" class="absolute top-0 border-l pl-1" :style="{ left: `${m.left}%` }">{{ m.label }}</span>
          </div>
        </div>
        <template v-for="p in projects" :key="p.id">
          <div class="mt-2 rounded bg-muted px-2 py-1 text-sm font-semibold">
            <RouterLink :to="`/projects/${p.id}`" class="hover:underline">{{ p.name }}</RouterLink>
          </div>
          <p v-if="!p.milestones.length" class="px-2 py-1.5 text-xs text-muted-foreground">{{ t('tracking.noMilestones') }}</p>
          <div v-for="m in p.milestones" :key="m.id" class="flex items-center border-b py-1.5" data-testid="tracking-milestone" :data-state="m.state">
            <div class="w-64 shrink-0 pr-2">
              <RouterLink :to="`/milestones/${m.id}`" class="block truncate text-sm font-medium hover:underline">{{ m.name }}</RouterLink>
              <span class="text-xs text-muted-foreground">
                {{ t(`tracking.states.${m.state}`) }}
                <template v-if="m.figures.has_plan"> · {{ t('tracking.planReal', { plan: pct(m.figures.plan), real: pct(m.figures.real) }) }}</template>
              </span>
            </div>
            <div class="relative h-6 flex-1">
              <div class="absolute inset-y-0 w-px bg-amber-600" :style="{ left: `${pos(today)}%` }" aria-hidden="true" />
              <div
                v-if="m.bar_start && m.bar_end"
                class="absolute top-1 h-4 overflow-hidden rounded bg-muted"
                :style="{ left: `${pos(m.bar_start)}%`, width: `max(4px, ${pos(m.bar_end) - pos(m.bar_start) + (100 * DAY) / range.span}%)` }"
                :title="`${date(m.bar_start)} → ${date(m.bar_end)}`"
                data-testid="tracking-bar"
              >
                <div class="h-full" :class="color(m.state)" :style="{ width: `${m.figures.real}%` }" />
              </div>
              <!-- The target date (RF-17). -->
              <span
                v-if="m.due_date"
                class="absolute top-0.5 size-3 -translate-x-1/2 rotate-45 border-2 border-foreground bg-background"
                :style="{ left: `${pos(m.due_date)}%` }"
                :title="t('tracking.target', { date: date(m.due_date) })"
                data-testid="tracking-target"
              />
              <span v-if="!m.bar_start && !m.due_date" class="text-xs text-muted-foreground">{{ t('tracking.states.no_data') }}</span>
            </div>
          </div>
        </template>
      </div>
    </div>
  </section>
</template>

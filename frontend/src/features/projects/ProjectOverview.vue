<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { getOverview } from '@/api/projectScreens'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import ProgramRows from '@/features/portfolio/ProgramRows.vue'
import { DEFAULT_SORT, sortRows, type Sort } from '@/features/portfolio/rows'
import BaselinePanel from '@/features/schedule/BaselinePanel.vue'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import { signed, useLevels } from '@/features/schedule/useLevels'
import AvatarStack from '@/features/work/AvatarStack.vue'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog } from '@/features/work/useCatalog'
import type { Project } from '@/types/work'

// The project at a glance (spec 008, RF-6 to RF-17).
const props = defineProps<{ project: Project }>()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const { statuses } = useCatalog()
const levels = useLevels()
const programSort = ref<Sort>(DEFAULT_SORT)

const { data: o, error } = useQuery({ queryKey: computed(() => ['overview', props.project.id]), queryFn: () => getOverview(props.project.id) })

const date = (d: string | null | undefined) =>
  d ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(new Date(d + 'T00:00:00')) : t('common.none')
const tableWith = (q: Record<string, string>) => ({ query: { tab: 'table', ...q } })

const byStatus = computed(() =>
  (o.value?.statuses ?? [])
    .map((s) => ({ ...s, name: statuses.value.get(s.status_id)?.name ?? '', color: statuses.value.get(s.status_id)?.color ?? '#a3a3a3', position: statuses.value.get(s.status_id)?.position ?? 0 }))
    .sort((a, b) => a.position - b.position),
)

// The chart: the plan of each day from the baseline (spec 009, RF-46) and the
// days recorded (spec 008, RF-13, RF-14).
const W = 760
const H = 220
const chart = computed(() => {
  const ov = o.value
  if (!ov) return null
  const day = (s: string) => new Date(s + 'T00:00:00Z').getTime()
  const today = day(ov.today)
  const points = [...ov.history.map((h) => ({ at: day(h.day), v: h.progress }))]
  if (!points.length || points[points.length - 1]!.at !== today) points.push({ at: today, v: ov.progress })
  const series = ov.plan_series.map((p) => ({ at: day(p.day), v: p.plan }))
  const start = Math.min(...points.map((p) => p.at), ...series.map((p) => p.at))
  const end = Math.max(today, start + 86400000, ...series.map((p) => p.at))
  const span = Math.max(end - start, 86400000)
  const x = (at: number) => 40 + ((Math.min(Math.max(at, start), end) - start) / span) * (W - 50)
  const y = (v: number) => 10 + (1 - v / 100) * (H - 30)
  return {
    plan: ov.has_plan ? series.map((p) => `${x(p.at)},${y(p.v)}`).join(' ') : '',
    real: points.map((p) => `${x(p.at)},${y(p.v)}`).join(' '),
    todayX: x(today),
    todayY: y(ov.progress),
    single: ov.history.length < 2,
    grid: [0, 25, 50, 75, 100].map((v) => ({ v, y: y(v) })),
  }
})
const riskClass = { overdue: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300', blocked: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300' }
// Behind takes the color of its level (spec 009, RF-47).
const riskStyle = (r: { reason: 'overdue' | 'blocked' | 'behind'; level?: 'ahead' | 'on_track' | 'at_risk' | 'late' }) =>
  r.reason === 'behind' ? levels.classes(r.level ?? 'late').badge : riskClass[r.reason]
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="overview">
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <template v-if="o">
      <div v-if="o.total === 0" class="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground" data-testid="overview-empty">
        {{ t('overview.empty') }}
        <RouterLink :to="{ query: { tab: 'table' } }" class="ml-1 font-medium text-foreground underline">{{ t('overview.firstTask') }}</RouterLink>
      </div>

      <!-- A program: its aggregate and its direct children (spec 010, RF-39, RF-40). -->
      <section v-if="o.program?.program" class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="ov-program" data-testid="overview-program">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
          <h2 id="ov-program" class="text-base font-semibold">{{ t('portfolio.aggregate') }}</h2>
          <span class="text-xs text-muted-foreground">
            {{ t('portfolio.projects', { count: o.program.projects }, o.program.projects) }} · {{ t('portfolio.belowPlan', { count: o.program.below_plan }, o.program.below_plan) }}
          </span>
          <span class="ml-auto text-2xl font-bold tabular-nums">{{ o.program.program.figures.real.toLocaleString('es', { maximumFractionDigits: 1 }) }} %</span>
        </div>
        <LevelBadge :figures="o.program.program.figures" />
        <ProgramRows v-if="o.program.rows.length" :rows="sortRows(o.program.rows, programSort)" :sort="programSort" @sort="programSort = $event" />
      </section>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('overview.indicators')">
        <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="overview-progress">
          <h2 class="text-sm font-medium text-muted-foreground">{{ t('overview.progress') }}</h2>
          <p class="flex items-baseline gap-2">
            <span class="text-3xl font-bold tracking-tight">{{ o.progress }} %</span>
          </p>
          <ProgressBar :value="o.progress" />
          <LevelBadge :figures="o.figures" />
          <p class="text-xs text-muted-foreground">{{ o.has_plan ? t('overview.expected', { n: o.expected }) : t('overview.noPlan') }}</p>
        </article>
        <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="overview-tasks">
          <h2 class="text-sm font-medium text-muted-foreground">{{ t('overview.tasks') }}</h2>
          <p class="flex items-baseline gap-2">
            <span class="text-3xl font-bold tracking-tight">{{ o.closed }}</span>
            <span class="text-sm text-muted-foreground">{{ t('overview.closedOf', { total: o.total }) }}</span>
          </p>
          <div class="flex h-2 gap-0.5 overflow-hidden rounded-full bg-muted" aria-hidden="true">
            <span v-for="s in byStatus" :key="s.status_id" :style="{ flex: s.count, background: s.color }" />
          </div>
          <p class="text-xs text-muted-foreground">
            <span v-for="(s, i) in byStatus" :key="s.status_id">{{ i ? ' · ' : '' }}{{ s.count }} {{ s.name.toLocaleLowerCase() }}</span>
          </p>
        </article>
        <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="overview-attention">
          <h2 class="text-sm font-medium text-muted-foreground">{{ t('overview.attention') }}</h2>
          <p class="flex flex-wrap items-baseline gap-4">
            <RouterLink :to="tableWith({ overdue: 'true' })" class="hover:underline" data-testid="overview-overdue">
              <span class="text-3xl font-bold text-destructive">{{ o.overdue }}</span> <span class="text-sm text-muted-foreground">{{ t('overview.overdue') }}</span>
            </RouterLink>
            <RouterLink :to="tableWith({ blocked: 'true' })" class="hover:underline" data-testid="overview-blocked">
              <span class="text-3xl font-bold text-amber-700 dark:text-amber-400">{{ o.blocked }}</span> <span class="text-sm text-muted-foreground">{{ t('overview.blocked') }}</span>
            </RouterLink>
          </p>
        </article>
        <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="overview-next">
          <h2 class="text-sm font-medium text-muted-foreground">{{ t('overview.nextMilestone') }}</h2>
          <template v-if="o.next_milestone">
            <p class="text-lg font-semibold">{{ o.next_milestone.name }}</p>
            <p class="text-sm text-muted-foreground">
              {{ date(o.next_milestone.due_date) }} ·
              {{ (o.next_milestone.days_left ?? 0) >= 0 ? t('overview.daysLeft', { count: o.next_milestone.days_left }, o.next_milestone.days_left ?? 0) : t('overview.daysLate', { count: -(o.next_milestone.days_left ?? 0) }, -(o.next_milestone.days_left ?? 0)) }}
            </p>
            <ProgressBar :value="o.next_milestone.percent" />
          </template>
          <p v-else class="text-sm text-muted-foreground">{{ t('overview.noMilestone') }}</p>
        </article>
      </section>

      <div class="flex flex-wrap items-start gap-4">
        <section class="flex min-w-0 flex-[999_1_560px] flex-col gap-3 rounded-xl border bg-background p-4" :aria-labelledby="'ov-chart'">
          <div class="flex flex-wrap items-center gap-3">
            <h2 id="ov-chart" class="text-base font-semibold">{{ t('overview.chart') }}</h2>
            <div class="ml-auto flex gap-4 text-xs text-muted-foreground">
              <span class="inline-flex items-center gap-1.5"><span class="h-0.5 w-4 bg-foreground" />{{ t('overview.real') }}</span>
              <span v-if="o.has_plan" class="inline-flex items-center gap-1.5"><span class="w-4 border-t-2 border-dashed border-muted-foreground" />{{ t('overview.plan') }}</span>
            </div>
          </div>
          <svg v-if="chart" :viewBox="`0 0 ${W} ${H}`" role="img" :aria-label="t('overview.chartLabel', { real: o.progress, expected: o.expected })" class="h-auto w-full" data-testid="overview-chart">
            <g v-for="g in chart.grid" :key="g.v">
              <line x1="40" :y1="g.y" :x2="W - 10" :y2="g.y" class="stroke-border" />
              <text x="4" :y="g.y + 4" class="fill-muted-foreground text-[11px]">{{ g.v }}</text>
            </g>
            <polyline v-if="chart.plan" :points="chart.plan" fill="none" class="stroke-muted-foreground" stroke-width="2" stroke-dasharray="6 5" />
            <polyline v-if="!chart.single" :points="chart.real" fill="none" class="stroke-foreground" stroke-width="2.5" />
            <line :x1="chart.todayX" y1="10" :x2="chart.todayX" :y2="H - 20" stroke="#d97706" stroke-dasharray="3 3" />
            <circle :cx="chart.todayX" :cy="chart.todayY" r="4" class="fill-foreground" />
          </svg>
          <p v-if="chart?.single" class="text-xs text-muted-foreground">{{ t('overview.chartStarts') }}</p>
        </section>

        <section class="flex min-w-0 flex-[1_1_360px] flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="ov-milestones" data-testid="overview-milestones">
          <h2 id="ov-milestones" class="text-base font-semibold">{{ t('overview.milestones') }}</h2>
          <p v-if="!o.milestones.length" class="text-sm text-muted-foreground">{{ t('overview.noMilestones') }}</p>
          <div v-for="m in o.milestones" :key="m.id" class="flex flex-col gap-1.5">
            <div class="flex items-baseline gap-2 text-sm">
              <span class="font-medium" :class="m.status === 'cancelado' && 'text-muted-foreground line-through'">{{ m.name }}</span>
              <span class="ml-auto text-xs text-muted-foreground">{{ date(m.due_date) }}</span>
            </div>
            <ProgressBar :value="m.percent" />
            <span class="text-xs text-muted-foreground">{{ t(`milestones.statuses.${m.status}`) }} · {{ t('overview.milestoneCount', { closed: m.closed, total: m.total }) }}</span>
            <LevelBadge :figures="m.figures" data-testid="overview-milestone-level" />
          </div>
        </section>
      </div>

      <div class="flex flex-wrap items-start gap-4">
        <section class="flex min-w-0 flex-[999_1_560px] flex-col gap-2 rounded-xl border bg-background p-4" aria-labelledby="ov-risks" data-testid="overview-risks">
          <h2 id="ov-risks" class="text-base font-semibold">{{ t('overview.risks') }}</h2>
          <p v-if="!o.risks.length" class="text-sm text-muted-foreground">{{ t('overview.noRisks') }}</p>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-muted-foreground">
                  <th class="py-2 pr-2 font-medium">{{ t('overview.task') }}</th>
                  <th class="p-2 font-medium">{{ t('overview.reason') }}</th>
                  <th class="p-2 font-medium">{{ t('workItems.assignees') }}</th>
                  <th class="p-2 font-medium">{{ t('workItems.due') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in o.risks" :key="r.id" class="border-t" data-testid="overview-risk">
                  <td class="py-2 pr-2">
                    <RouterLink :to="{ query: { tab: 'table', item: r.key } }" class="hover:underline">
                      <span class="font-mono text-xs text-muted-foreground">{{ r.key }}</span> {{ r.title }}
                    </RouterLink>
                  </td>
                  <td class="p-2">
                    <span class="rounded-full px-2 py-0.5 text-xs font-semibold" :class="riskStyle(r)">
                      {{
                        r.reason === 'overdue'
                          ? t('overview.reasons.overdue', { count: r.days }, r.days)
                          : r.reason === 'behind'
                            ? t('overview.reasons.behind', { level: levels.name(r.level ?? 'late'), gap: signed(r.gap) })
                            : t('overview.reasons.blocked')
                      }}
                    </span>
                  </td>
                  <td class="p-2">
                    <AvatarStack v-if="r.assignees.length" :people="r.assignees" />
                    <span v-else class="text-muted-foreground">{{ t('workItems.unassigned') }}</span>
                  </td>
                  <td class="p-2 whitespace-nowrap text-muted-foreground">{{ date(r.due_date) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="flex min-w-0 flex-[1_1_360px] flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="ov-load" data-testid="overview-load">
          <div class="flex items-center gap-2">
            <h2 id="ov-load" class="text-base font-semibold">{{ t('overview.load') }}</h2>
            <RouterLink v-if="project.can.manage_team" :to="{ query: { tab: 'team' } }" class="ml-auto text-sm font-medium hover:underline">{{ t('overview.manageTeam') }}</RouterLink>
          </div>
          <p v-if="!o.load.length" class="text-sm text-muted-foreground">{{ t('overview.noLoad') }}</p>
          <div v-for="m in o.load" :key="m.user.id" class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1 text-sm">
            <span class="truncate font-medium">{{ m.user.name }}</span>
            <span class="text-xs" :class="m.load.overloaded ? 'font-semibold text-orange-700 dark:text-orange-400' : 'text-muted-foreground'">
              {{ m.capacity === null ? m.load.total : `${m.load.total} / ${m.capacity}` }}
            </span>
            <ProgressBar class="col-span-2" :value="m.capacity ? Math.min(100, Math.round((m.load.total / m.capacity) * 100)) : 0" />
          </div>
        </section>
      </div>

      <BaselinePanel :project="project" />

      <section class="rounded-xl border bg-background p-4 text-sm" data-testid="overview-health">
        <h2 class="mb-1 text-base font-semibold">{{ t('overview.health') }}: {{ t(`projects.healths.${project.health}`) }}</h2>
        <p class="text-muted-foreground">{{ project.health_explanation || t('overview.noHealthNote') }}</p>
        <RouterLink v-if="project.can.manage" :to="{ query: { tab: 'config' } }" class="mt-1 inline-block font-medium hover:underline">{{ t('overview.editHealth') }}</RouterLink>
      </section>
    </template>
  </div>
</template>

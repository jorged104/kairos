<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DashboardLoad, DashboardWeek } from '@/api/dashboard'

// The caller's progress: work items closed per week, punctuality and the
// current load by project, each with its table of data (spec 013, RF-28 to
// RF-35). One series per chart in one hue; «atrasadas» is a state and goes in
// the destructive color, always with its label.
// With owner, another person's (spec 013, RF-47).
const props = defineProps<{ weeks: DashboardWeek[]; load: DashboardLoad[]; owner?: string }>()
const { t, locale } = useI18n()

const date = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z'))
const pct = (n: number) => `${Math.round(n)} %`

// Weekly bars (RF-28, RF-29).
const W = 360
const H = 120
const step = W / 12
const max = computed(() => Math.max(1, ...props.weeks.map((w) => w.closed)))
const total = computed(() => props.weeks.reduce((s, w) => s + w.closed, 0))
const bars = computed(() =>
  props.weeks.map((w, i) => {
    const h = (w.closed / max.value) * (H - 18)
    return { ...w, x: i * step + 4, y: H - h, h, label: i % 3 === 0 || i === props.weeks.length - 1 }
  }),
)
const peak = computed(() => bars.value.reduce((a, b) => (b.closed > a.closed ? b : a), bars.value[0]!))
const weekText = (w: DashboardWeek) =>
  `${t('dashboard.charts.weekOf', { date: date(w.start) })}: ${t('dashboard.charts.closedN', { n: w.closed })}, ${t('dashboard.charts.weightN', { n: w.weight })}`

// Punctuality (RF-30 to RF-32).
const withDue = computed(() => props.weeks.reduce((s, w) => s + w.with_due, 0))
const onTime = computed(() => props.weeks.reduce((s, w) => s + w.on_time, 0))
const onTimeTotal = computed(() => (withDue.value ? (100 * onTime.value) / withDue.value : null))
const onTimeText = (w: DashboardWeek) =>
  `${t('dashboard.charts.weekOf', { date: date(w.start) })}: ${w.with_due ? t('dashboard.charts.onTimeOf', { on: w.on_time, all: w.with_due }) : t('dashboard.charts.noData')}`

// Load by project (RF-33, RF-34).
const loadMax = computed(() => Math.max(1, ...props.load.map((l) => l.open)))
const loadName = (l: DashboardLoad) => l.project?.name ?? t('dashboard.charts.others')
</script>

<template>
  <section class="flex flex-col gap-3" aria-labelledby="my-progress" data-testid="progress-charts">
    <h2 id="my-progress" class="text-base font-semibold">{{ owner ? t('dashboard.others.charts', { name: owner }) : t('dashboard.charts.title') }}</h2>
    <div class="grid gap-4 lg:grid-cols-3">
      <!-- Closed per week. -->
      <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="chart-weekly">
        <h3 class="text-sm font-semibold">{{ t('dashboard.charts.weekly') }}</h3>
        <svg :viewBox="`0 0 ${W} ${H + 20}`" class="w-full text-primary" role="img" :aria-label="t('dashboard.charts.weeklyAlt', { total })">
          <line :x1="0" :x2="W" :y1="H" :y2="H" class="stroke-border" stroke-width="1" />
          <g v-for="b in bars" :key="b.start" tabindex="0" class="outline-none focus-visible:opacity-80" :aria-label="weekText(b)" data-testid="weekly-bar">
            <title>{{ weekText(b) }}</title>
            <rect :x="b.x - 2" :y="0" :width="step - 4" :height="H" fill="transparent" />
            <rect v-if="b.closed" :x="b.x" :y="b.y" :width="step - 8" :height="b.h" rx="4" fill="currentColor" />
            <text v-if="b.label" :x="b.x + (step - 8) / 2" :y="H + 14" text-anchor="middle" class="fill-muted-foreground text-[9px]">{{ date(b.start) }}</text>
          </g>
          <text v-if="peak.closed" :x="peak.x + (step - 8) / 2" :y="peak.y - 4" text-anchor="middle" class="fill-foreground text-[10px] font-semibold">{{ peak.closed }}</text>
        </svg>
        <details class="text-xs">
          <summary class="cursor-pointer text-muted-foreground">{{ t('dashboard.charts.table') }}</summary>
          <table class="mt-2 w-full" data-testid="weekly-table">
            <thead>
              <tr class="text-left text-muted-foreground">
                <th class="py-1 font-medium">{{ t('dashboard.charts.week') }}</th>
                <th class="py-1 text-right font-medium">{{ t('dashboard.charts.closed') }}</th>
                <th class="py-1 text-right font-medium">{{ t('dashboard.charts.weight') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="w in weeks" :key="w.start" class="border-t">
                <td class="py-1">{{ date(w.start) }}</td>
                <td class="py-1 text-right tabular-nums">{{ w.closed }}</td>
                <td class="py-1 text-right tabular-nums">{{ w.weight }}</td>
              </tr>
            </tbody>
          </table>
        </details>
      </article>

      <!-- Punctuality. -->
      <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="chart-ontime">
        <h3 class="text-sm font-semibold">{{ t('dashboard.charts.onTime') }}</h3>
        <p class="text-3xl font-bold tabular-nums" data-testid="ontime-total">
          {{ onTimeTotal === null ? t('dashboard.charts.noData') : pct(onTimeTotal) }}
        </p>
        <p v-if="onTimeTotal !== null" class="-mt-1 text-xs text-muted-foreground">{{ t('dashboard.charts.onTimeOf', { on: onTime, all: withDue }) }}</p>
        <svg :viewBox="`0 0 ${W} 64`" class="w-full text-primary" role="img" :aria-label="t('dashboard.charts.onTimeAlt')">
          <line :x1="0" :x2="W" y1="56" y2="56" class="stroke-border" stroke-width="1" />
          <g v-for="(w, i) in weeks" :key="w.start" tabindex="0" class="outline-none" :aria-label="onTimeText(w)" data-testid="ontime-bar">
            <title>{{ onTimeText(w) }}</title>
            <rect :x="i * step + 2" y="0" :width="step - 4" height="56" fill="transparent" />
            <rect
              v-if="w.with_due && w.on_time"
              :x="i * step + 4"
              :y="56 - (52 * w.on_time) / w.with_due"
              :width="step - 8"
              :height="(52 * w.on_time) / w.with_due"
              rx="4"
              fill="currentColor"
            />
            <circle v-else-if="!w.with_due" :cx="i * step + step / 2" cy="52" r="2" class="fill-muted-foreground/50" />
          </g>
        </svg>
        <details class="text-xs">
          <summary class="cursor-pointer text-muted-foreground">{{ t('dashboard.charts.table') }}</summary>
          <table class="mt-2 w-full" data-testid="ontime-table">
            <thead>
              <tr class="text-left text-muted-foreground">
                <th class="py-1 font-medium">{{ t('dashboard.charts.week') }}</th>
                <th class="py-1 text-right font-medium">{{ t('dashboard.charts.withDue') }}</th>
                <th class="py-1 text-right font-medium">{{ t('dashboard.charts.onTimeCol') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="w in weeks" :key="w.start" class="border-t">
                <td class="py-1">{{ date(w.start) }}</td>
                <td class="py-1 text-right tabular-nums">{{ w.with_due }}</td>
                <td class="py-1 text-right tabular-nums">{{ w.with_due ? pct((100 * w.on_time) / w.with_due) : t('dashboard.charts.noData') }}</td>
              </tr>
            </tbody>
          </table>
        </details>
      </article>

      <!-- Load by project. -->
      <article class="flex flex-col gap-2 rounded-xl border bg-background p-4" data-testid="chart-load">
        <h3 class="text-sm font-semibold">{{ t('dashboard.charts.load') }}</h3>
        <p v-if="!load.length" class="text-sm text-muted-foreground">{{ owner ? t('dashboard.others.noLoad') : t('dashboard.charts.noLoad') }}</p>
        <template v-else>
          <ul class="flex gap-3 text-xs text-muted-foreground" :aria-label="t('dashboard.charts.loadAlt')">
            <li class="inline-flex items-center gap-1.5"><span class="size-2.5 rounded-sm bg-destructive" aria-hidden="true" />{{ t('dashboard.charts.lateSeries') }}</li>
            <li class="inline-flex items-center gap-1.5"><span class="size-2.5 rounded-sm bg-primary" aria-hidden="true" />{{ t('dashboard.charts.onTrackSeries') }}</li>
          </ul>
          <ul class="flex flex-col gap-2" role="img" :aria-label="t('dashboard.charts.loadAlt')">
            <li v-for="l in load" :key="l.project?.id ?? 'others'" class="flex flex-col gap-0.5 text-xs" data-testid="load-row">
              <span class="flex justify-between gap-2">
                <span class="truncate">{{ loadName(l) }}</span>
                <span class="shrink-0 tabular-nums">{{ l.open }}<template v-if="l.late"> · <span class="font-semibold text-destructive">{{ l.late }}</span></template></span>
              </span>
              <span class="flex h-2.5 gap-0.5" :style="{ width: `${(100 * l.open) / loadMax}%` }" :title="`${loadName(l)}: ${l.open} / ${l.late}`">
                <span v-if="l.late" class="h-full rounded-l bg-destructive" :class="l.late === l.open && 'rounded-r'" :style="{ flex: l.late }" />
                <span v-if="l.open > l.late" class="h-full rounded-r bg-primary" :class="!l.late && 'rounded-l'" :style="{ flex: l.open - l.late }" />
              </span>
            </li>
          </ul>
          <details class="text-xs">
            <summary class="cursor-pointer text-muted-foreground">{{ t('dashboard.charts.table') }}</summary>
            <table class="mt-2 w-full" data-testid="load-table">
              <thead>
                <tr class="text-left text-muted-foreground">
                  <th class="py-1 font-medium">{{ t('dashboard.charts.project') }}</th>
                  <th class="py-1 text-right font-medium">{{ t('dashboard.charts.open') }}</th>
                  <th class="py-1 text-right font-medium">{{ t('dashboard.charts.late') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="l in load" :key="l.project?.id ?? 'others'" class="border-t">
                  <td class="py-1">{{ loadName(l) }}</td>
                  <td class="py-1 text-right tabular-nums">{{ l.open }}</td>
                  <td class="py-1 text-right tabular-nums">{{ l.late }}</td>
                </tr>
              </tbody>
            </table>
          </details>
        </template>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import type { WorkloadBar, WorkloadRow } from '@/api/workload'
import { Button } from '@/components/ui/button'
import { PX, ticks, toDay, weekend, type Scale } from '@/features/timeline/dates'
import ItemPanel from '@/features/workitems/ItemPanel.vue'

// Cronograma por persona (spec 021, RF-1 to RF-15): a row per person with the
// bars of their open work items, read only, and the day they are free. With
// byProject, each project takes a colour of a fixed palette (RF-25).
const props = defineProps<{
  rows: WorkloadRow[]
  today: string
  /** Whose panel the viewer may open (RF-14, RF-15). */
  linkable: (userId: string) => boolean
  byProject?: boolean
}>()
const { t, locale } = useI18n()
const route = useRoute()

const SCALES: Scale[] = ['days', 'weeks']
const scale = ref<Scale>('weeks')
const px = computed(() => PX[scale.value])
const LEFT = 260
const LANE = 28
const now = computed(() => toDay(props.today))

// Some weeks around today and every bar (RF-12).
const range = computed(() => {
  const days = [now.value - 14, now.value + 56]
  for (const r of props.rows) for (const b of r.bars) days.push(toDay(b.start), toDay(b.end))
  return { from: Math.min(...days) - 7, to: Math.max(...days) + 7 }
})
const width = computed(() => (range.value.to - range.value.from + 1) * px.value)
const x = (day: number) => (day - range.value.from) * px.value
const headerTicks = computed(() => ticks(range.value.from, range.value.to, scale.value, locale.value))
const weekends = computed(() => (scale.value === 'days' ? headerTicks.value.filter((tk) => weekend(tk.day)) : []))

// Overlapping bars of one person go to separate lanes.
const lines = computed(() =>
  props.rows.map((r) => {
    const ends: number[] = []
    const bars = [...r.bars].sort((a, b) => a.start.localeCompare(b.start)).map((b) => {
      const start = toDay(b.start)
      let lane = ends.findIndex((e) => e < start)
      if (lane < 0) lane = ends.push(0) - 1
      ends[lane] = toDay(b.end)
      return { bar: b, lane }
    })
    return { row: r, bars, lanes: Math.max(ends.length, 1) }
  }),
)

// Projects by first appearance; past the palette, «Otros» (RF-25).
const COLORS = 7
const projects = computed(() => {
  const seen = new Map<string, { id: string; name: string; identifier: string }>()
  // Without a project, its own entry (spec 022, RF-36).
  for (const r of props.rows)
    for (const b of r.bars)
      if (!b.hidden && !seen.has(b.project.id)) seen.set(b.project.id, b.loose ? { ...b.project, identifier: '', name: t('loose.noProject') } : b.project)
  return [...seen.values()].sort((a, b) => a.identifier.localeCompare(b.identifier))
})
const colorOf = computed(() => new Map(projects.value.map((p, i) => [p.id, i < COLORS ? i + 1 : 0])))
const legend = computed(() => {
  const out: { key: string; label: string; color: number }[] = projects.value
    .slice(0, COLORS)
    .map((p) => ({ key: p.id, label: p.identifier ? `${p.identifier} · ${p.name}` : p.name, color: colorOf.value.get(p.id)! }))
  if (projects.value.length > COLORS) out.push({ key: 'others', label: t('workload.others'), color: 0 })
  return out
})

const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }))
const date = (iso: string) => dateFmt.value.format(new Date(iso + 'T00:00:00Z'))
const free = (r: WorkloadRow) => (r.free_from ? t('workload.freeFrom', { date: date(r.free_from) }) : t('workload.free'))
const open = (b: WorkloadBar) => ({ query: { ...route.query, item: b.key } })
const describe = (b: WorkloadBar) =>
  // Of a project the viewer does not see: its dates only (spec 023, RF-26).
  b.hidden ? t('workload.hiddenBar', { start: date(b.start), end: date(b.end), ratio: b.done_ratio }) + (b.overdue ? ` · ${t('workload.overdue')}` : '') :
  t('workload.bar', { key: b.key, title: b.title, project: b.loose ? t('loose.noProject') : b.project.name, start: date(b.start), end: date(b.end), ratio: b.done_ratio }) +
  (b.overdue ? ` · ${t('workload.overdue')}` : '')

// The detail of the bar under the pointer or the focus (RF-8).
const tip = ref<{ text: string; left: number; top: number } | null>(null)
function show(e: MouseEvent | FocusEvent, b: WorkloadBar) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  tip.value = { text: describe(b), left: r.left, top: r.bottom + 4 }
}

// Opens on today (RF-12).
const scroller = ref<HTMLElement | null>(null)
function centre() {
  const el = scroller.value
  if (el) el.scrollLeft = Math.max(0, LEFT + x(now.value) - el.clientWidth / 2)
}
onMounted(centre)
watch(scale, () => nextTick(centre))
</script>

<template>
  <div class="flex flex-col gap-3" data-testid="workload">
    <div class="flex flex-wrap items-center gap-2">
      <div role="group" :aria-label="t('timeline.scale')" class="inline-flex overflow-hidden rounded-md border">
        <Button
          v-for="s in SCALES"
          :key="s"
          size="sm"
          :variant="scale === s ? 'default' : 'ghost'"
          class="rounded-none"
          :aria-pressed="scale === s"
          :data-testid="`workload-scale-${s}`"
          @click="scale = s"
        >
          {{ t(`timeline.scales.${s}`) }}
        </Button>
      </div>
      <ul v-if="byProject && legend.length" class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground" :aria-label="t('workload.legend')" data-testid="workload-legend">
        <li v-for="l in legend" :key="l.key" class="flex items-center gap-1.5">
          <span class="size-2.5 rounded-sm" :class="`ws-c${l.color}`" style="background: var(--ws)" aria-hidden="true" />{{ l.label }}
        </li>
      </ul>
    </div>

    <div ref="scroller" class="overflow-x-auto rounded-xl border bg-background">
      <div :style="{ width: `${LEFT + width}px` }">
        <div class="sticky top-0 z-10 flex border-b bg-muted/40 text-xs text-muted-foreground">
          <div class="sticky left-0 z-10 shrink-0 border-r bg-muted px-3 py-2 font-medium" :style="{ width: `${LEFT}px` }">{{ t('workload.person') }}</div>
          <div class="relative h-8 shrink-0" :style="{ width: `${width}px` }">
            <span
              v-for="tk in headerTicks"
              :key="tk.day"
              class="absolute top-0 h-full overflow-hidden border-l py-2 whitespace-nowrap"
              :class="[scale === 'days' ? 'px-0.5 text-center' : 'px-1.5', scale === 'days' && weekend(tk.day) && 'bg-muted/60']"
              :style="{ left: `${x(tk.day)}px`, width: scale === 'days' ? `${px}px` : undefined }"
            >{{ tk.label }}</span>
          </div>
        </div>

        <div class="relative">
          <div
            v-for="w in weekends"
            :key="w.day"
            class="pointer-events-none absolute top-0 bottom-0 bg-muted/40"
            :style="{ left: `${LEFT + x(w.day)}px`, width: `${px}px` }"
            aria-hidden="true"
          />
          <div class="pointer-events-none absolute top-0 bottom-0 z-[1] w-0.5 bg-amber-600" :style="{ left: `${LEFT + x(now)}px` }" aria-hidden="true" data-testid="workload-today" />

          <div v-for="l in lines" :key="l.row.user.id" class="flex border-t border-border/50 first:border-t-0" data-testid="workload-row">
            <div class="sticky left-0 z-[3] flex shrink-0 flex-col justify-center gap-0.5 border-r bg-background px-3 py-1.5 text-sm" :style="{ width: `${LEFT}px` }">
              <RouterLink
                v-if="linkable(l.row.user.id)"
                :to="{ path: '/', query: { user: l.row.user.id } }"
                class="truncate font-medium hover:underline"
                data-testid="workload-person"
              >{{ l.row.user.name }}</RouterLink>
              <span v-else class="truncate font-medium" data-testid="workload-person">{{ l.row.user.name }}</span>
              <span class="text-xs text-muted-foreground">
                <span :class="!l.row.free_from && 'font-medium text-emerald-700 dark:text-emerald-400'" data-testid="workload-free">{{ free(l.row) }}</span>
                <template v-if="l.row.undated"> · <span data-testid="workload-undated">{{ t('workload.undated', { n: l.row.undated }, l.row.undated) }}</span></template>
              </span>
            </div>
            <div class="relative shrink-0" :style="{ width: `${width}px`, height: `${l.lanes * LANE + 12}px` }">
              <component
                :is="b.bar.hidden ? 'span' : RouterLink"
                v-for="(b, i) in l.bars"
                :key="b.bar.hidden ? `hidden-${i}` : b.bar.id"
                :to="b.bar.hidden ? undefined : open(b.bar)"
                :tabindex="b.bar.hidden ? 0 : undefined"
                class="absolute flex h-5 overflow-hidden rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                :class="[
                  byProject ? [`ws-c${b.bar.hidden ? 0 : (colorOf.get(b.bar.project.id) ?? 0)}`, 'ws-track', b.bar.overdue && 'ring-2 ring-destructive'] : b.bar.overdue ? 'bg-destructive/25' : 'bg-primary/25',
                ]"
                :style="{ left: `${x(toDay(b.bar.start))}px`, width: `${Math.max((toDay(b.bar.end) - toDay(b.bar.start) + 1) * px, 6)}px`, top: `${6 + b.lane * LANE}px` }"
                :aria-label="describe(b.bar)"
                :data-overdue="b.bar.overdue || undefined"
                data-testid="workload-bar"
                @mouseenter="show($event, b.bar)"
                @focus="show($event, b.bar)"
                @mouseleave="tip = null"
                @blur="tip = null"
              >
                <span class="h-full" :class="byProject ? 'ws-fill' : b.bar.overdue ? 'bg-destructive' : 'bg-primary'" :style="{ width: `${b.bar.done_ratio}%` }" />
              </component>
            </div>
          </div>
          <p v-if="!rows.length" class="p-6 text-sm text-muted-foreground">{{ t('workload.empty') }}</p>
        </div>
      </div>
    </div>

    <div
      v-if="tip"
      class="pointer-events-none fixed z-50 max-w-sm rounded-md border bg-popover px-2 py-1 text-xs text-popover-foreground shadow-md"
      :style="{ left: `${tip.left}px`, top: `${tip.top}px` }"
      role="status"
      data-testid="workload-tip"
    >{{ tip.text }}</div>
    <ItemPanel />
  </div>
</template>

<style scoped>
/* A fixed categorical palette, stepped for each mode; 0 is «Otros». */
.ws-c0 { --ws: #8a8984; }
.ws-c1 { --ws: #2a78d6; }
.ws-c2 { --ws: #eb6834; }
.ws-c3 { --ws: #1baf7a; }
.ws-c4 { --ws: #eda100; }
.ws-c5 { --ws: #e87ba4; }
.ws-c6 { --ws: #008300; }
.ws-c7 { --ws: #6250d6; }
:global(.dark) .ws-c1 { --ws: #3987e5; }
:global(.dark) .ws-c2 { --ws: #d95926; }
:global(.dark) .ws-c3 { --ws: #199e70; }
:global(.dark) .ws-c4 { --ws: #c98500; }
:global(.dark) .ws-c5 { --ws: #d55181; }
:global(.dark) .ws-c7 { --ws: #9085e9; }
.ws-track { background: color-mix(in oklab, var(--ws) 30%, transparent); }
.ws-fill { background: var(--ws); }
</style>

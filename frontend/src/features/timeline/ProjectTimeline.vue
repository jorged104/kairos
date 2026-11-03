<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { filteredTree, type Filters } from '@/api/projectScreens'
import { listMilestones } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useCatalog } from '@/features/work/useCatalog'
import { patchWorkItem } from '@/features/workitems/patch'
import ItemPanel from '@/features/workitems/ItemPanel.vue'
import type { Project, WorkNode } from '@/types/work'
import { PX, dragged, fromDay, ticks, toDay, today as todayDay, type DragMode, type Scale } from './dates'

// The work items with dates as bars on a time line, grouped by effective
// milestone; dragging a bar moves its dates, its ends change one (spec 008,
// RF-70 to RF-79).
const props = withDefaults(defineProps<{ project: Project; filters?: Filters }>(), { filters: () => ({}) })
const { t, locale } = useI18n()
const qc = useQueryClient()
const route = useRoute()
const errorMessage = useErrorMessage()
const { catalog } = useCatalog()

const { data: nodes } = useQuery({
  queryKey: computed(() => ['tree', props.project.id, 'filtered', props.filters, '']),
  queryFn: () => filteredTree(props.project.id, props.filters),
})
const { data: milestones } = useQuery({ queryKey: computed(() => ['milestones', props.project.id]), queryFn: () => listMilestones(props.project.id) })
const writable = computed(() => props.project.can.write_work_items && props.project.lifecycle === 'activo')

const scale = ref<Scale>('months')
const showDeps = ref(true)
const px = computed(() => PX[scale.value])
const now = todayDay()
const LEFT = 300
const ROW = 36

const dated = computed(() => (nodes.value ?? []).filter((n) => n.start_date && n.due_date))
const undated = computed(() => (nodes.value ?? []).filter((n) => !n.start_date || !n.due_date))

// The range: the project, the bars and today, with some margin.
const range = computed(() => {
  const days = [now]
  if (props.project.start_date) days.push(toDay(props.project.start_date))
  if (props.project.end_date) days.push(toDay(props.project.end_date))
  for (const n of dated.value) days.push(toDay(n.start_date!), toDay(n.due_date!))
  for (const m of milestones.value ?? []) if (m.due_date) days.push(toDay(m.due_date))
  return { from: Math.min(...days) - 7, to: Math.max(...days) + 14 }
})
const width = computed(() => (range.value.to - range.value.from + 1) * px.value)
const x = (day: number) => (day - range.value.from) * px.value
const headerTicks = computed(() => ticks(range.value.from, range.value.to, scale.value, locale.value))

// Groups by effective milestone, then «Sin hito» (RF-70).
const groups = computed(() => {
  const byId = new Map<string, WorkNode[]>()
  const none: WorkNode[] = []
  for (const n of dated.value) {
    if (!n.milestone_ids?.length) none.push(n)
    for (const m of n.milestone_ids ?? []) byId.set(m, [...(byId.get(m) ?? []), n])
  }
  const out = (milestones.value ?? [])
    .filter((m) => byId.has(m.id))
    .sort((a, b) => (a.due_date ?? '9').localeCompare(b.due_date ?? '9'))
    .map((m) => ({ key: m.id, name: m.name, due: m.due_date, status: m.status, nodes: byId.get(m.id)! }))
  if (none.length) out.push({ key: '', name: t('timeline.noMilestone'), due: null, status: 'abierto' as const, nodes: none })
  return out
})

// Late, blocked or behind is red (RF-71).
const isClosed = (n: WorkNode) => !!catalog.value.statuses.find((s) => s.id === n.status_id)?.is_closed
function atRisk(n: WorkNode) {
  if (isClosed(n)) return false
  const start = toDay(n.start_date!)
  const end = toDay(n.due_date!)
  if (end < now || n.blocked) return true
  const gone = Math.min(Math.max(now - start + 1, 0), end - start + 1)
  return n.derived_done_ratio - Math.round((100 * gone) / (end - start + 1)) < -10
}

// Dragging (RF-75 to RF-78).
const error = ref<unknown>(null)
const drag = ref<{ id: string; mode: DragMode; x0: number; start: number; end: number; delta: number; invalid: boolean } | null>(null)
function begin(e: PointerEvent, n: WorkNode, mode: DragMode) {
  if (!writable.value || e.button !== 0) return
  e.preventDefault()
  ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
  drag.value = { id: n.id, mode, x0: e.clientX, start: toDay(n.start_date!), end: toDay(n.due_date!), delta: 0, invalid: false }
}
function moveDrag(e: PointerEvent) {
  const d = drag.value
  if (!d) return
  const delta = Math.round((e.clientX - d.x0) / px.value)
  d.invalid = dragged(d.start, d.end, d.mode, delta) === null
  if (!d.invalid) d.delta = delta
}
async function endDrag() {
  const d = drag.value
  drag.value = null
  if (!d || d.delta === 0) return
  await save(d.id, dragged(d.start, d.end, d.mode, d.delta))
}
async function save(id: string, next: { start: number; end: number } | null) {
  if (!next) return
  error.value = null
  try {
    await patchWorkItem(id, { start_date: fromDay(next.start), due_date: fromDay(next.end) })
  } catch (e) {
    error.value = e // the bar goes back (RF-78)
  }
  qc.invalidateQueries({ queryKey: ['overview', props.project.id] })
  await qc.invalidateQueries({ queryKey: ['tree', props.project.id] })
}
// The same with the keyboard: arrows move it a day, with Shift only its end.
function onKey(e: KeyboardEvent, n: WorkNode) {
  const delta = e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowRight' ? 1 : 0
  if (!delta || !writable.value) return
  e.preventDefault()
  void save(n.id, dragged(toDay(n.start_date!), toDay(n.due_date!), e.shiftKey ? 'end' : 'move', delta))
}
const bar = (n: WorkNode) => {
  let start = toDay(n.start_date!)
  let end = toDay(n.due_date!)
  const d = drag.value
  if (d && d.id === n.id) {
    const next = dragged(d.start, d.end, d.mode, d.delta)
    if (next) ({ start, end } = next)
  }
  return { left: x(start), width: (end - start + 1) * px.value }
}

// Dependencies between visible bars; a «precedes» whose dates fail is red (RF-79).
const rowsY = computed(() => {
  const at = new Map<string, number>()
  let row = 0
  for (const g of groups.value) {
    row++ // the group's heading
    for (const n of g.nodes) {
      if (!at.has(n.id)) at.set(n.id, row * ROW + ROW / 2)
      row++
    }
  }
  return { at, height: row * ROW }
})
const arrows = computed(() => {
  if (!showDeps.value) return []
  const byId = new Map(dated.value.map((n) => [n.id, n]))
  const out: { d: string; bad: boolean; key: string }[] = []
  for (const from of dated.value) {
    for (const s of from.successors ?? []) {
      const to = byId.get(s.to_id)
      const y1 = rowsY.value.at.get(from.id)
      const y2 = rowsY.value.at.get(s.to_id)
      if (!to || y1 === undefined || y2 === undefined) continue
      const a = bar(from)
      const b = bar(to)
      const x1 = a.left + a.width
      const x2 = b.left
      const bad = s.type === 'precedes' && toDay(to.start_date!) < toDay(from.due_date!) + s.lag_days
      out.push({ key: `${from.id}-${s.to_id}`, bad, d: `M${x1},${y1} h8 V${y2} H${Math.max(x2 - 2, x1 + 10)}` })
    }
  }
  return out
})
const date = (iso: string | null) => (iso ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(new Date(iso + 'T00:00:00')) : '')
const open = (n: WorkNode) => ({ query: { ...route.query, item: n.key } })
</script>

<template>
  <div class="flex flex-col gap-3" data-testid="timeline">
    <div class="flex flex-wrap items-center gap-2">
      <div role="group" :aria-label="t('timeline.scale')" class="inline-flex overflow-hidden rounded-md border">
        <Button
          v-for="s in ['weeks', 'months', 'quarters'] as Scale[]"
          :key="s"
          size="sm"
          :variant="scale === s ? 'default' : 'ghost'"
          class="rounded-none"
          :aria-pressed="scale === s"
          :data-testid="`timeline-scale-${s}`"
          @click="scale = s"
        >
          {{ t(`timeline.scales.${s}`) }}
        </Button>
      </div>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="showDeps" type="checkbox" class="size-4 accent-primary" data-testid="timeline-deps" />
        {{ t('timeline.dependencies') }}
      </label>
      <span class="ml-auto text-xs text-muted-foreground">{{ t('timeline.hint') }}</span>
    </div>
    <Alert v-if="error" variant="destructive" role="alert" data-testid="timeline-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <div class="overflow-x-auto rounded-xl border bg-background" @pointermove="moveDrag" @pointerup="endDrag" @pointercancel="drag = null">
      <div :style="{ width: `${LEFT + width}px` }">
        <div class="sticky top-0 z-10 flex border-b bg-muted/40 text-xs text-muted-foreground">
          <div class="sticky left-0 z-10 shrink-0 border-r bg-muted px-3 py-2 font-medium" :style="{ width: `${LEFT}px` }">{{ t('timeline.task') }}</div>
          <div class="relative h-8 shrink-0" :style="{ width: `${width}px` }">
            <span v-for="tk in headerTicks" :key="tk.day" class="absolute top-0 h-full border-l px-1.5 py-2 whitespace-nowrap" :style="{ left: `${x(tk.day)}px` }">{{ tk.label }}</span>
          </div>
        </div>

        <div class="relative">
          <div class="pointer-events-none absolute top-0 bottom-0 z-[1] w-0.5 bg-amber-600" :style="{ left: `${LEFT + x(now)}px` }" aria-hidden="true" data-testid="timeline-today" />
          <svg v-if="arrows.length" class="pointer-events-none absolute top-0 z-[2]" :style="{ left: `${LEFT}px` }" :width="width" :height="rowsY.height" aria-hidden="true">
            <defs>
              <marker id="tl-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto"><path d="M0,0 L8,4 L0,8 z" class="fill-muted-foreground" /></marker>
              <marker id="tl-arrow-bad" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto"><path d="M0,0 L8,4 L0,8 z" class="fill-destructive" /></marker>
            </defs>
            <path v-for="a in arrows" :key="a.key" :d="a.d" fill="none" :class="a.bad ? 'stroke-destructive' : 'stroke-muted-foreground'" stroke-width="1.5" :marker-end="a.bad ? 'url(#tl-arrow-bad)' : 'url(#tl-arrow)'" data-testid="timeline-arrow" />
          </svg>

          <template v-for="g in groups" :key="g.key">
            <div class="flex bg-muted/30" :style="{ height: `${ROW}px` }" data-testid="timeline-group">
              <div class="sticky left-0 z-[3] flex shrink-0 items-center gap-2 border-r bg-muted/60 px-3 text-sm font-semibold" :style="{ width: `${LEFT}px` }">
                <span class="truncate">{{ g.name }}</span>
                <span class="ml-auto text-xs font-normal text-muted-foreground">{{ date(g.due) }}</span>
              </div>
              <div class="relative shrink-0" :style="{ width: `${width}px` }">
                <span
                  v-if="g.due"
                  class="absolute top-2.5 size-4 rotate-45 rounded-[3px]"
                  :class="g.status === 'cumplido' ? 'bg-slate-500' : 'bg-emerald-500'"
                  :style="{ left: `${x(toDay(g.due)) - 8}px` }"
                  role="img"
                  :aria-label="t('timeline.milestoneOn', { name: g.name, date: date(g.due) })"
                />
              </div>
            </div>
            <div v-for="n in g.nodes" :key="g.key + n.id" class="flex border-t border-border/50" :style="{ height: `${ROW}px` }" data-testid="timeline-row">
              <div class="sticky left-0 z-[3] flex shrink-0 items-center gap-2 border-r bg-background px-3 pl-6 text-sm" :style="{ width: `${LEFT}px` }">
                <span class="font-mono text-[11px] text-muted-foreground">{{ n.key }}</span>
                <RouterLink :to="open(n)" class="truncate hover:underline">{{ n.title }}</RouterLink>
              </div>
              <div class="relative shrink-0" :style="{ width: `${width}px` }">
                <div
                  class="absolute top-2 flex h-5 overflow-hidden rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                  :class="[atRisk(n) ? 'bg-red-200 dark:bg-red-950' : 'bg-muted-foreground/25', writable && 'cursor-grab', drag?.id === n.id && drag.invalid && 'ring-2 ring-destructive']"
                  :style="{ left: `${bar(n).left}px`, width: `${Math.max(bar(n).width, 6)}px` }"
                  :tabindex="writable ? 0 : -1"
                  role="slider"
                  :aria-label="t('timeline.bar', { key: n.key, start: date(n.start_date ?? null), end: date(n.due_date) })"
                  :aria-valuenow="n.derived_done_ratio"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  data-testid="timeline-bar"
                  @pointerdown="begin($event, n, 'move')"
                  @keydown="onKey($event, n)"
                >
                  <div class="h-full" :class="atRisk(n) ? 'bg-destructive' : 'bg-foreground'" :style="{ width: `${n.derived_done_ratio}%` }" />
                  <template v-if="writable">
                    <span class="absolute inset-y-0 left-0 w-2 cursor-ew-resize" data-testid="timeline-start" @pointerdown.stop="begin($event, n, 'start')" />
                    <span class="absolute inset-y-0 right-0 w-2 cursor-ew-resize" data-testid="timeline-end" @pointerdown.stop="begin($event, n, 'end')" />
                  </template>
                </div>
              </div>
            </div>
          </template>
          <p v-if="!groups.length" class="p-6 text-sm text-muted-foreground">{{ t('timeline.empty') }}</p>
        </div>
      </div>
    </div>

    <section v-if="undated.length" class="flex flex-col gap-2 rounded-xl border bg-background p-4" :aria-label="t('timeline.undated')" data-testid="timeline-undated">
      <h3 class="text-sm font-semibold">{{ t('timeline.undated') }} <span class="font-normal text-muted-foreground">· {{ undated.length }}</span></h3>
      <ul class="flex flex-col gap-1 text-sm">
        <li v-for="n in undated" :key="n.id">
          <RouterLink :to="open(n)" class="hover:underline"><span class="font-mono text-xs text-muted-foreground">{{ n.key }}</span> {{ n.title }}</RouterLink>
        </li>
      </ul>
    </section>
    <ItemPanel />
  </div>
</template>

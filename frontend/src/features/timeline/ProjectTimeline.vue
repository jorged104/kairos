<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ChevronDown, ChevronRight, Plus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { filteredTree, type Filters } from '@/api/projectScreens'
import { createWorkItem, linkMilestoneWorkItem, listMilestones } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { filterDefaults } from '@/features/views/useProjectFilters'
import { useCatalog } from '@/features/work/useCatalog'
import { patchWorkItem } from '@/features/workitems/patch'
import ItemPanel from '@/features/workitems/ItemPanel.vue'
import { allParents, childrenOf, visibleRows, type TreeRow } from '@/features/workitems/tree'
import { useAuthStore } from '@/stores/auth'
import type { Project, WorkNode } from '@/types/work'
import { PX, SCALES, dragged, fromDay, ticks, toDay, today as todayDay, weekend, type DragMode, type Scale } from './dates'

// The work items with dates as bars on a time line, grouped by effective
// milestone and in their hierarchy; dragging a bar moves its dates, its ends
// change one, and new ones are made in place (spec 008, RF-70 to RF-89).
const props = withDefaults(defineProps<{ project: Project; filters?: Filters }>(), { filters: () => ({}) })
const { t, locale } = useI18n()
const qc = useQueryClient()
const route = useRoute()
const auth = useAuthStore()
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

const hasDates = (n: WorkNode) => !!n.start_date && !!n.due_date
const dated = computed(() => (nodes.value ?? []).filter(hasDates))
const undated = computed(() => (nodes.value ?? []).filter((n) => !hasDates(n)))
const byId = computed(() => new Map((nodes.value ?? []).map((n) => [n.id, n])))

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
const weekends = computed(() => (scale.value === 'days' ? headerTicks.value.filter((tk) => weekend(tk.day)) : []))

// Each group keeps the hierarchy: a dated parent from another group leaves
// its children at the top, an undated one is shown to hold them (RF-83, RF-84).
const collapsed = ref(new Set<string>())
function toggle(id: string) {
  const next = new Set(collapsed.value)
  if (!next.delete(id)) next.add(id)
  collapsed.value = next
}
function treeOf(members: WorkNode[]) {
  const set = new Map(members.map((n) => [n.id, n]))
  for (const n of members) {
    for (let p = n.parent_id ? byId.value.get(n.parent_id) : undefined; p && !hasDates(p) && !set.has(p.id); p = p.parent_id ? byId.value.get(p.parent_id) : undefined) set.set(p.id, p)
  }
  const children = childrenOf([...set.values()])
  const open = allParents(children)
  for (const id of collapsed.value) open.delete(id)
  return { rows: visibleRows(children, open), children }
}

type Line = { kind: 'item'; row: TreeRow; summary: { start: number; end: number } | null } | { kind: 'new'; parent: WorkNode | null; level: number }
interface Group { key: string; name: string; due: string | null; status: string; lines: Line[] }
const adding = ref<{ group: string; parent: string | null } | null>(null)
const groups = computed<Group[]>(() => {
  const ms = new Map<string, WorkNode[]>()
  const none: WorkNode[] = []
  for (const n of dated.value) {
    if (!n.milestone_ids?.length) none.push(n)
    for (const m of n.milestone_ids ?? []) ms.set(m, [...(ms.get(m) ?? []), n])
  }
  const raw = (milestones.value ?? [])
    .filter((m) => ms.has(m.id))
    .sort((a, b) => (a.due_date ?? '9').localeCompare(b.due_date ?? '9'))
    .map((m) => ({ key: m.id, name: m.name, due: m.due_date, status: m.status, members: ms.get(m.id)! }))
  if (none.length || writable.value) raw.push({ key: '', name: t('timeline.noMilestone'), due: null, status: 'abierto', members: none })
  return raw.map((g) => {
    const { rows, children } = treeOf(g.members)
    // The span of the dated ones under an undated parent.
    const span = (id: string): { start: number; end: number } | null => {
      let out: { start: number; end: number } | null = null
      for (const c of children.get(id) ?? []) {
        const s = hasDates(c) ? { start: toDay(c.start_date!), end: toDay(c.due_date!) } : span(c.id)
        if (s) out = out ? { start: Math.min(out.start, s.start), end: Math.max(out.end, s.end) } : s
      }
      return out
    }
    const lines: Line[] = []
    for (const row of rows) {
      lines.push({ kind: 'item', row, summary: hasDates(row.node) ? null : span(row.node.id) })
      if (adding.value?.group === g.key && adding.value.parent === row.node.id) lines.push({ kind: 'new', parent: row.node, level: row.level + 1 })
    }
    if (writable.value) lines.push({ kind: 'new', parent: null, level: 0 })
    return { key: g.key, name: g.name, due: g.due, status: g.status, lines }
  })
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

// Dragging (RF-75 to RF-78), with the dates it would get by the pointer (RF-82).
const error = ref<unknown>(null)
const drag = ref<{ id: string; mode: DragMode; x0: number; start: number; end: number; delta: number; invalid: boolean; cx: number; cy: number } | null>(null)
function begin(e: PointerEvent, n: WorkNode, mode: DragMode) {
  if (!writable.value || e.button !== 0) return
  e.preventDefault()
  ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
  drag.value = { id: n.id, mode, x0: e.clientX, start: toDay(n.start_date!), end: toDay(n.due_date!), delta: 0, invalid: false, cx: e.clientX, cy: e.clientY }
}
function moveDrag(e: PointerEvent) {
  const d = drag.value
  if (!d) return
  const delta = Math.round((e.clientX - d.x0) / px.value)
  d.invalid = dragged(d.start, d.end, d.mode, delta) === null
  if (!d.invalid) d.delta = delta
  d.cx = e.clientX
  d.cy = e.clientY
}
async function endDrag() {
  const d = drag.value
  drag.value = null
  if (!d || d.delta === 0) return
  await save(d.id, dragged(d.start, d.end, d.mode, d.delta))
}
const tip = computed(() => {
  const d = drag.value
  if (!d) return null
  const next = dragged(d.start, d.end, d.mode, d.delta)!
  return { left: d.cx + 14, top: d.cy + 18, mode: d.mode, start: longDate(next.start), end: longDate(next.end), days: next.end - next.start + 1 }
})
async function refresh() {
  qc.invalidateQueries({ queryKey: ['overview', props.project.id] })
  qc.invalidateQueries({ queryKey: ['views', props.project.id] })
  await qc.invalidateQueries({ queryKey: ['tree', props.project.id] })
}
async function save(id: string, next: { start: number; end: number } | null) {
  if (!next) return
  error.value = null
  try {
    await patchWorkItem(id, { start_date: fromDay(next.start), due_date: fromDay(next.end) })
  } catch (e) {
    error.value = e // the bar goes back (RF-78)
  }
  await refresh()
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

// Making them here: a week from today at the foot of a group, the parent's
// dates for a sub-item, and the filters with one value (RF-86 to RF-89).
const newTitle = ref('')
const firstType = computed(() => props.project.work_item_type_ids.find((id) => catalog.value.work_item_types.find((ty) => ty.id === id && !ty.retired)) ?? '')
const isAdding = (g: Group, parent: WorkNode | null) => adding.value?.group === g.key && adding.value.parent === (parent?.id ?? null)
function startAdding(g: Group, parent: WorkNode | null) {
  adding.value = { group: g.key, parent: parent?.id ?? null }
  newTitle.value = ''
  if (parent && collapsed.value.has(parent.id)) toggle(parent.id)
}
async function create(g: Group, parent: WorkNode | null) {
  const title = newTitle.value.trim()
  if (!title) return
  error.value = null
  const fromParent = parent && hasDates(parent)
  const { values, milestoneId } = filterDefaults(props.filters, auth.me?.id)
  try {
    const created = await createWorkItem(props.project.id, {
      title,
      type_id: firstType.value,
      parent_id: parent?.id ?? null,
      start_date: fromParent ? parent.start_date! : fromDay(now),
      due_date: fromParent ? parent.due_date! : fromDay(now + 6),
      ...values,
    })
    // A sub-item takes its parent's milestones by itself.
    const milestone = parent ? undefined : g.key || milestoneId
    if (milestone && milestone !== 'none') await linkMilestoneWorkItem(milestone, created.id).catch(() => undefined)
    newTitle.value = ''
    if (parent) adding.value = null
  } catch (e) {
    error.value = e
  }
  await qc.invalidateQueries({ queryKey: ['milestones', props.project.id] })
  await refresh()
}
function cancelAdding() {
  adding.value = null
  newTitle.value = ''
}

// Dependencies between visible bars; a «precedes» whose dates fail is red (RF-79).
const rowsY = computed(() => {
  const at = new Map<string, number>()
  let row = 0
  for (const g of groups.value) {
    row++ // the group's heading
    for (const l of g.lines) {
      if (l.kind === 'item' && !at.has(l.row.node.id) && hasDates(l.row.node)) at.set(l.row.node.id, row * ROW + ROW / 2)
      row++
    }
  }
  return { at, height: row * ROW }
})
const arrows = computed(() => {
  if (!showDeps.value) return []
  const out: { d: string; bad: boolean; key: string }[] = []
  for (const from of dated.value) {
    for (const s of from.successors ?? []) {
      const to = byId.value.get(s.to_id)
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
const longDate = (day: number) => new Intl.DateTimeFormat(locale.value, { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(fromDay(day) + 'T00:00:00Z'))
const open = (n: WorkNode) => ({ query: { ...route.query, item: n.key } })
const indent = (level: number) => ({ paddingLeft: `${12 + level * 16}px` })
</script>

<template>
  <div class="flex flex-col gap-3" data-testid="timeline">
    <div class="flex flex-wrap items-center gap-2">
      <div role="group" :aria-label="t('timeline.scale')" class="inline-flex overflow-hidden rounded-md border">
        <Button
          v-for="s in SCALES"
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
            <span
              v-for="tk in headerTicks"
              :key="tk.day"
              class="absolute top-0 h-full overflow-hidden border-l py-2 whitespace-nowrap"
              :class="[scale === 'days' ? 'px-0.5 text-center' : 'px-1.5', scale === 'days' && weekend(tk.day) && 'bg-muted/60']"
              :style="{ left: `${x(tk.day)}px`, width: scale === 'days' ? `${px}px` : undefined }"
              data-testid="timeline-tick"
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
            <template v-for="(l, i) in g.lines" :key="l.kind === 'item' ? g.key + l.row.node.id : `${g.key}-new-${l.parent?.id ?? ''}-${i}`">
              <div v-if="l.kind === 'item'" class="group flex border-t border-border/50" :style="{ height: `${ROW}px` }" data-testid="timeline-row" :data-level="l.row.level">
                <div class="sticky left-0 z-[3] flex shrink-0 items-center gap-1.5 border-r bg-background pr-2 text-sm" :style="{ width: `${LEFT}px`, ...indent(l.row.level) }">
                  <button
                    v-if="l.row.hasChildren"
                    type="button"
                    class="flex size-4 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-muted"
                    :aria-expanded="l.row.expanded"
                    :aria-label="t(l.row.expanded ? 'timeline.collapse' : 'timeline.expand', { key: l.row.node.key })"
                    data-testid="timeline-toggle"
                    @click="toggle(l.row.node.id)"
                  >
                    <ChevronDown v-if="l.row.expanded" class="size-3.5" aria-hidden="true" />
                    <ChevronRight v-else class="size-3.5" aria-hidden="true" />
                  </button>
                  <span v-else class="size-4 shrink-0" />
                  <span class="font-mono text-[11px] text-muted-foreground">{{ l.row.node.key }}</span>
                  <RouterLink :to="open(l.row.node)" class="truncate hover:underline" :class="l.row.hasChildren && 'font-medium'">{{ l.row.node.title }}</RouterLink>
                  <button
                    v-if="writable"
                    type="button"
                    class="ml-auto flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground opacity-0 group-hover:opacity-100 hover:bg-muted focus-visible:opacity-100"
                    :aria-label="t('timeline.addChild', { key: l.row.node.key })"
                    data-testid="timeline-add-child"
                    @click="startAdding(g, l.row.node)"
                  >
                    <Plus class="size-3.5" aria-hidden="true" />
                  </button>
                </div>
                <div class="relative shrink-0" :style="{ width: `${width}px` }">
                  <div
                    v-if="hasDates(l.row.node)"
                    class="absolute top-2 flex h-5 overflow-hidden rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                    :class="[atRisk(l.row.node) ? 'bg-red-200 dark:bg-red-950' : 'bg-muted-foreground/25', writable && 'cursor-grab', drag?.id === l.row.node.id && drag.invalid && 'ring-2 ring-destructive']"
                    :style="{ left: `${bar(l.row.node).left}px`, width: `${Math.max(bar(l.row.node).width, 6)}px` }"
                    :tabindex="writable ? 0 : -1"
                    role="slider"
                    :aria-label="t('timeline.bar', { key: l.row.node.key, start: date(l.row.node.start_date ?? null), end: date(l.row.node.due_date) })"
                    :aria-valuenow="l.row.node.derived_done_ratio"
                    aria-valuemin="0"
                    aria-valuemax="100"
                    data-testid="timeline-bar"
                    @pointerdown="begin($event, l.row.node, 'move')"
                    @keydown="onKey($event, l.row.node)"
                  >
                    <div class="h-full" :class="atRisk(l.row.node) ? 'bg-destructive' : 'bg-foreground'" :style="{ width: `${l.row.node.derived_done_ratio}%` }" />
                    <template v-if="writable">
                      <span class="absolute inset-y-0 left-0 w-2 cursor-ew-resize" data-testid="timeline-start" @pointerdown.stop="begin($event, l.row.node, 'start')" />
                      <span class="absolute inset-y-0 right-0 w-2 cursor-ew-resize" data-testid="timeline-end" @pointerdown.stop="begin($event, l.row.node, 'end')" />
                    </template>
                  </div>
                  <div
                    v-else-if="l.summary"
                    class="absolute top-3.5 h-1.5 rounded-sm border-x-2 border-foreground/60 bg-foreground/30"
                    :style="{ left: `${x(l.summary.start)}px`, width: `${(l.summary.end - l.summary.start + 1) * px}px` }"
                    :title="t('timeline.summary')"
                    data-testid="timeline-summary"
                  />
                </div>
              </div>
              <div v-else class="flex border-t border-border/50" :style="{ height: `${ROW}px` }" data-testid="timeline-new">
                <div class="sticky left-0 z-[3] flex shrink-0 items-center border-r bg-background pr-2 text-sm" :style="{ width: `${LEFT}px`, ...indent(l.level) }">
                  <form v-if="isAdding(g, l.parent)" class="flex w-full items-center gap-1" @submit.prevent="create(g, l.parent)">
                    <input
                      v-model="newTitle"
                      class="h-7 w-full rounded-md border bg-background px-2 text-sm"
                      :placeholder="t('timeline.newTitle')"
                      :aria-label="t('timeline.newTitle')"
                      data-testid="timeline-new-title"
                      autofocus
                      @keydown.esc="cancelAdding"
                    />
                  </form>
                  <Button v-else size="sm" variant="ghost" class="h-7 justify-start px-1 text-muted-foreground" data-testid="timeline-add" @click="startAdding(g, null)">
                    <Plus aria-hidden="true" />{{ t('timeline.add') }}
                  </Button>
                </div>
                <div class="shrink-0" :style="{ width: `${width}px` }" />
              </div>
            </template>
          </template>
          <p v-if="!groups.length" class="p-6 text-sm text-muted-foreground">{{ t('timeline.empty') }}</p>
        </div>
      </div>
    </div>

    <div
      v-if="tip"
      class="pointer-events-none fixed z-50 rounded-md border bg-popover px-2 py-1 text-xs text-popover-foreground shadow-md"
      :class="drag?.invalid && 'border-destructive'"
      :style="{ left: `${tip.left}px`, top: `${tip.top}px` }"
      role="status"
      data-testid="timeline-tip"
    >
      <span :class="tip.mode !== 'end' && 'font-semibold'">{{ tip.start }}</span>
      →
      <span :class="tip.mode !== 'start' && 'font-semibold'">{{ tip.end }}</span>
      <span class="text-muted-foreground"> · {{ t('timeline.days', { count: tip.days }, tip.days) }}</span>
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

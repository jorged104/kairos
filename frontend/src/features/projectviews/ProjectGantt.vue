<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { BoardProgram, BoardProject } from '@/api/projectBoard'
import { getProject, updateProject } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import { useLevels } from '@/features/schedule/useLevels'
import type { ProjectStatus } from '@/types/work'
import type { Group } from './filters'

// The Timeline of projects: a bar from start to end filled with the real
// progress and colored by the traffic light, the baseline under it, the
// milestones and today; whoever manages a project drags its bar to change its
// dates (spec 012, RF-46 to RF-59).
const props = defineProps<{
  rows: BoardProject[]
  today: string
  group: Group
  programs: BoardProgram[]
  statuses: ProjectStatus[]
}>()
const { t, locale } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const levels = useLevels()

type Scale = 'weeks' | 'months' | 'quarters'
const scale = ref<Scale>('months')
const showBaseline = ref(true)
const showMilestones = ref(true)
const PX: Record<Scale, number> = { weeks: 20, months: 4, quarters: 1.4 } // per day

const DAY = 86_400_000
const at = (d: string) => Date.parse(d + 'T00:00:00Z')
const iso = (ms: number) => new Date(ms).toISOString().slice(0, 10)
const range = computed(() => {
  const days = [at(props.today)]
  for (const p of props.rows) {
    for (const d of [p.start_date, p.end_date, p.base_start, p.base_end]) if (d) days.push(at(d))
    for (const m of p.milestones) days.push(at(m.due_date))
  }
  const a = new Date(Math.min(...days))
  const b = new Date(Math.max(...days))
  const from = Date.UTC(a.getUTCFullYear(), a.getUTCMonth() - (scale.value === 'quarters' ? a.getUTCMonth() % 3 : 0), 1)
  const to = Date.UTC(b.getUTCFullYear(), b.getUTCMonth() + 1, 1)
  return { from, to, days: Math.round((to - from) / DAY) }
})
const width = computed(() => Math.max(600, range.value.days * PX[scale.value]))
const x = (d: string) => ((at(d) - range.value.from) / DAY) * PX[scale.value]

const ticks = computed(() => {
  const out: { left: number; label: string }[] = []
  const r = range.value
  if (scale.value === 'weeks') {
    const fmt = new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' })
    let d = r.from + ((8 - new Date(r.from).getUTCDay()) % 7) * DAY // the first Monday
    for (; d < r.to; d += 7 * DAY) out.push({ left: ((d - r.from) / DAY) * PX.weeks, label: fmt.format(d) })
    return out
  }
  const step = scale.value === 'months' ? 1 : 3
  const fmt = new Intl.DateTimeFormat(locale.value, { month: 'short', year: '2-digit', timeZone: 'UTC' })
  for (let d = new Date(r.from); d.getTime() < r.to; d.setUTCMonth(d.getUTCMonth() + step)) {
    const label = step === 1 ? fmt.format(d) : t('projectViews.quarter', { q: Math.floor(d.getUTCMonth() / 3) + 1, y: String(d.getUTCFullYear()).slice(2) })
    out.push({ left: ((d.getTime() - r.from) / DAY) * PX[scale.value], label })
  }
  return out
})

// The groups of rows (RF-51, RF-52).
interface RowGroup {
  key: string
  name: string
  figures?: BoardProgram['figures']
  rows: BoardProject[]
}
const groups = computed<RowGroup[]>(() => {
  const by = (key: (p: BoardProject) => string, name: (k: string, p: BoardProject) => string) => {
    const m = new Map<string, RowGroup>()
    for (const p of props.rows) {
      const k = key(p)
      if (!m.has(k)) m.set(k, { key: k, name: name(k, p), rows: [] })
      m.get(k)!.rows.push(p)
    }
    return [...m.values()]
  }
  switch (props.group) {
    case 'none':
      return [{ key: 'all', name: '', rows: props.rows }]
    case 'owner':
      return by((p) => p.owner.id, (_, p) => p.owner.name).sort((a, b) => a.name.localeCompare(b.name))
    case 'status': {
      const order = props.statuses.map((s) => s.id)
      return by((p) => p.status_id, (k) => props.statuses.find((s) => s.id === k)?.name ?? k).sort((a, b) => order.indexOf(a.key) - order.indexOf(b.key))
    }
    default: {
      const gs = by((p) => p.program?.id ?? 'none', (k, p) => (k === 'none' ? t('projectViews.noProgram') : p.program!.name))
      for (const g of gs) g.figures = props.programs.find((x) => x.id === g.key)?.figures
      return gs.sort((a, b) => (a.key === 'none' ? 1 : b.key === 'none' ? -1 : a.name.localeCompare(b.name)))
    }
  }
})

const draggable = (p: BoardProject) => p.can_manage && p.lifecycle === 'activo' && !!p.start_date && !!p.end_date

// Dragging: an edge moves one date, the bar moves both (RF-55, RF-56).
const drag = ref<{ id: string; edge: 'start' | 'end' | 'both'; x0: number; days: number } | null>(null)
const shift = (p: BoardProject, edge: 'start' | 'end') => {
  const d = drag.value
  return d && d.id === p.id && (d.edge === edge || d.edge === 'both') ? d.days : 0
}
const startOf = (p: BoardProject) => iso(at(p.start_date!) + shift(p, 'start') * DAY)
const endOf = (p: BoardProject) => iso(at(p.end_date!) + shift(p, 'end') * DAY)
function down(e: PointerEvent, p: BoardProject, edge: 'start' | 'end' | 'both') {
  if (!draggable(p)) return
  e.preventDefault()
  ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
  drag.value = { id: p.id, edge, x0: e.clientX, days: 0 }
}
function moveTo(e: PointerEvent) {
  if (drag.value) drag.value.days = Math.round((e.clientX - drag.value.x0) / PX[scale.value])
}
async function up(p: BoardProject) {
  const d = drag.value
  if (!d || d.id !== p.id) return
  const start = startOf(p)
  const end = endOf(p)
  drag.value = null
  if (d.days !== 0) await save(p, start, end)
}
// The keyboard: arrows move the bar, Shift the end and Alt the start.
async function key(e: KeyboardEvent, p: BoardProject) {
  if (!draggable(p) || (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight')) return
  e.preventDefault()
  const n = e.key === 'ArrowLeft' ? -1 : 1
  const s = e.shiftKey ? 0 : n
  const f = e.altKey ? 0 : n
  await save(p, iso(at(p.start_date!) + s * DAY), iso(at(p.end_date!) + f * DAY))
}

const error = ref<unknown>(null)
const saving = ref<string | null>(null)
async function save(p: BoardProject, start: string, end: string) {
  error.value = null
  if (end < start) {
    error.value = 'invalid_date_range'
    return
  }
  saving.value = p.id
  try {
    // The PATCH replaces every editable field, so the rest is read fresh.
    const cur = await getProject(p.id)
    await updateProject(p.id, {
      version: cur.version, name: cur.name, team_id: cur.team.id, owner_id: cur.owner.id, parent_id: cur.parent?.id ?? null,
      start_date: start, end_date: end, health: cur.health, health_explanation: cur.health_explanation, custom_values: cur.custom_values,
    })
    qc.invalidateQueries({ queryKey: ['project', p.id] })
  } catch (e) {
    error.value = e // the bar goes back (RF-57)
  } finally {
    saving.value = null
    await qc.invalidateQueries({ queryKey: ['project-board'] })
  }
}

const color = (p: BoardProject) => (p.figures.has_plan && p.figures.level ? levels.classes(p.figures.level).solid : 'bg-muted-foreground/50')
const date = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }).format(at(d))
</script>

<template>
  <section class="flex flex-col gap-3" data-testid="project-gantt">
    <div class="flex flex-wrap items-center gap-3 text-sm">
      <div role="group" :aria-label="t('projectViews.scale')" class="flex gap-0.5 rounded-lg bg-muted p-0.5">
        <button
          v-for="s in (['weeks', 'months', 'quarters'] as const)"
          :key="s"
          type="button"
          class="h-8 rounded-md px-3 text-xs"
          :class="scale === s ? 'bg-background font-semibold shadow-xs' : 'text-muted-foreground'"
          :aria-pressed="scale === s"
          :data-testid="`gantt-scale-${s}`"
          @click="scale = s"
        >
          {{ t(`projectViews.scales.${s}`) }}
        </button>
      </div>
      <label class="flex items-center gap-2"><input v-model="showBaseline" type="checkbox" class="size-4 accent-primary" data-testid="gantt-baseline" />{{ t('projectViews.baseline') }}</label>
      <label class="flex items-center gap-2"><input v-model="showMilestones" type="checkbox" class="size-4 accent-primary" data-testid="gantt-milestones" />{{ t('projectViews.milestones') }}</label>
    </div>
    <Alert v-if="error" variant="destructive" role="alert" data-testid="project-gantt-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <div class="overflow-x-auto rounded-xl border bg-background">
      <div :style="{ width: `${width + 288}px` }">
        <div class="flex border-b text-xs text-muted-foreground">
          <div class="sticky left-0 z-10 w-72 shrink-0 bg-background px-3 py-2 font-medium">{{ t('portfolio.project') }}</div>
          <div class="relative h-8" :style="{ width: `${width}px` }">
            <span v-for="tk in ticks" :key="tk.left" class="absolute top-0 bottom-0 border-l px-1.5 py-2 whitespace-nowrap" :style="{ left: `${tk.left}px` }">{{ tk.label }}</span>
          </div>
        </div>

        <div v-for="g in groups" :key="g.key" data-testid="gantt-group">
          <div v-if="group !== 'none'" class="sticky left-0 flex flex-wrap items-center gap-2 border-b bg-muted/50 px-3 py-1.5 text-sm">
            <span class="font-semibold">{{ g.name }}</span>
            <span class="text-xs text-muted-foreground">· {{ t('portfolio.projects', { count: g.rows.length }, g.rows.length) }}</span>
            <LevelBadge v-if="g.figures?.has_plan" :figures="g.figures" />
          </div>
          <div v-for="p in g.rows" :key="p.id" class="flex border-b last:border-b-0" data-testid="gantt-row" :data-id="p.id">
            <div class="sticky left-0 z-10 flex w-72 shrink-0 flex-col justify-center gap-0.5 bg-background px-3 py-1.5">
              <RouterLink :to="`/projects/${p.id}`" class="truncate text-sm font-medium hover:underline" data-testid="gantt-link">
                <span class="font-mono text-[11px] text-muted-foreground">{{ p.identifier }}</span> {{ p.name }}
              </RouterLink>
              <span class="truncate text-xs text-muted-foreground">{{ statuses.find((s) => s.id === p.status_id)?.name }} · {{ p.owner.name }}</span>
            </div>
            <div class="relative h-12" :style="{ width: `${width}px` }">
              <span v-for="tk in ticks" :key="tk.left" class="absolute top-0 bottom-0 w-px bg-muted" :style="{ left: `${tk.left}px` }" aria-hidden="true" />
              <!-- The baseline under the bar (RF-49). -->
              <span
                v-if="showBaseline && p.base_start && p.base_end"
                class="absolute top-8 h-1.5 rounded bg-muted-foreground/30"
                :style="{ left: `${x(p.base_start)}px`, width: `${x(p.base_end) - x(p.base_start) + PX[scale]}px` }"
                :title="t('projectViews.baseRange', { from: date(p.base_start), to: date(p.base_end) })"
                data-testid="gantt-base"
              />
              <div
                v-if="p.start_date && p.end_date"
                class="group absolute top-2 h-5 rounded-md bg-muted outline-offset-2 focus-visible:outline-2 focus-visible:outline-ring"
                :class="[draggable(p) && 'cursor-grab', saving === p.id && 'opacity-60']"
                :style="{ left: `${x(startOf(p))}px`, width: `${Math.max(4, x(endOf(p)) - x(startOf(p)) + PX[scale])}px` }"
                :tabindex="draggable(p) ? 0 : -1"
                role="img"
                :aria-label="t('projectViews.barLabel', { name: p.name, from: date(startOf(p)), to: date(endOf(p)), real: p.figures.real })"
                :aria-keyshortcuts="draggable(p) ? 'ArrowLeft ArrowRight Shift+ArrowLeft Shift+ArrowRight Alt+ArrowLeft Alt+ArrowRight' : undefined"
                :title="draggable(p) ? t('projectViews.dragDates') : undefined"
                data-testid="gantt-bar"
                @pointerdown="down($event, p, 'both')"
                @pointermove="moveTo"
                @pointerup="up(p)"
                @keydown="key($event, p)"
              >
                <div class="h-full rounded-md" :class="color(p)" :style="{ width: `${p.figures.real}%` }" />
                <span
                  v-if="p.figures.has_plan"
                  class="absolute -top-1 h-7 w-0.5 bg-foreground"
                  :style="{ left: `${p.figures.plan}%` }"
                  :title="t('projectViews.plan', { plan: p.figures.plan })"
                  aria-hidden="true"
                />
                <template v-if="draggable(p)">
                  <span class="absolute inset-y-0 left-0 w-2 cursor-ew-resize" data-testid="gantt-start" aria-hidden="true" @pointerdown.stop="down($event, p, 'start')" @pointermove="moveTo" @pointerup.stop="up(p)" />
                  <span class="absolute inset-y-0 right-0 w-2 cursor-ew-resize" data-testid="gantt-end" aria-hidden="true" @pointerdown.stop="down($event, p, 'end')" @pointermove="moveTo" @pointerup.stop="up(p)" />
                </template>
              </div>
              <span v-else class="absolute top-3 left-2 text-xs text-muted-foreground" data-testid="gantt-nodates">{{ t('projectViews.noDates') }}</span>
              <template v-if="showMilestones">
                <span
                  v-for="m in p.milestones"
                  :key="m.id"
                  class="absolute top-3.5 size-2.5 -translate-x-1/2 rotate-45 border-2 border-foreground"
                  :class="m.done ? 'bg-foreground' : 'bg-background'"
                  :style="{ left: `${x(m.due_date) + PX[scale] / 2}px` }"
                  :title="t('projectViews.milestone', { name: m.name, date: date(m.due_date) })"
                  data-testid="gantt-milestone"
                />
              </template>
              <span class="absolute top-0 bottom-0 w-0.5 bg-orange-600" :style="{ left: `${x(today)}px` }" aria-hidden="true" />
            </div>
          </div>
        </div>
        <p v-if="!rows.length" class="p-4 text-sm text-muted-foreground">{{ t('projectViews.empty') }}</p>
      </div>
    </div>
    <p class="text-xs text-muted-foreground">{{ t('projectViews.ganttHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { onClickOutside } from '@vueuse/core'
import { MoreHorizontal } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { setProjectStatus, type BoardProject } from '@/api/projectBoard'
import { LEVELS } from '@/api/schedule'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import { useLevels } from '@/features/schedule/useLevels'
import { HEALTHS, type ProjectStatus } from '@/types/work'
import { levelOf, type Columns } from './filters'

// The Kanban of projects: a column per status of the catalog, or by traffic
// light, health or program; dragging a card, or «Mover a», changes the status
// of the project (spec 012, RF-33 to RF-45).
const props = defineProps<{ rows: BoardProject[]; columns: Columns; statuses: ProjectStatus[] }>()
const { t, locale } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const levels = useLevels()

const MAX = 100 // per column (RF-45)

interface Column {
  key: string
  name: string
  color?: string
  retired?: boolean
  rows: BoardProject[]
}
const byStatus = computed(() => props.columns === 'status')
const cols = computed<Column[]>(() => {
  const of = (match: (p: BoardProject) => boolean) => props.rows.filter(match)
  switch (props.columns) {
    case 'level':
      return [...LEVELS, 'none' as const].map((l) => ({
        key: l, name: l === 'none' ? t('schedule.noPlan') : levels.name(l), rows: of((p) => levelOf(p) === l),
      }))
    case 'health':
      return HEALTHS.map((h) => ({ key: h, name: t(`projects.healths.${h}`), rows: of((p) => p.health === h) }))
    case 'program': {
      const programs = new Map<string, string>()
      for (const p of props.rows) if (p.program) programs.set(p.program.id, p.program.name)
      return [
        ...[...programs].sort((a, b) => a[1].localeCompare(b[1])).map(([id, name]) => ({ key: id, name, rows: of((p) => p.program?.id === id) })),
        { key: 'none', name: t('projectViews.noProgram'), rows: of((p) => !p.program) },
      ]
    }
    default: {
      // The active statuses, and the retired ones some project still has (RF-34).
      const used = new Set(props.rows.map((p) => p.status_id))
      return props.statuses
        .filter((s) => !s.retired || used.has(s.id))
        .map((s) => ({ key: s.id, name: s.name, color: s.color, retired: s.retired, rows: of((p) => p.status_id === s.id) }))
    }
  }
})
const movable = (p: BoardProject) => byStatus.value && p.can_manage && p.lifecycle !== 'archivado'
const targets = computed(() => props.statuses.filter((s) => !s.retired))

const error = ref<unknown>(null)
const moving = ref<string | null>(null)
async function move(p: BoardProject, statusId: string) {
  if (statusId === p.status_id) return
  error.value = null
  moving.value = p.id
  try {
    await setProjectStatus(p.id, p.version, statusId)
    qc.invalidateQueries({ queryKey: ['project', p.id] })
  } catch (e) {
    // The card goes back: the board is read again as it is (RF-39).
    error.value = e
  } finally {
    moving.value = null
    await qc.invalidateQueries({ queryKey: ['project-board'] })
  }
}

const dragging = ref<BoardProject | null>(null)
function onDragStart(e: DragEvent, p: BoardProject) {
  dragging.value = p
  e.dataTransfer?.setData('text/plain', p.id)
}
async function onDrop(col: Column) {
  const p = dragging.value
  dragging.value = null
  if (!p || col.retired || !byStatus.value) return
  await move(p, col.key)
}

// «Mover a», the same move without dragging (RF-41).
const menu = ref<string | null>(null)
const menuEl = ref<HTMLElement | null>(null)
onClickOutside(menuEl, () => (menu.value = null))

const date = (d: string | null) =>
  d ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z')) : t('projectViews.noEnd')
const pct = (n: number) => `${n.toLocaleString('es', { maximumFractionDigits: 1 })} %`
const initials = (name: string) =>
  name
    .split(/\s+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
    .toUpperCase()
</script>

<template>
  <div class="flex flex-col gap-3" data-testid="project-board">
    <Alert v-if="error" variant="destructive" role="alert" data-testid="project-board-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <div class="overflow-x-auto pb-2">
      <div class="grid auto-cols-[17rem] grid-flow-col gap-3">
        <section
          v-for="col in cols"
          :key="col.key"
          class="flex flex-col gap-2 rounded-xl bg-muted/60 p-2 transition-shadow duration-200"
          :class="byStatus && dragging && dragging.status_id !== col.key && 'ring-2 ring-primary/30'"
          :aria-label="col.name"
          data-testid="project-board-column"
          :data-key="col.key"
          @dragover.prevent
          @drop.prevent="onDrop(col)"
        >
          <header class="flex items-center gap-2 px-1 py-0.5">
            <span v-if="col.color" class="size-2.5 rounded-full" :style="{ background: col.color }" aria-hidden="true" />
            <h3 class="text-sm font-semibold">{{ col.name }}</h3>
            <span v-if="col.retired" class="text-xs text-muted-foreground">({{ t('catalog.retired') }})</span>
            <span class="ml-auto rounded-full bg-background px-2 text-xs font-semibold tabular-nums" data-testid="project-board-count">{{ col.rows.length }}</span>
          </header>

          <TransitionGroup tag="div" name="card" class="relative flex flex-col gap-2">
            <article
              v-for="p in col.rows.slice(0, MAX)"
              :key="p.id"
              class="relative flex flex-col gap-2 rounded-lg border bg-background p-3 shadow-xs transition-[translate,box-shadow,opacity,scale] duration-150 hover:-translate-y-0.5 hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0"
              :class="[(moving === p.id && 'opacity-60') || (dragging?.id === p.id && 'scale-95 opacity-50'), menu === p.id && 'z-30']"
              :draggable="movable(p)"
              data-testid="project-card"
              @dragstart="onDragStart($event, p)"
              @dragend="dragging = null"
            >
              <div class="flex items-center gap-1.5">
                <span class="font-mono text-[11px] text-muted-foreground">{{ p.identifier }}</span>
                <span v-if="p.is_program" class="rounded bg-muted px-1.5 text-[11px] text-muted-foreground">{{ t('portfolio.program') }}</span>
                <span v-else-if="p.program" class="truncate rounded bg-muted px-1.5 text-[11px] text-muted-foreground">{{ p.program.name }}</span>
                <button
                  v-if="movable(p)"
                  type="button"
                  class="ml-auto grid size-7 shrink-0 place-items-center rounded hover:bg-muted"
                  :aria-label="t('projectViews.moveTo', { name: p.name })"
                  :aria-expanded="menu === p.id"
                  data-testid="project-card-move"
                  @click="menu = menu === p.id ? null : p.id"
                >
                  <MoreHorizontal class="size-4" aria-hidden="true" />
                </button>
              </div>
              <RouterLink :to="`/projects/${p.id}`" class="text-sm leading-snug font-semibold hover:underline" data-testid="project-card-link">{{ p.name }}</RouterLink>
              <!-- The real progress with the plan of today marked on it (RF-36). -->
              <div class="relative h-1.5 rounded-full bg-muted" aria-hidden="true">
                <div class="h-full rounded-full bg-primary" :style="{ width: `${p.figures.real}%` }" />
                <div v-if="p.figures.has_plan" class="absolute -top-0.5 h-2.5 w-0.5 bg-foreground" :style="{ left: `${p.figures.plan}%` }" />
              </div>
              <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                <span>{{ t('projectViews.real', { real: pct(p.figures.real) }) }}</span>
                <span v-if="p.figures.has_plan">· {{ t('projectViews.plan', { plan: pct(p.figures.plan) }) }}</span>
                <span class="ml-auto">
                  <LevelBadge v-if="p.figures.has_plan" :figures="p.figures" compact />
                  <span v-else data-testid="project-card-noplan">{{ t('schedule.noPlan') }}</span>
                </span>
              </div>
              <div class="flex items-center gap-2 text-xs text-muted-foreground">
                <span class="grid size-6 place-items-center rounded-full bg-muted text-[10px] font-semibold text-foreground" aria-hidden="true">{{ initials(p.owner.name) }}</span>
                <span class="truncate">{{ p.owner.name }}</span>
                <span class="ml-auto shrink-0">{{ date(p.end_date) }}</span>
              </div>
              <div
                v-if="menu === p.id"
                :ref="(el) => (menuEl = el as HTMLElement | null)"
                class="absolute top-10 right-2 z-20 flex w-52 flex-col rounded-lg border bg-popover p-1 shadow-lg animate-in fade-in-0 zoom-in-95 slide-in-from-top-1 duration-150 motion-reduce:animate-none"
                role="menu"
                data-testid="project-card-menu"
              >
                <p class="px-2 py-1 text-xs text-muted-foreground">{{ t('workItems.moveTo') }}</p>
                <button
                  v-for="s in targets.filter((x) => x.id !== p.status_id)"
                  :key="s.id"
                  type="button"
                  role="menuitem"
                  class="flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted"
                  @click="(menu = null), move(p, s.id)"
                >
                  <span class="size-2 rounded-full" :style="{ background: s.color }" aria-hidden="true" />{{ s.name }}
                </button>
              </div>
            </article>
          </TransitionGroup>
          <p v-if="!col.rows.length" class="px-2 py-3 text-center text-xs text-muted-foreground">{{ t('projectViews.emptyColumn') }}</p>
          <p v-if="col.rows.length > MAX" class="px-2 text-xs text-muted-foreground" data-testid="project-board-more">
            {{ t('projectViews.more', { n: col.rows.length - MAX }) }}
          </p>
        </section>
      </div>
    </div>
    <p v-if="byStatus" class="text-xs text-muted-foreground">{{ t('projectViews.dragHint') }}</p>
  </div>
</template>

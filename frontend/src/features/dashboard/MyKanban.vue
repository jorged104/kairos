<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { onClickOutside } from '@vueuse/core'
import { MoreHorizontal } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { DashboardTask } from '@/api/dashboard'
import { getWorkItem } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { patchWorkItem } from '@/features/workitems/patch'
import type { Status } from '@/types/work'

// The caller's work items in a column per status of the catalog, the most
// late first; dragging or «Mover a» changes the status with the same flow and
// permissions as in the project (spec 013, RF-8 to RF-22).
const props = defineProps<{ tasks: DashboardTask[]; statuses: Status[] }>()
const { t, locale } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const MAX = 100 // per column (RF-21)

// RF-12, RF-13.
function order(a: DashboardTask, b: DashboardTask) {
  if (a.closed) return (b.closed_on ?? '').localeCompare(a.closed_on ?? '')
  if (a.late_days !== b.late_days) return b.late_days - a.late_days
  if (a.due_date !== b.due_date) return !a.due_date ? 1 : !b.due_date ? -1 : a.due_date.localeCompare(b.due_date)
  return a.key.localeCompare(b.key, undefined, { numeric: true })
}
const columns = computed(() => {
  const used = new Set(props.tasks.map((x) => x.status_id))
  return props.statuses
    .filter((s) => !s.retired || used.has(s.id))
    .map((s) => ({ status: s, tasks: props.tasks.filter((x) => x.status_id === s.id).sort(order) }))
})

const error = ref<unknown>(null)
const moving = ref<string | null>(null)
async function move(task: DashboardTask, statusId: string) {
  if (statusId === task.status_id) return
  error.value = null
  moving.value = task.id
  try {
    await patchWorkItem(task.id, { status_id: statusId })
  } catch (e) {
    error.value = e // the card goes back: the panel is read again (RF-19)
  } finally {
    moving.value = null
    await qc.invalidateQueries({ queryKey: ['dashboard'] })
  }
}

// Dragging: the statuses its flow allows come with the work item (RF-17, RF-18).
const dragging = ref<{ task: DashboardTask; allowed: string[] | null } | null>(null)
async function onDragStart(e: DragEvent, task: DashboardTask) {
  if (!task.can_move) return e.preventDefault()
  e.dataTransfer?.setData('text/plain', task.id)
  dragging.value = { task, allowed: null }
  const w = await getWorkItem(task.id)
  if (dragging.value?.task.id === task.id) dragging.value.allowed = w.allowed_status_ids
}
const allowed = (statusId: string) => {
  const d = dragging.value
  if (!d || d.task.status_id === statusId) return true
  return d.allowed === null || d.allowed.includes(statusId)
}
async function onDrop(status: Status) {
  const d = dragging.value
  const ok = allowed(status.id) && !status.retired
  dragging.value = null
  if (d && ok) await move(d.task, status.id)
}

// «Mover a», the same move without dragging (RF-20).
const menu = ref<{ id: string; allowed: string[] } | null>(null)
const menuEl = ref<HTMLElement | null>(null)
onClickOutside(menuEl, () => (menu.value = null))
async function openMenu(task: DashboardTask) {
  if (menu.value?.id === task.id) return (menu.value = null)
  try {
    const w = await getWorkItem(task.id)
    menu.value = { id: task.id, allowed: w.allowed_status_ids }
  } catch (e) {
    error.value = e
  }
}

const date = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z'))
</script>

<template>
  <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="my-tasks" data-testid="my-kanban">
    <h2 id="my-tasks" class="text-base font-semibold">{{ t('dashboard.tasks.title') }}</h2>
    <Alert v-if="error" variant="destructive" role="alert" data-testid="my-kanban-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <p v-if="!tasks.length" class="text-sm text-muted-foreground" data-testid="my-kanban-empty">{{ t('dashboard.tasks.empty') }}</p>
    <template v-else>
      <div class="overflow-x-auto pb-2">
        <div class="grid auto-cols-[16rem] grid-flow-col gap-3">
          <section
            v-for="c in columns"
            :key="c.status.id"
            class="flex flex-col gap-2 rounded-xl bg-muted/60 p-2 transition-opacity"
            :class="dragging && !allowed(c.status.id) && 'opacity-40'"
            :aria-label="c.status.name"
            :aria-disabled="dragging && !allowed(c.status.id) ? 'true' : undefined"
            data-testid="my-column"
            :data-status="c.status.id"
            @dragover.prevent
            @drop.prevent="onDrop(c.status)"
          >
            <header class="flex items-center gap-2 px-1 py-0.5">
              <span class="size-2.5 rounded-full" :style="{ background: c.status.color }" aria-hidden="true" />
              <h3 class="text-sm font-semibold">{{ c.status.name }}</h3>
              <span v-if="c.status.retired" class="text-xs text-muted-foreground">({{ t('dashboard.tasks.retired') }})</span>
              <span class="ml-auto rounded-full bg-background px-2 text-xs font-semibold tabular-nums" data-testid="my-column-count">{{ c.tasks.length }}</span>
            </header>
            <p v-if="c.status.is_closed" class="px-1 text-[11px] text-muted-foreground">{{ t('dashboard.tasks.closedHint') }}</p>

            <article
              v-for="task in c.tasks.slice(0, MAX)"
              :key="task.id"
              class="relative flex flex-col gap-1.5 rounded-lg border bg-background p-3 shadow-xs"
              :class="moving === task.id && 'opacity-60'"
              :draggable="task.can_move"
              data-testid="my-task"
              @dragstart="onDragStart($event, task)"
              @dragend="dragging = null"
            >
              <div class="flex items-center gap-1.5">
                <span class="font-mono text-[11px] text-muted-foreground">{{ task.key }}</span>
                <span class="truncate text-[11px] text-muted-foreground">· {{ task.project.name }}</span>
                <button
                  v-if="task.can_move"
                  type="button"
                  class="ml-auto grid size-7 shrink-0 place-items-center rounded hover:bg-muted"
                  :aria-label="t('dashboard.tasks.moveTo', { key: task.key })"
                  :aria-expanded="menu?.id === task.id"
                  data-testid="my-task-move"
                  @click="openMenu(task)"
                >
                  <MoreHorizontal class="size-4" aria-hidden="true" />
                </button>
              </div>
              <RouterLink :to="`/work-items/${task.key}`" class="text-sm leading-snug font-medium hover:underline" data-testid="my-task-link">{{ task.title }}</RouterLink>
              <p class="text-xs">
                <span v-if="task.closed && task.closed_on" class="text-muted-foreground">{{ t('dashboard.tasks.closedOn', { date: date(task.closed_on) }) }}</span>
                <span v-else-if="task.late_days" class="font-semibold text-destructive" data-testid="my-task-late">
                  {{ t('dashboard.tasks.late', { count: task.late_days }, task.late_days) }}
                </span>
                <span v-else-if="task.due_date" class="text-muted-foreground">{{ t('dashboard.tasks.due', { date: date(task.due_date) }) }}</span>
                <span v-else class="text-muted-foreground">{{ t('dashboard.tasks.noDue') }}</span>
              </p>
              <div
                v-if="menu?.id === task.id"
                :ref="(el) => (menuEl = el as HTMLElement | null)"
                class="absolute top-10 right-2 z-20 flex w-48 flex-col rounded-lg border bg-popover p-1 shadow-lg"
                role="menu"
                data-testid="my-task-menu"
              >
                <p class="px-2 py-1 text-xs text-muted-foreground">{{ t('workItems.moveTo') }}</p>
                <button
                  v-for="s in statuses.filter((x) => !x.retired && menu!.allowed.includes(x.id) && x.id !== task.status_id)"
                  :key="s.id"
                  type="button"
                  role="menuitem"
                  class="flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted"
                  @click="(menu = null), move(task, s.id)"
                >
                  <span class="size-2 rounded-full" :style="{ background: s.color }" aria-hidden="true" />{{ s.name }}
                </button>
                <p v-if="!menu.allowed.length" class="px-2 py-1 text-xs text-muted-foreground">{{ t('dashboard.tasks.noTargets') }}</p>
              </div>
            </article>
            <p v-if="c.tasks.length > MAX" class="px-2 text-xs text-muted-foreground" data-testid="my-column-more">{{ t('dashboard.tasks.more', { n: c.tasks.length - MAX }) }}</p>
          </section>
        </div>
      </div>
      <p class="text-xs text-muted-foreground">{{ t('dashboard.tasks.dragHint') }}</p>
    </template>
  </section>
</template>

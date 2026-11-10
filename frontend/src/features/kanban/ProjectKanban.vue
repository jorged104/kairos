<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { onClickOutside } from '@vueuse/core'
import { MoreHorizontal, Plus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { filteredTree, setColumnLimit, type Filters } from '@/api/projectScreens'
import { createWorkItem, getWorkItem, listMilestones } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import AvatarStack from '@/features/work/AvatarStack.vue'
import LabelChips from '@/features/work/LabelChips.vue'
import TypeStripe from '@/features/work/TypeStripe.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { patchWorkItem } from '@/features/workitems/patch'
import ItemPanel from '@/features/workitems/ItemPanel.vue'
import type { Label, Project, WorkNode } from '@/types/work'

// A column per active status; dragging a card, or «Mover a», changes its
// status within the type's workflow (spec 008, RF-60 to RF-69).
const props = withDefaults(defineProps<{ project: Project; filters?: Filters }>(), { filters: () => ({}) })
const { t, locale } = useI18n()
const qc = useQueryClient()
const route = useRoute()
const errorMessage = useErrorMessage()
const { catalog, sizes, labels, priorities, types } = useCatalog()

const key = computed(() => ['tree', props.project.id, 'filtered', props.filters, ''])
const { data: nodes } = useQuery({ queryKey: key, queryFn: () => filteredTree(props.project.id, props.filters) })
const { data: milestones } = useQuery({ queryKey: computed(() => ['milestones', props.project.id]), queryFn: () => listMilestones(props.project.id) })
const writable = computed(() => props.project.can.write_work_items && props.project.lifecycle === 'activo')

type Lane = 'none' | 'assignee' | 'milestone' | 'priority'
const lane = ref<Lane>('none')
const hideOld = ref(true)
const now = Date.now()

const statuses = computed(() => catalog.value.statuses.filter((s) => !s.retired).sort((a, b) => a.position - b.position))
const visible = computed(() =>
  (nodes.value ?? []).filter((n) => !(hideOld.value && n.closed_at && now - new Date(n.closed_at).getTime() > 14 * 86400000)),
)
const milestoneNames = computed(() => new Map((milestones.value ?? []).map((m) => [m.id, m.name])))
const lanes = computed(() => {
  const out = new Map<string, { name: string; nodes: WorkNode[] }>()
  const add = (k: string, name: string, n: WorkNode) => {
    if (!out.has(k)) out.set(k, { name, nodes: [] })
    out.get(k)!.nodes.push(n)
  }
  for (const n of visible.value) {
    if (lane.value === 'none') add('', '', n)
    else if (lane.value === 'priority') add(n.priority_id, priorities.value.get(n.priority_id)?.name ?? '', n)
    else if (lane.value === 'assignee') {
      if (!n.assignees.length) add('', t('workItems.unassigned'), n)
      for (const a of n.assignees) add(a.id, a.name, n)
    } else {
      if (!n.milestone_ids?.length) add('', t('timeline.noMilestone'), n)
      for (const m of n.milestone_ids ?? []) add(m, milestoneNames.value.get(m) ?? '', n)
    }
  }
  if (lane.value === 'none' && !out.size) out.set('', { name: '', nodes: [] })
  return [...out.entries()].map(([k, v]) => ({ key: k, ...v }))
})
const count = (statusId: string) => visible.value.filter((n) => n.status_id === statusId).length
const limit = (statusId: string) => props.project.column_limits?.[statusId]

const labelsOf = (n: WorkNode): Label[] => n.label_ids.map((id) => labels.value.find((l) => l.id === id)).filter((l): l is Label => !!l)
const due = (n: WorkNode) => (n.due_date ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(new Date(n.due_date + 'T00:00:00')) : '')
const late = (n: WorkNode) => !!n.due_date && !catalog.value.statuses.find((s) => s.id === n.status_id)?.is_closed && new Date(n.due_date + 'T23:59:59') < new Date()
const open = (n: WorkNode) => ({ query: { ...route.query, item: n.key } })

// Dragging: the allowed statuses come with the work item (RF-63).
const error = ref<unknown>(null)
const dragging = ref<{ node: WorkNode; allowed: string[] | null } | null>(null)
async function onDragStart(e: DragEvent, n: WorkNode) {
  if (!writable.value) return e.preventDefault()
  e.dataTransfer?.setData('text/plain', n.id)
  dragging.value = { node: n, allowed: null }
  const w = await getWorkItem(n.id)
  if (dragging.value?.node.id === n.id) dragging.value.allowed = w.allowed_status_ids
}
const allowed = (statusId: string) => {
  const d = dragging.value
  if (!d || d.node.status_id === statusId) return true
  return d.allowed === null || d.allowed.includes(statusId)
}
const refresh = () => {
  qc.invalidateQueries({ queryKey: ['overview', props.project.id] })
  qc.invalidateQueries({ queryKey: ['views', props.project.id] })
  return qc.invalidateQueries({ queryKey: ['tree', props.project.id] })
}
async function move(n: WorkNode, statusId: string) {
  error.value = null
  if (n.status_id === statusId) return
  try {
    await patchWorkItem(n.id, { status_id: statusId })
  } catch (e) {
    error.value = e // the card stays in its column (RF-64)
  }
  await refresh()
}
async function onDrop(statusId: string) {
  const d = dragging.value
  const ok = allowed(statusId)
  dragging.value = null
  if (!d || !ok) return
  await move(d.node, statusId)
}

// «Mover a», the same moves without dragging (RF-65).
const menu = ref<{ id: string; allowed: string[] } | null>(null)
const menuEl = ref<HTMLElement | null>(null)
onClickOutside(menuEl, () => (menu.value = null))
async function openMenu(n: WorkNode) {
  if (menu.value?.id === n.id) return (menu.value = null)
  const w = await getWorkItem(n.id)
  menu.value = { id: n.id, allowed: w.allowed_status_ids }
}

// A column's limit, for who manages the team (RF-66).
const editingLimit = ref<string | null>(null)
const limitDraft = ref('')
async function saveLimit(statusId: string) {
  const v = limitDraft.value.trim()
  editingLimit.value = null
  try {
    await setColumnLimit(props.project.id, statusId, v ? Number(v) : null)
    await qc.invalidateQueries({ queryKey: ['project', props.project.id] })
  } catch (e) {
    error.value = e
  }
}

// A new one in a column starts in its status (RF-69).
const adding = ref<string | null>(null)
const newTitle = ref('')
const firstType = computed(() => props.project.work_item_type_ids.find((id) => catalog.value.work_item_types.find((ty) => ty.id === id && !ty.retired)) ?? '')
async function create(statusId: string) {
  const title = newTitle.value.trim()
  if (!title) return
  error.value = null
  try {
    await createWorkItem(props.project.id, { title, type_id: firstType.value, status_id: statusId })
    newTitle.value = ''
    await refresh()
  } catch (e) {
    error.value = e
  }
}
const sizeName = (n: WorkNode) => (n.size_id ? (sizes.value.get(n.size_id)?.name ?? '') : '')
</script>

<template>
  <div class="flex flex-col gap-3" data-testid="kanban">
    <div class="flex flex-wrap items-center gap-3 text-sm">
      <label class="flex items-center gap-1.5 text-muted-foreground">
        {{ t('kanban.lanes') }}
        <select v-model="lane" class="h-8 rounded-md border bg-background px-2 text-sm text-foreground" data-testid="kanban-lanes">
          <option value="none">{{ t('kanban.laneNone') }}</option>
          <option value="assignee">{{ t('workItems.assignees') }}</option>
          <option value="milestone">{{ t('workItems.milestone') }}</option>
          <option value="priority">{{ t('workItems.priority') }}</option>
        </select>
      </label>
      <label class="flex items-center gap-2">
        <input v-model="hideOld" type="checkbox" class="size-4 accent-primary" data-testid="kanban-hide-old" />
        {{ t('kanban.hideOld') }}
      </label>
    </div>
    <Alert v-if="error" variant="destructive" role="alert" data-testid="kanban-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <div class="overflow-x-auto pb-2">
      <div class="flex min-w-max flex-col gap-4">
        <section v-for="ln in lanes" :key="ln.key" class="flex flex-col gap-2" :aria-label="ln.name || t('kanban.title')">
          <h3 v-if="lane !== 'none'" class="text-sm font-semibold">{{ ln.name }} <span class="font-normal text-muted-foreground">· {{ ln.nodes.length }}</span></h3>
          <div class="grid auto-cols-[18rem] grid-flow-col gap-3">
            <section
              v-for="st in statuses"
              :key="st.id"
              class="flex flex-col gap-2 rounded-xl bg-muted/60 p-2 transition-[opacity,box-shadow] duration-200"
              :class="dragging && (allowed(st.id) ? 'ring-2 ring-primary/30' : 'opacity-40')"
              :aria-label="st.name"
              :aria-disabled="dragging && !allowed(st.id) ? 'true' : undefined"
              data-testid="kanban-column"
              :data-status="st.id"
              @dragover.prevent
              @drop.prevent="onDrop(st.id)"
            >
              <header class="flex items-center gap-2 px-1 py-0.5">
                <span class="size-2.5 rounded-full" :style="{ background: st.color }" />
                <h4 class="text-sm font-semibold">{{ st.name }}</h4>
                <span class="text-xs text-muted-foreground">{{ ln.nodes.filter((n) => n.status_id === st.id).length }}</span>
                <template v-if="lane === 'none'">
                  <input
                    v-if="editingLimit === st.id"
                    v-model="limitDraft"
                    type="number"
                    min="1"
                    max="999"
                    class="ml-auto h-7 w-16 rounded border bg-background px-1 text-xs"
                    :aria-label="t('kanban.limit')"
                    data-testid="kanban-limit-input"
                    @keydown.enter.prevent="saveLimit(st.id)"
                    @blur="saveLimit(st.id)"
                  />
                  <button
                    v-else-if="project.can.manage_team || limit(st.id)"
                    type="button"
                    class="ml-auto rounded px-1 text-xs font-semibold"
                    :class="limit(st.id) && count(st.id) > limit(st.id)! ? 'text-destructive' : 'text-muted-foreground'"
                    :disabled="!project.can.manage_team"
                    data-testid="kanban-limit"
                    @click="(editingLimit = st.id), (limitDraft = String(limit(st.id) ?? ''))"
                  >
                    {{ limit(st.id) ? t('kanban.limitOf', { n: count(st.id), limit: limit(st.id) }) : t('kanban.setLimit') }}
                  </button>
                </template>
              </header>

              <TransitionGroup tag="div" name="card" class="relative flex flex-col gap-2">
                <article
                  v-for="n in ln.nodes.filter((x) => x.status_id === st.id)"
                  :key="n.id"
                  class="relative flex flex-col gap-2 rounded-lg border bg-background p-3 shadow-xs transition-[translate,box-shadow,opacity,scale] duration-150 hover:-translate-y-0.5 hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0"
                  :class="[dragging?.node.id === n.id && 'scale-95 opacity-50', menu?.id === n.id && 'z-30']"
                  :draggable="writable"
                  data-testid="kanban-card"
                  @dragstart="onDragStart($event, n)"
                  @dragend="dragging = null"
                >
                  <TypeStripe :type="types.get(n.type_id)" />
                  <div class="flex items-center gap-1.5">
                    <span class="font-mono text-[11px] text-muted-foreground">{{ n.key }}</span>
                    <span v-if="n.blocked" class="rounded-full bg-amber-100 px-1.5 text-[11px] font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-300">{{ t('kanban.blocked') }}</span>
                    <span v-if="sizeName(n)" class="ml-auto rounded border px-1.5 text-[11px] text-muted-foreground">{{ sizeName(n) }}</span>
                    <button
                      v-if="writable"
                      type="button"
                      class="grid size-7 place-items-center rounded hover:bg-muted"
                      :class="!sizeName(n) && 'ml-auto'"
                      :aria-label="t('kanban.moveTo', { key: n.key })"
                      :aria-expanded="menu?.id === n.id"
                      data-testid="kanban-move"
                      @click="openMenu(n)"
                    >
                      <MoreHorizontal class="size-4" aria-hidden="true" />
                    </button>
                  </div>
                  <RouterLink :to="open(n)" class="text-sm leading-snug font-medium hover:underline">{{ n.title }}</RouterLink>
                  <LabelChips v-if="n.label_ids.length" :labels="labelsOf(n)" :max="3" />
                  <div class="flex items-center gap-2">
                    <AvatarStack v-if="n.assignees.length" :people="n.assignees" :max="3" />
                    <span v-if="n.due_date" class="ml-auto text-xs" :class="late(n) ? 'font-semibold text-destructive' : 'text-muted-foreground'">{{ due(n) }}</span>
                  </div>
                  <div
                    v-if="menu?.id === n.id"
                    :ref="(el) => (menuEl = el as HTMLElement | null)"
                    class="absolute top-10 right-2 z-20 flex w-48 flex-col rounded-lg border bg-popover p-1 shadow-lg animate-in fade-in-0 zoom-in-95 slide-in-from-top-1 duration-150 motion-reduce:animate-none"
                    role="menu"
                    data-testid="kanban-move-menu"
                  >
                    <p class="px-2 py-1 text-xs text-muted-foreground">{{ t('workItems.moveTo') }}</p>
                    <button
                      v-for="s in statuses.filter((x) => menu!.allowed.includes(x.id))"
                      :key="s.id"
                      type="button"
                      role="menuitem"
                      class="flex items-center gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted"
                      @click="(menu = null), move(n, s.id)"
                    >
                      <span class="size-2 rounded-full" :style="{ background: s.color }" />{{ s.name }}
                    </button>
                    <p v-if="!menu.allowed.length" class="px-2 py-1 text-sm text-muted-foreground">{{ t('common.empty') }}</p>
                  </div>
                </article>
              </TransitionGroup>

              <template v-if="writable && lane === 'none' && !st.is_closed">
                <form v-if="adding === st.id" class="flex flex-col gap-1" @submit.prevent="create(st.id)">
                  <input
                    v-model="newTitle"
                    class="h-9 rounded-md border bg-background px-2 text-sm"
                    :placeholder="t('workItems.newPlaceholder')"
                    :aria-label="t('workItems.newTitle')"
                    data-testid="kanban-new-title"
                    @keydown.esc="adding = null"
                  />
                </form>
                <Button v-else size="sm" variant="ghost" class="justify-start text-muted-foreground" data-testid="kanban-add" @click="(adding = st.id), (newTitle = '')">
                  <Plus aria-hidden="true" />{{ t('kanban.add') }}
                </Button>
              </template>
            </section>
          </div>
        </section>
      </div>
    </div>
    <p class="text-xs text-muted-foreground">{{ t('kanban.hint') }}</p>
    <ItemPanel />
  </div>
</template>

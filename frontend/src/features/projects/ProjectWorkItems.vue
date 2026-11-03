<script setup lang="ts">
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ChevronDown, ChevronRight, Plus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { filteredTree, type Filters, type ViewConfig } from '@/api/projectScreens'
import { createWorkItem, getWorkItem, linkMilestoneWorkItem, listMilestones, projectTree } from '@/api/work'
import { EditableCell, GridTable, QuickAddRow, type CellOption } from '@/components/data-table'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import { GROUPS, filterDefaults } from '@/features/views/useProjectFilters'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { patchWorkItem } from '@/features/workitems/patch'
import AssigneesPicker from '@/features/work/AssigneesPicker.vue'
import LabelsPicker from '@/features/work/LabelsPicker.vue'
import ItemPanel from '@/features/workitems/ItemPanel.vue'
import { allParents, childrenOf, visibleRows } from '@/features/workitems/tree'
import { useAuthStore } from '@/stores/auth'
import type { Project, WorkNode } from '@/types/work'

// Work item tree of a project, edited cell by cell and with quick rows to
// create (RF-139, RF-140). With filters, an order or a grouping it lists the
// matching ones (spec 008, RF-55 to RF-59).
const props = withDefaults(defineProps<{ project: Project; filters?: Filters; sort?: string; group?: ViewConfig['group'] }>(), {
  filters: () => ({}),
  sort: '',
  group: '',
})
const emit = defineEmits<{ arrange: [change: Partial<ViewConfig>] }>()
const { t } = useI18n()
const qc = useQueryClient()
const { catalog, types, statuses, priorities, sizes, labels } = useCatalog()
const { users } = usePeople()

const { data: tree } = useQuery({ queryKey: ['tree', props.project.id], queryFn: () => projectTree(props.project.id) })

const route = useRoute()
const openItem = computed(() => (typeof route.query.item === 'string' ? route.query.item : null))
const panelTo = (key: string) => ({ query: { ...route.query, item: key } })
const children = computed(() => childrenOf(tree.value ?? []))
const expanded = ref(new Set<string>())
const rows = computed(() => visibleRows(children.value, expanded.value))
function toggle(id: string) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

// Filters, an order or a grouping switch to the list of matching ones.
const filtering = computed(() => Object.keys(props.filters).length > 0 || !!props.sort || !!props.group)
const { data: list } = useQuery({
  queryKey: computed(() => ['tree', props.project.id, 'filtered', props.filters, props.sort]),
  queryFn: () => filteredTree(props.project.id, props.filters, props.sort),
  enabled: filtering,
  placeholderData: keepPreviousData,
})

const enabledTypes = computed(() =>
  props.project.work_item_type_ids.map((id) => types.value.get(id)).filter((x) => !!x && !x.retired),
)
const writable = computed(() => props.project.can.write_work_items && props.project.lifecycle === 'activo')

function refresh() {
  qc.invalidateQueries({ queryKey: ['milestones', props.project.id] })
  qc.invalidateQueries({ queryKey: ['overview', props.project.id] })
  qc.invalidateQueries({ queryKey: ['views', props.project.id] })
  return qc.invalidateQueries({ queryKey: ['tree', props.project.id] })
}

// Quick rows: one at the end for roots and one under the row whose child is being added.
const newTypeId = ref('')
const addingChild = ref<string | null>(null)
// A new one takes the filters with a single value (RF-59).
const auth = useAuthStore()
async function create(title: string, parentId: string | null) {
  const { values, milestoneId } = filtering.value ? filterDefaults(props.filters, auth.me?.id) : { values: {}, milestoneId: undefined }
  const created = await createWorkItem(props.project.id, {
    title,
    type_id: newTypeId.value || enabledTypes.value[0]?.id || '',
    parent_id: parentId,
    ...values,
  })
  if (milestoneId && milestoneId !== 'none') await linkMilestoneWorkItem(milestoneId, created.id).catch(() => undefined)
  if (parentId) expanded.value = new Set([...expanded.value, parentId])
  await refresh()
}
function addChild(id: string) {
  addingChild.value = id
  expanded.value = new Set([...expanded.value, id])
}

// Cells save one field; the rest of the work item is read fresh (patchWorkItem).
type Field = 'title' | 'type_id' | 'status_id' | 'priority_id' | 'size_id' | 'due_date' | 'weight'
const cellValue = (field: Field, value: string) => (field === 'weight' ? Number(value) : field === 'title' ? value : value || null)
function saver(n: WorkNode, field: Field) {
  return async (value: string) => {
    await patchWorkItem(n.id, { [field]: cellValue(field, value) })
    qc.invalidateQueries({ queryKey: ['work-item', n.key] })
    await refresh()
  }
}
// The workflow decides the next statuses of each work item (RF-87), so they load on edit.
const statusChoices = (n: WorkNode) => async (): Promise<CellOption[]> => {
  const w = await getWorkItem(n.id)
  return [w.status_id, ...w.allowed_status_ids].map((id) => ({ value: id, label: statuses.value.get(id)?.name ?? id }))
}
const typeOptions = computed<CellOption[]>(() => enabledTypes.value.map((ty) => ({ value: ty!.id, label: ty!.name })))
const priorityOptions = computed<CellOption[]>(() =>
  catalog.value.priorities.filter((p) => !p.retired).map((p) => ({ value: p.id, label: p.name })),
)
const people = computed(() => users.value.map((u) => ({ id: u.id, name: u.name })))
// Retired sizes only where they already are (spec 006, RF-9, RF-56).
const sizeOptions = (n: WorkNode): CellOption[] =>
  catalog.value.sizes.filter((sz) => !sz.retired || sz.id === n.size_id).map((sz) => ({ value: sz.id, label: sz.name }))
const sizeName = (n: WorkNode) => (n.size_id ? (sizes.value.get(n.size_id)?.name ?? '') : '')
function saveLabels(n: WorkNode) {
  return async (ids: string[]) => {
    await patchWorkItem(n.id, { label_ids: ids })
    qc.invalidateQueries({ queryKey: ['work-item', n.key] })
    await refresh()
  }
}
function saveAssignees(n: WorkNode) {
  return async (ids: string[]) => {
    await patchWorkItem(n.id, { assignee_ids: ids, assignee_id: ids[0] ?? null })
    qc.invalidateQueries({ queryKey: ['work-item', n.key] })
    await refresh()
  }
}

// The filtered list and the tree share the row layout; a grouping adds a
// heading row per group with its count (RF-58).
type Row = { node: WorkNode; level: number; hasChildren: boolean; expanded: boolean; heading?: string }
const groupOf = (n: WorkNode): { key: string; name: string }[] => {
  switch (props.group) {
    case 'status':
      return [{ key: n.status_id, name: statuses.value.get(n.status_id)?.name ?? '' }]
    case 'type':
      return [{ key: n.type_id, name: types.value.get(n.type_id)?.name ?? '' }]
    case 'assignee':
      return n.assignees.length ? n.assignees.map((a) => ({ key: a.id, name: a.name })) : [{ key: '', name: t('workItems.unassigned') }]
    case 'milestone':
      return n.milestone_ids?.length
        ? n.milestone_ids.map((id) => ({ key: id, name: milestoneNames.value.get(id) ?? '' }))
        : [{ key: '', name: t('timeline.noMilestone') }]
    default:
      return [{ key: '', name: '' }]
  }
}
const { data: milestones } = useQuery({ queryKey: computed(() => ['milestones', props.project.id]), queryFn: () => listMilestones(props.project.id) })
const milestoneNames = computed(() => new Map((milestones.value ?? []).map((m) => [m.id, m.name])))
const shownRows = computed<Row[]>(() => {
  if (!filtering.value) return rows.value
  const nodes = list.value ?? []
  if (!props.group) return nodes.map((node) => ({ node, level: 0, hasChildren: false, expanded: false }))
  const groups = new Map<string, { name: string; nodes: WorkNode[] }>()
  for (const n of nodes) {
    for (const g of groupOf(n)) {
      if (!groups.has(g.key)) groups.set(g.key, { name: g.name, nodes: [] })
      groups.get(g.key)!.nodes.push(n)
    }
  }
  const out: Row[] = []
  for (const [key, g] of [...groups.entries()].sort((x, y) => (x[0] === '' ? 1 : y[0] === '' ? -1 : x[1].name.localeCompare(y[1].name)))) {
    out.push({ node: g.nodes[0]!, level: 0, hasChildren: false, expanded: false, heading: `${g.name} · ${g.nodes.length}` + '\u0000' + key })
    for (const node of g.nodes) out.push({ node, level: 0, hasChildren: false, expanded: false })
  }
  return out
})
const SORTS = ['', 'key', 'title', 'status', 'priority', 'due', '-due', 'progress', '-progress', 'weight', '-weight']
const typeOf = (n: WorkNode) => types.value.get(n.type_id)
const statusOf = (n: WorkNode) => statuses.value.get(n.status_id)
const priorityName = (n: WorkNode) => priorities.value.get(n.priority_id)?.name ?? ''
const indent = (level: number) => `${level * 1.25 + 0.5}rem`
const canAdd = computed(() => writable.value)
const columns = computed(() => (canAdd.value ? 12 : 11))
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-wrap items-center gap-2">
      <template v-if="!filtering">
        <Button size="sm" variant="outline" @click="expanded = allParents(children)">{{ t('workItems.expandAll') }}</Button>
        <Button size="sm" variant="outline" @click="expanded = new Set()">{{ t('workItems.collapseAll') }}</Button>
      </template>
      <span class="flex-1" />
      <label class="flex items-center gap-1.5 text-sm text-muted-foreground">
        {{ t('table.sortBy') }}
        <NativeSelect :model-value="sort" class="w-40" data-testid="table-sort" @update:model-value="(v: string) => emit('arrange', { sort: v })">
          <option v-for="o in SORTS" :key="o" :value="o">{{ t(`table.sorts.${o || 'tree'}`) }}</option>
        </NativeSelect>
      </label>
      <label class="flex items-center gap-1.5 text-sm text-muted-foreground">
        {{ t('table.groupBy') }}
        <NativeSelect :model-value="group" class="w-36" data-testid="table-group" @update:model-value="(v: string) => emit('arrange', { group: v as ViewConfig['group'] })">
          <option v-for="g in GROUPS" :key="g" :value="g">{{ t(`table.groups.${g || 'none'}`) }}</option>
        </NativeSelect>
      </label>
    </div>
    <GridTable :empty="(filtering || !writable) && shownRows.length === 0">
      <thead>
        <tr>
          <th class="w-24">{{ t('workItems.key') }}</th>
          <th>{{ t('workItems.title') }}</th>
          <th class="w-32">{{ t('workItems.type') }}</th>
          <th class="w-36">{{ t('workItems.status') }}</th>
          <th class="w-28">{{ t('workItems.priority') }}</th>
          <th class="w-44">{{ t('workItems.assignees') }}</th>
          <th class="w-40">{{ t('labels.title') }}</th>
          <th class="w-20">{{ t('workItems.size') }}</th>
          <th class="w-20" :title="t('workItems.weightHint')">{{ t('workItems.weight') }}</th>
          <th class="w-36">{{ t('workItems.progress') }}</th>
          <th class="w-36">{{ t('workItems.due') }}</th>
          <th v-if="canAdd" class="w-10" />
        </tr>
      </thead>
      <tbody>
        <template v-for="(r, i) in shownRows" :key="r.heading ?? r.node.id + '-' + i">
          <tr v-if="r.heading" class="bg-muted/40" data-testid="table-group-row">
            <th :colspan="columns" scope="rowgroup" class="px-3 py-1.5 text-left text-xs font-semibold">{{ r.heading.split('\u0000')[0] }}</th>
          </tr>
          <tr v-else class="hover:bg-muted/30" data-testid="work-item-row">
            <td class="px-2">
              <RouterLink
                :to="panelTo(r.node.key)"
                class="font-mono text-xs text-muted-foreground hover:text-foreground hover:underline"
                :class="{ 'font-semibold text-primary': openItem === r.node.key }"
                data-testid="open-item"
              >
                {{ r.node.key }}
              </RouterLink>
            </td>
            <EditableCell
              :value="r.node.title"
              :label="t('workItems.title')"
              :save="saver(r.node, 'title')"
              :editable="writable"
              :style="{ paddingLeft: indent(r.level) }"
              required
            >
              <button
                v-if="r.hasChildren"
                type="button"
                class="text-muted-foreground"
                :aria-label="r.expanded ? t('workItems.collapse') : t('workItems.expand')"
                :aria-expanded="r.expanded"
                @click.stop="toggle(r.node.id)"
              >
                <ChevronDown v-if="r.expanded" class="size-4" />
                <ChevronRight v-else class="size-4" />
              </button>
              <span v-else-if="!filtering" class="w-4" />
              <span :class="r.hasChildren && 'font-medium'">{{ r.node.title }}</span>
            </EditableCell>
            <EditableCell
              kind="select"
              :value="r.node.type_id"
              :label="t('workItems.type')"
              :options="typeOptions"
              :save="saver(r.node, 'type_id')"
              :editable="writable"
              required
            >
              <span class="size-2 shrink-0 rounded-full" :style="{ background: typeOf(r.node)?.color }" />{{ typeOf(r.node)?.name }}
            </EditableCell>
            <EditableCell
              kind="select"
              :value="r.node.status_id"
              :label="t('workItems.status')"
              :options="statusChoices(r.node)"
              :save="saver(r.node, 'status_id')"
              :editable="writable"
              required
            >
              <span class="inline-flex items-center gap-1.5 rounded-full bg-muted px-2 py-0.5 text-xs font-medium">
                <span class="size-2 rounded-full" :style="{ background: statusOf(r.node)?.color }" />{{ statusOf(r.node)?.name }}
              </span>
            </EditableCell>
            <EditableCell
              kind="select"
              :value="r.node.priority_id"
              :label="t('workItems.priority')"
              :options="priorityOptions"
              :save="saver(r.node, 'priority_id')"
              :editable="writable"
              required
            >
              {{ priorityName(r.node) }}
            </EditableCell>
            <td class="border-r px-2 align-middle">
              <AssigneesPicker
                :value="r.node.assignees"
                :options="people"
                :label="t('workItems.assignees')"
                :editable="writable"
                :save="saveAssignees(r.node)"
              />
            </td>
            <td class="border-r px-2 align-middle">
              <LabelsPicker
                :value="r.node.label_ids"
                :options="labels"
                :label="t('labels.title')"
                :editable="writable"
                :visible="2"
                :save="saveLabels(r.node)"
              />
            </td>
            <EditableCell
              kind="select"
              :value="r.node.size_id ?? ''"
              :label="t('workItems.size')"
              :options="sizeOptions(r.node)"
              :save="saver(r.node, 'size_id')"
              :editable="writable"
            >
              <span :class="!r.node.size_id && 'text-muted-foreground'">{{ sizeName(r.node) || t('common.none') }}</span>
            </EditableCell>
            <EditableCell
              kind="number"
              :value="String(r.node.weight)"
              :label="t('workItems.weight')"
              :save="saver(r.node, 'weight')"
              :editable="writable"
              :class="r.node.weight === 0 ? 'text-muted-foreground' : undefined"
              required
            />
            <td class="bg-muted/20 px-2"><ProgressBar :value="r.node.derived_done_ratio" /></td>
            <EditableCell kind="date" :value="r.node.due_date ?? ''" :label="t('workItems.due')" :save="saver(r.node, 'due_date')" :editable="writable" />
            <td v-if="canAdd" class="px-1 text-center">
              <button
                type="button"
                class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
                :title="t('workItems.addChild')"
                :aria-label="t('workItems.addChild')"
                @click="addChild(r.node.id)"
              >
                <Plus class="size-4" />
              </button>
            </td>
          </tr>
          <QuickAddRow
            v-if="canAdd && addingChild === r.node.id"
            open
            :colspan="columns"
            :indent="indent(r.level + 1)"
            :label="t('workItems.newChild', { key: r.node.key })"
            :placeholder="t('workItems.newChildPlaceholder', { key: r.node.key })"
            :submit="(title) => create(title, r.node.id)"
            @close="addingChild = null"
          >
            <NativeSelect v-model="newTypeId" class="w-36" :aria-label="t('workItems.type')">
              <option v-for="ty in enabledTypes" :key="ty!.id" :value="ty!.id">{{ ty!.name }}</option>
            </NativeSelect>
          </QuickAddRow>
        </template>
        <QuickAddRow
          v-if="canAdd"
          :colspan="columns"
          :label="filtering ? t('table.newInView') : t('workItems.newTitle')"
          :placeholder="t('workItems.newPlaceholder')"
          :submit="(title) => create(title, null)"
        >
          <NativeSelect v-model="newTypeId" class="w-36" :aria-label="t('workItems.type')">
            <option v-for="ty in enabledTypes" :key="ty!.id" :value="ty!.id">{{ ty!.name }}</option>
          </NativeSelect>
        </QuickAddRow>
      </tbody>
    </GridTable>

    <ItemPanel />
  </div>
</template>

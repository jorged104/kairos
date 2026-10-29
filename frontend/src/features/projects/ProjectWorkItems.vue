<script setup lang="ts">
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/vue-query'
import { refDebounced } from '@vueuse/core'
import { ChevronDown, ChevronRight, Plus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { createWorkItem, getWorkItem, listMilestones, listWorkItems, projectTree } from '@/api/work'
import { EditableCell, GridTable, QuickAddRow, type CellOption } from '@/components/data-table'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { patchWorkItem } from '@/features/workitems/patch'
import WorkItemSheet from '@/features/workitems/WorkItemSheet.vue'
import { allParents, childrenOf, visibleRows } from '@/features/workitems/tree'
import type { Project, WorkNode } from '@/types/work'

// Work item tree of a project with a filtered list mode (RF-129, RF-130),
// edited cell by cell and with quick rows to create (RF-139, RF-140).
const props = defineProps<{ project: Project }>()
const { t } = useI18n()
const qc = useQueryClient()
const { catalog, types, statuses, priorities } = useCatalog()
const { users } = usePeople()

const { data: tree } = useQuery({ queryKey: ['tree', props.project.id], queryFn: () => projectTree(props.project.id) })

// The work item open in the side panel, kept in the URL (?item=KAI-12) so it
// survives a reload and the back button closes it (spec 005, RF-37).
const route = useRoute()
const router = useRouter()
const openItem = computed(() => (typeof route.query.item === 'string' ? route.query.item : null))
const panelTo = (key: string) => ({ query: { ...route.query, item: key } })
function closePanel() {
  const { item: _item, ...query } = route.query
  void router.replace({ query })
}
const children = computed(() => childrenOf(tree.value ?? []))
const expanded = ref(new Set<string>())
const rows = computed(() => visibleRows(children.value, expanded.value))
function toggle(id: string) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

// Filters switch to the paginated list of the API.
const search = ref('')
const q = refDebounced(search, 300)
const typeId = ref('')
const closed = ref('')
const milestoneId = ref('')
const filtering = computed(() => !!(q.value || typeId.value || closed.value || milestoneId.value))
const { data: milestones } = useQuery({ queryKey: ['milestones', props.project.id], queryFn: () => listMilestones(props.project.id) })
const { data: list } = useQuery({
  queryKey: computed(() => ['work-items', props.project.id, q.value, typeId.value, closed.value, milestoneId.value]),
  queryFn: () =>
    listWorkItems(props.project.id, { q: q.value, type_id: typeId.value, closed: closed.value, milestone_id: milestoneId.value, page_size: 200 }),
  enabled: filtering,
  placeholderData: keepPreviousData,
})

const enabledTypes = computed(() =>
  props.project.work_item_type_ids.map((id) => types.value.get(id)).filter((x) => !!x && !x.retired),
)
const writable = computed(() => props.project.can.write_work_items && props.project.lifecycle === 'activo')

function refresh() {
  qc.invalidateQueries({ queryKey: ['work-items', props.project.id] })
  qc.invalidateQueries({ queryKey: ['milestones', props.project.id] })
  return qc.invalidateQueries({ queryKey: ['tree', props.project.id] })
}

// Quick rows: one at the end for roots and one under the row whose child is being added.
const newTypeId = ref('')
const addingChild = ref<string | null>(null)
async function create(title: string, parentId: string | null) {
  await createWorkItem(props.project.id, {
    title,
    type_id: newTypeId.value || enabledTypes.value[0]?.id || '',
    parent_id: parentId,
  })
  if (parentId) expanded.value = new Set([...expanded.value, parentId])
  await refresh()
}
function addChild(id: string) {
  addingChild.value = id
  expanded.value = new Set([...expanded.value, id])
}

// Cells save one field; the rest of the work item is read fresh (patchWorkItem).
type Field = 'title' | 'type_id' | 'status_id' | 'priority_id' | 'assignee_id' | 'due_date' | 'weight'
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
const userOptions = computed<CellOption[]>(() => users.value.map((u) => ({ value: u.id, label: u.name })))

// The filtered list and the tree share the row layout.
const shownRows = computed(() =>
  filtering.value ? (list.value?.items ?? []).map((node) => ({ node, level: 0, hasChildren: false, expanded: false })) : rows.value,
)
const typeOf = (n: WorkNode) => types.value.get(n.type_id)
const statusOf = (n: WorkNode) => statuses.value.get(n.status_id)
const priorityName = (n: WorkNode) => priorities.value.get(n.priority_id)?.name ?? ''
const indent = (level: number) => `${level * 1.25 + 0.5}rem`
const columns = computed(() => (writable.value && !filtering.value ? 10 : 9))
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="grid gap-2 rounded-lg border bg-background p-3 sm:grid-cols-4">
      <Input v-model="search" :placeholder="t('workItems.searchPlaceholder')" :aria-label="t('workItems.filterText')" />
      <NativeSelect v-model="typeId" :aria-label="t('workItems.filterType')">
        <option value="">{{ t('workItems.allTypes') }}</option>
        <option v-for="ty in enabledTypes" :key="ty!.id" :value="ty!.id">{{ ty!.name }}</option>
      </NativeSelect>
      <NativeSelect v-model="closed" :aria-label="t('workItems.state')">
        <option value="">{{ t('workItems.openAndClosed') }}</option>
        <option value="false">{{ t('workItems.onlyOpen') }}</option>
        <option value="true">{{ t('workItems.onlyClosed') }}</option>
      </NativeSelect>
      <NativeSelect v-model="milestoneId" :aria-label="t('workItems.milestone')">
        <option value="">{{ t('workItems.anyMilestone') }}</option>
        <option v-for="m in milestones ?? []" :key="m.id" :value="m.id">{{ m.name }}</option>
      </NativeSelect>
    </div>

    <div v-if="!filtering" class="flex gap-2">
      <Button size="sm" variant="outline" @click="expanded = allParents(children)">{{ t('workItems.expandAll') }}</Button>
      <Button size="sm" variant="outline" @click="expanded = new Set()">{{ t('workItems.collapseAll') }}</Button>
    </div>

    <GridTable :empty="(filtering || !writable) && shownRows.length === 0">
      <thead>
        <tr>
          <th class="w-24">{{ t('workItems.key') }}</th>
          <th>{{ t('workItems.title') }}</th>
          <th class="w-32">{{ t('workItems.type') }}</th>
          <th class="w-36">{{ t('workItems.status') }}</th>
          <th class="w-28">{{ t('workItems.priority') }}</th>
          <th class="w-44">{{ t('workItems.assignee') }}</th>
          <th class="w-20" :title="t('workItems.weightHint')">{{ t('workItems.weight') }}</th>
          <th class="w-36">{{ t('workItems.progress') }}</th>
          <th class="w-36">{{ t('workItems.due') }}</th>
          <th v-if="columns === 10" class="w-10" />
        </tr>
      </thead>
      <tbody>
        <template v-for="r in shownRows" :key="r.node.id">
          <tr class="hover:bg-muted/30" data-testid="work-item-row">
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
            <EditableCell
              kind="select"
              :value="r.node.assignee?.id ?? ''"
              :label="t('workItems.assignee')"
              :options="userOptions"
              :save="saver(r.node, 'assignee_id')"
              :editable="writable"
            >
              <span :class="!r.node.assignee && 'text-muted-foreground'">{{ r.node.assignee?.name ?? t('workItems.unassigned') }}</span>
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
            <td v-if="columns === 10" class="px-1 text-center">
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
            v-if="columns === 10 && addingChild === r.node.id"
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
          v-if="columns === 10"
          :colspan="columns"
          :label="t('workItems.newTitle')"
          :placeholder="t('workItems.newPlaceholder')"
          :submit="(title) => create(title, null)"
        >
          <NativeSelect v-model="newTypeId" class="w-36" :aria-label="t('workItems.type')">
            <option v-for="ty in enabledTypes" :key="ty!.id" :value="ty!.id">{{ ty!.name }}</option>
          </NativeSelect>
        </QuickAddRow>
      </tbody>
    </GridTable>

    <aside
      v-if="openItem"
      class="fixed inset-y-0 right-0 z-30 w-full overflow-y-auto border-l bg-background p-4 shadow-2xl sm:w-[min(44rem,90vw)] sm:p-6"
      :aria-label="openItem"
      data-testid="item-panel"
    >
      <WorkItemSheet :key="openItem" :wikey="openItem" panel @close="closePanel" />
    </aside>
  </div>
</template>

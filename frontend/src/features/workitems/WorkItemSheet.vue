<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ChevronDown, ChevronRight, Expand, Maximize2, Minimize2, Trash2, X } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import type { SavedPage } from '@/api/pages'
import { deleteWorkItem, getProject, getWorkItemByKey, projectTree, updateWorkItem, workItemAncestors, type WorkItemUpdateInput } from '@/api/work'
import type { CellOption } from '@/components/data-table/types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { onClickOutside } from '@vueuse/core'
import { createComment } from '@/api/comments'
import CommentThread from '@/features/comments/CommentThread.vue'
import CommentsSection from '@/features/comments/CommentsSection.vue'
import { useComments } from '@/features/comments/useComments'
import PageSection, { type PageCommenting } from '@/features/pages/PageSection.vue'
import { useFullWidth } from '@/features/pages/useFullWidth'
import AssigneesPicker from '@/features/work/AssigneesPicker.vue'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog, useCustomFields } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { toUpdateInput } from '@/features/workitems/patch'
import { childrenOf } from '@/features/workitems/tree'
import type { AttributeValue } from '@/types/api'
import type { CustomField, WorkItem } from '@/types/work'
import InlineField from './InlineField.vue'

// A work item as a page (spec 005): title and properties, edited where they
// are shown, the main ones first (RF-68), and below them its page (RF-60 to RF-64). In the side panel of the tree
// it offers to open it as a page and closes with Escape (RF-37, RF-38,
// RF-42); as a page, the full width (RF-40).
const props = defineProps<{ wikey: string; panel?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const router = useRouter()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const { catalog, types, statuses } = useCatalog()
const fields = useCustomFields()
const { users } = usePeople()

const key = computed(() => ['work-item', props.wikey.toUpperCase()])
const { data: wi, error: loadError } = useQuery({ queryKey: key, queryFn: () => getWorkItemByKey(props.wikey) })
const id = computed(() => wi.value?.id ?? '')
const projectId = computed(() => wi.value?.project.id ?? '')
const enabled = computed(() => !!wi.value)
const { data: project } = useQuery({ queryKey: computed(() => ['project', projectId.value]), queryFn: () => getProject(projectId.value), enabled })
const { data: tree } = useQuery({ queryKey: computed(() => ['tree', projectId.value]), queryFn: () => projectTree(projectId.value), enabled })
const { data: ancestors } = useQuery({ queryKey: computed(() => ['ancestors', id.value]), queryFn: () => workItemAncestors(id.value), enabled })

const writable = computed(() => !!wi.value?.can.write && project.value?.lifecycle === 'activo')
const itemFields = computed(() =>
  fields.value.filter(
    (f) =>
      f.entity === 'work_item' &&
      project.value?.custom_field_ids.includes(f.id) &&
      (f.work_item_type_ids.length === 0 || f.work_item_type_ids.includes(wi.value?.type_id ?? '')),
  ),
)

// The choices of each property.
const typeOptions = computed<CellOption[]>(() =>
  (project.value?.work_item_type_ids ?? []).flatMap((x) => {
    const ty = types.value.get(x)
    return ty ? [{ value: ty.id, label: ty.name }] : []
  }),
)
const statusOptions = computed<CellOption[]>(() =>
  (wi.value ? [wi.value.status_id, ...wi.value.allowed_status_ids] : []).map((s) => ({ value: s, label: statuses.value.get(s)?.name ?? s })),
)
const priorityOptions = computed<CellOption[]>(() =>
  catalog.value.priorities.filter((p) => !p.retired || p.id === wi.value?.priority_id).map((p) => ({ value: p.id, label: p.name })),
)
const people = computed(() => users.value.map((u) => ({ id: u.id, name: u.name })))
// A retired size only stays where it is (spec 006, RF-9).
const sizeOptions = computed<CellOption[]>(() =>
  catalog.value.sizes.filter((sz) => !sz.retired || sz.id === wi.value?.size_id).map((sz) => ({ value: sz.id, label: sz.name })),
)
// A work item can't hang from itself or a descendant (RF-77).
const parentOptions = computed<CellOption[]>(() => {
  if (!wi.value || !tree.value) return wi.value?.parent ? [{ value: wi.value.parent.id, label: wi.value.parent.key }] : []
  const children = childrenOf(tree.value)
  const excluded = new Set<string>([wi.value.id])
  const stack = [wi.value.id]
  while (stack.length) for (const c of children.get(stack.pop()!) ?? []) excluded.add(c.id) && stack.push(c.id)
  return tree.value.filter((n) => !excluded.has(n.id)).map((n) => ({ value: n.id, label: '  '.repeat(n.depth) + n.key + ' ' + n.title }))
})

const error = ref<unknown>(null)
const conflict = computed(() => error.value instanceof ApiError && error.value.code === 'version_conflict')
function refresh(updated?: WorkItem) {
  error.value = null
  if (updated) qc.setQueryData(key.value, updated)
  for (const k of [['tree', projectId.value], ['milestones', projectId.value], ['ancestors', id.value], ['wi-milestones', id.value]]) {
    qc.invalidateQueries({ queryKey: k })
  }
  return qc.invalidateQueries({ queryKey: key.value })
}
const save = useMutation({
  // The page's autosave also moves the version: the cached one is the latest.
  mutationFn: (input: WorkItemUpdateInput) => updateWorkItem(id.value, { ...input, version: wi.value?.version ?? input.version }),
  onSuccess: (w) => refresh(w),
  onError: (e) => (error.value = e),
})
/** Saves one property; the others go as they are now. */
function saveField(patch: Partial<WorkItemUpdateInput>) {
  if (!wi.value) return Promise.resolve()
  return save.mutateAsync({ ...toUpdateInput(wi.value), ...patch })
}
const orNull = (v: string) => v || null
const hours = (v: string) => (v === '' ? null : Number(v))
const changeStatus = (statusId: string) => saveField({ status_id: statusId }).catch(() => undefined)

// The main properties are always shown, size and weight among them (spec 006,
// RF-10); the rest fold away, as in Notion (RF-68).
const moreProperties = ref(false)

// Custom fields as text in the fields, typed for the API.
const fieldKind = (f: CustomField) =>
  f.type === 'number' ? 'number' : f.type === 'date' ? 'date' : f.type === 'select' || f.type === 'boolean' ? 'select' : 'text'
function fieldOptions(f: CustomField): CellOption[] {
  if (f.type === 'boolean') return [{ value: 'true', label: t('common.yes') }, { value: 'false', label: t('common.no') }]
  // A retired option stays visible for whoever already has it (RF-107).
  const current = wi.value?.custom_values[f.id]
  return f.options
    .filter((o) => !o.retired || o.value === current)
    .map((o) => ({ value: o.value, label: o.retired ? t('attributes.retiredOption', { value: o.value }) : o.value }))
}
const fieldText = (v: AttributeValue | undefined) => (v === null || v === undefined ? '' : String(v))
function saveCustom(f: CustomField, v: string) {
  const typed: AttributeValue = v === '' ? null : f.type === 'number' ? Number(v) : f.type === 'boolean' ? v === 'true' : v
  return saveField({ custom_values: { ...wi.value!.custom_values, [f.id]: typed } })
}

// Deleting takes a second click on the bin, within a few seconds.
const armed = ref(false)
let disarm: ReturnType<typeof setTimeout> | undefined
const remove = useMutation({
  mutationFn: () => deleteWorkItem(id.value),
  onSuccess: () => router.push(`/projects/${projectId.value}?tab=work-items`),
  onError: (e) => (error.value = e),
})
function onDelete() {
  clearTimeout(disarm)
  if (!armed.value) {
    armed.value = true
    disarm = setTimeout(() => (armed.value = false), 3000)
    return
  }
  armed.value = false
  remove.mutate()
}

// The page: an autosave updates the cached work item, versions included.
const pageSection = ref<InstanceType<typeof PageSection> | null>(null)
function onPageSaved(saved: SavedPage) {
  qc.setQueryData<WorkItem>(key.value, (old) =>
    old && { ...old, description: saved.page, page_version: saved.page_version, version: saved.version, refs: saved.refs },
  )
}
async function reloadPage() {
  const fresh = await getWorkItemByKey(props.wikey)
  qc.setQueryData(key.value, fresh)
  return fresh
}

const fullWidth = useFullWidth()

// Comments: the list below the properties (spec 006, RF-22) and the threads
// of a highlight in the page, floating next to it (RF-47).
const { list: comments, apply: applyComments, openAnchors } = useComments(() => id.value)
const popover = ref<{ ids: string[]; rect: DOMRect } | null>(null)
const popoverEl = ref<HTMLElement | null>(null)
const popoverThreads = computed(() => (comments.value?.threads ?? []).filter((th) => popover.value?.ids.includes(th.id)))
onClickOutside(popoverEl, () => (popover.value = null))
const commenting = computed<PageCommenting>(() => ({
  workItemId: id.value,
  canComment: !!comments.value?.can_comment,
  openThreads: openAnchors.value,
  anchor: async (input, body) => {
    const r = applyComments(await createComment(id.value, body, { anchor: input }))
    return r.page!
  },
  show: (ids, rect) => {
    if (ids.some((x) => openAnchors.value.includes(x))) popover.value = { ids, rect }
  },
}))
const popoverAt = (r: DOMRect) => ({ top: `${r.bottom + 8}px`, left: `${Math.max(8, Math.min(r.left, window.innerWidth - 424))}px` })

/** Closes the panel once the page has saved what is pending (RF-32). */
async function close() {
  await pageSection.value?.flush()
  emit('close')
}
// Escape closes the panel unless a menu, a search or a field took it first (RF-42).
function onKey(e: KeyboardEvent) {
  if (e.key !== 'Escape' || e.defaultPrevented) return
  if (popover.value) {
    popover.value = null
    return
  }
  if (!document.querySelector('[data-testid=suggestions], [data-testid=block-menu], [data-testid=comment-anchor-popover]')) void close()
}
onMounted(() => props.panel && window.addEventListener('keydown', onKey))
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  clearTimeout(disarm)
})
defineExpose({ close })
</script>

<template>
  <div class="flex flex-col gap-4">
    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>
    <template v-if="wi">
      <div class="flex flex-wrap items-center gap-2">
        <Button
          v-if="wi.can.delete && writable"
          size="sm"
          :variant="armed ? 'destructive' : 'ghost'"
          :disabled="wi.child_count > 0 || remove.isPending.value"
          :title="wi.child_count > 0 ? t('errors.has_children') : armed ? t('workItems.confirmDelete') : t('workItems.delete')"
          :aria-label="armed ? t('workItems.confirmDelete') : t('workItems.delete')"
          data-testid="delete-item"
          @click="onDelete"
        >
          <Trash2 aria-hidden="true" />
          <span v-if="armed">{{ t('workItems.confirmDelete') }}</span>
        </Button>
        <nav class="flex min-w-0 flex-wrap items-center gap-1 text-sm text-muted-foreground">
          <RouterLink :to="`/projects/${wi.project.id}?tab=work-items`" class="hover:underline">{{ wi.project.name }}</RouterLink>
          <template v-for="a in [...(ancestors ?? [])].reverse()" :key="a.id">
            <span>/</span>
            <RouterLink :to="`/work-items/${a.key}`" class="hover:underline">{{ a.key }}</RouterLink>
          </template>
          <span>/</span>
          <span class="font-mono">{{ wi.key }}</span>
        </nav>
        <div class="ml-auto flex items-center gap-1">
          <template v-if="panel">
            <Button as-child size="sm" variant="outline">
              <RouterLink :to="`/work-items/${wi.key}`" data-testid="open-as-page"><Expand aria-hidden="true" />{{ t('page.openAsPage') }}</RouterLink>
            </Button>
            <Button size="sm" variant="ghost" :aria-label="t('page.closePanel')" data-testid="close-panel" @click="close">
              <X aria-hidden="true" />
            </Button>
          </template>
          <Button
            v-else
            size="sm"
            :variant="fullWidth.on.value ? 'default' : 'outline'"
            :aria-pressed="fullWidth.on.value"
            data-testid="full-width"
            @click="fullWidth.toggle()"
          >
            <component :is="fullWidth.on.value ? Minimize2 : Maximize2" aria-hidden="true" />
            {{ fullWidth.on.value ? t('page.exitFullWidth') : t('page.fullWidth') }}
          </Button>
        </div>
      </div>
      <h1 class="font-heading text-2xl leading-tight font-bold sm:text-3xl">
        <InlineField
          id="w-title"
          :value="wi.title"
          :label="t('workItems.title')"
          :editable="writable"
          required
          class="h-auto py-0.5 font-heading text-2xl font-bold sm:text-3xl"
          :save="(v) => saveField({ title: v })"
        />
      </h1>

      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription class="flex flex-wrap items-center gap-2">
          {{ errorMessage(error) }}
          <span v-if="error instanceof ApiError && error.details.blocked_by">({{ error.details.blocked_by }})</span>
          <Button v-if="conflict" size="sm" variant="outline" @click="refresh()">{{ t('work.reload') }}</Button>
        </AlertDescription>
      </Alert>

      <div v-if="writable && wi.allowed_status_ids.length" class="flex flex-wrap items-center gap-2 text-sm">
        <span class="text-muted-foreground">{{ t('workItems.moveTo') }}</span>
        <Button v-for="s in wi.allowed_status_ids" :key="s" size="sm" variant="outline" data-testid="status-button" @click="changeStatus(s)">
          {{ statuses.get(s)?.name }}
        </Button>
      </div>

      <section class="flex flex-col gap-3 rounded-lg border bg-background p-4" data-testid="properties">
        <dl class="grid gap-x-6 gap-y-1 text-sm" :class="panel ? 'sm:grid-cols-2' : 'sm:grid-cols-3 xl:grid-cols-4'">
          <div class="flex flex-col">
            <dt><label for="w-status" class="text-muted-foreground">{{ t('workItems.status') }}</label></dt>
            <dd>
              <InlineField id="w-status" kind="select" :value="wi.status_id" :label="t('workItems.status')" :options="statusOptions" :editable="writable" required :save="(v) => saveField({ status_id: v })" />
            </dd>
          </div>
          <div class="flex flex-col">
            <dt><label for="w-assignees" class="text-muted-foreground">{{ t('workItems.assignees') }}</label></dt>
            <dd>
              <AssigneesPicker
                id="w-assignees"
                :value="wi.assignees"
                :options="people"
                :label="t('workItems.assignees')"
                :editable="writable"
                :save="(ids) => saveField({ assignee_ids: ids, assignee_id: ids[0] ?? null })"
              />
            </dd>
          </div>
          <div class="flex flex-col">
            <dt><label for="w-priority" class="text-muted-foreground">{{ t('workItems.priority') }}</label></dt>
            <dd>
              <InlineField id="w-priority" kind="select" :value="wi.priority_id" :label="t('workItems.priority')" :options="priorityOptions" :editable="writable" required :save="(v) => saveField({ priority_id: v })" />
            </dd>
          </div>
          <div class="flex flex-col">
            <dt><label for="w-start" class="text-muted-foreground">{{ t('workItems.start') }}</label></dt>
            <dd>
              <InlineField id="w-start" kind="date" :value="wi.start_date ?? ''" :label="t('workItems.start')" :editable="writable" :save="(v) => saveField({ start_date: orNull(v) })" />
            </dd>
          </div>
          <div class="flex flex-col">
            <dt><label for="w-due" class="text-muted-foreground">{{ t('workItems.due') }}</label></dt>
            <dd>
              <InlineField id="w-due" kind="date" :value="wi.due_date ?? ''" :label="t('workItems.due')" :editable="writable" :save="(v) => saveField({ due_date: orNull(v) })" />
            </dd>
          </div>
          <div class="flex flex-col">
            <dt><label for="w-size" class="text-muted-foreground">{{ t('workItems.size') }}</label></dt>
            <dd>
              <InlineField
                id="w-size"
                kind="select"
                :value="wi.size_id ?? ''"
                :label="t('workItems.size')"
                :options="sizeOptions"
                :editable="writable"
                :save="(v) => saveField({ size_id: orNull(v) })"
              />
            </dd>
          </div>
          <div class="flex flex-col">
            <dt>
              <label for="w-weight" class="text-muted-foreground" :title="t('workItems.weightHint')">
                {{ t('workItems.weight') }}<span v-if="wi.weight === 0"> · {{ t('workItems.tracking') }}</span>
              </label>
            </dt>
            <dd>
              <InlineField
                id="w-weight"
                kind="number"
                :min="0"
                :max="100"
                :value="String(wi.weight)"
                :label="t('workItems.weight')"
                :editable="writable"
                required
                :save="(v) => saveField({ weight: Number(v) })"
              />
            </dd>
          </div>
          <template v-if="moreProperties">
            <div class="flex flex-col">
              <dt><label for="w-type" class="text-muted-foreground">{{ t('workItems.type') }}</label></dt>
              <dd>
                <InlineField id="w-type" kind="select" :value="wi.type_id" :label="t('workItems.type')" :options="typeOptions" :editable="writable" required :save="(v) => saveField({ type_id: v })" />
              </dd>
            </div>
            <div class="flex flex-col">
              <dt><label for="w-parent" class="text-muted-foreground">{{ t('workItems.parent') }}</label></dt>
              <dd>
                <InlineField
                  id="w-parent"
                  kind="select"
                  :value="wi.parent?.id ?? ''"
                  :label="t('workItems.parent')"
                  :options="parentOptions"
                  :empty-label="t('workItems.noParent')"
                  :editable="writable"
                  :save="(v) => saveField({ parent_id: orNull(v) })"
                />
              </dd>
            </div>
            <div class="flex flex-col">
              <dt class="text-muted-foreground">{{ t('workItems.author') }}</dt>
              <dd class="flex h-8 items-center">{{ wi.author.name }}</dd>
            </div>
            <div class="flex flex-col">
              <dt><label for="w-est" class="text-muted-foreground">{{ t('workItems.estimated') }}</label></dt>
              <dd>
                <InlineField
                  id="w-est"
                  kind="number"
                  :min="0"
                  :max="10000"
                  :step="0.25"
                  :value="fieldText(wi.estimated_hours)"
                  :label="t('workItems.estimated')"
                  :editable="writable"
                  :save="(v) => saveField({ estimated_hours: hours(v) })"
                />
              </dd>
            </div>
            <div class="flex flex-col">
              <dt><label for="w-rem" class="text-muted-foreground">{{ t('workItems.remaining') }}</label></dt>
              <dd>
                <InlineField
                  id="w-rem"
                  kind="number"
                  :min="0"
                  :max="10000"
                  :step="0.25"
                  :value="fieldText(wi.remaining_hours)"
                  :label="t('workItems.remaining')"
                  :editable="writable"
                  :save="(v) => saveField({ remaining_hours: hours(v) })"
                />
              </dd>
            </div>
            <div class="flex flex-col">
              <dt><label for="w-ratio" class="text-muted-foreground">{{ t('workItems.doneRatio') }}</label></dt>
              <dd>
                <InlineField
                  id="w-ratio"
                  kind="number"
                  :min="0"
                  :max="100"
                  :value="String(wi.done_ratio)"
                  :label="t('workItems.doneRatio')"
                  :editable="writable"
                  required
                  :save="(v) => saveField({ done_ratio: Number(v) })"
                />
              </dd>
            </div>
            <div v-for="f in itemFields" :key="f.id" class="flex flex-col">
              <dt><label :for="`wf-${f.id}`" class="text-muted-foreground">{{ f.name }}{{ f.required ? ' *' : '' }}</label></dt>
              <dd>
                <InlineField
                  :id="`wf-${f.id}`"
                  :kind="fieldKind(f)"
                  :value="fieldText(wi.custom_values[f.id])"
                  :label="f.name"
                  :options="fieldOptions(f)"
                  :editable="writable && !f.retired_at"
                  :save="(v) => saveCustom(f, v)"
                />
              </dd>
            </div>
          </template>
        </dl>
        <button
          type="button"
          class="-mt-1 flex w-fit items-center gap-1 rounded-md px-1.5 py-0.5 text-sm text-muted-foreground hover:bg-muted/60 hover:text-foreground"
          :aria-expanded="moreProperties"
          data-testid="more-properties"
          @click="moreProperties = !moreProperties"
        >
          <component :is="moreProperties ? ChevronDown : ChevronRight" class="size-4" aria-hidden="true" />
          {{ moreProperties ? t('workItems.lessProperties') : t('workItems.moreProperties', { count: 6 + itemFields.length }) }}
        </button>
        <div class="grid gap-2 sm:grid-cols-2">
          <div>
            <p class="text-xs text-muted-foreground">{{ t('workItems.ownProgress') }}</p>
            <ProgressBar :value="wi.done_ratio" />
          </div>
          <div v-if="wi.child_count">
            <p class="text-xs text-muted-foreground">
              {{ t('workItems.derivedProgress', { estimated: wi.derived.estimated_hours, remaining: wi.derived.remaining_hours }) }}
            </p>
            <ProgressBar :value="wi.derived.done_ratio" />
          </div>
        </div>
      </section>

      <CommentsSection :work-item-id="wi.id" />

      <PageSection
        :key="wi.id"
        ref="pageSection"
        :owner="{ kind: 'work-items', id: wi.id }"
        :doc="wi.description"
        :page-version="wi.page_version"
        :refs="wi.refs"
        :editable="writable"
        :reload="reloadPage"
        :commenting="commenting"
        @saved="onPageSaved"
      />
      <Teleport to="body">
        <div
          v-if="popover && popoverThreads.length && comments"
          ref="popoverEl"
          class="fixed z-50 flex max-h-[60vh] w-[26rem] max-w-[calc(100vw-1rem)] flex-col gap-2 overflow-y-auto rounded-lg border bg-popover p-2 shadow-xl"
          :style="popoverAt(popover.rect)"
          role="dialog"
          :aria-label="t('comments.title')"
          data-testid="comment-popover"
        >
          <CommentThread
            v-for="th in popoverThreads"
            :key="th.id"
            :thread="th"
            :refs="comments.refs"
            :work-item-id="wi.id"
            :can-comment="comments.can_comment"
            :apply="applyComments"
          />
        </div>
      </Teleport>
    </template>
  </div>
</template>

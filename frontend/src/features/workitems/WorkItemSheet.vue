<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Expand, Maximize2, Minimize2, X } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import type { SavedPage } from '@/api/pages'
import {
  addDependency,
  deleteWorkItem,
  getProject,
  getWorkItemByKey,
  projectTree,
  removeDependency,
  updateWorkItem,
  workItemAncestors,
  workItemMilestones,
  type WorkItemUpdateInput,
} from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import PageSection from '@/features/pages/PageSection.vue'
import { useFullWidth } from '@/features/pages/useFullWidth'
import AttributeField from '@/features/users/AttributeField.vue'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog, useCustomFields } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { toUpdateInput } from '@/features/workitems/patch'
import { childrenOf } from '@/features/workitems/tree'
import { DEPENDENCY_TYPES, type DependencyType, type WorkItem } from '@/types/work'

// A work item as a page (spec 005): title, properties and, below, its page,
// then milestones and dependencies (RF-61 to RF-96, RF-39). In the side
// panel of the tree it offers to open it as a page and closes with Escape
// (RF-37, RF-38, RF-42); as a page, the full width (RF-40).
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
const { data: milestones } = useQuery({ queryKey: computed(() => ['wi-milestones', id.value]), queryFn: () => workItemMilestones(id.value), enabled })

const writable = computed(() => !!wi.value?.can.write && project.value?.lifecycle === 'activo')
const statusOptions = computed(() => (wi.value ? [wi.value.status_id, ...wi.value.allowed_status_ids] : []))
const enabledTypes = computed(() => (project.value?.work_item_type_ids ?? []).map((x) => types.value.get(x)).filter((x) => !!x))
const itemFields = computed(() =>
  fields.value.filter(
    (f) =>
      f.entity === 'work_item' &&
      project.value?.custom_field_ids.includes(f.id) &&
      (f.work_item_type_ids.length === 0 || f.work_item_type_ids.includes(form.value.type_id)),
  ),
)
// A work item can't hang from itself or a descendant (RF-77).
const parentOptions = computed(() => {
  if (!wi.value || !tree.value) return []
  const children = childrenOf(tree.value)
  const excluded = new Set<string>([wi.value.id])
  const stack = [wi.value.id]
  while (stack.length) for (const c of children.get(stack.pop()!) ?? []) excluded.add(c.id) && stack.push(c.id)
  return tree.value.filter((n) => !excluded.has(n.id))
})

const toForm = toUpdateInput
const editing = ref(false)
const form = ref<WorkItemUpdateInput>({} as WorkItemUpdateInput)
watch(wi, (w) => {
  if (w && !editing.value) form.value = toForm(w)
}, { immediate: true })

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
const nullable = (v: string | null) => v || null
const save = useMutation({
  // The page's autosave also moves the version: the cached one is the latest.
  mutationFn: (input: WorkItemUpdateInput) =>
    updateWorkItem(id.value, {
      ...input,
      version: wi.value?.version ?? input.version,
      start_date: nullable(input.start_date), due_date: nullable(input.due_date),
      estimated_hours: input.estimated_hours === null || String(input.estimated_hours) === '' ? null : Number(input.estimated_hours),
      remaining_hours: input.remaining_hours === null || String(input.remaining_hours) === '' ? null : Number(input.remaining_hours),
      done_ratio: Number(input.done_ratio),
      weight: Number(input.weight),
    }),
  onSuccess: (w) => {
    editing.value = false
    return refresh(w)
  },
  onError: (e) => (error.value = e),
})
const changeStatus = (statusId: string) => wi.value && save.mutate({ ...toForm(wi.value), status_id: statusId })
const remove = useMutation({
  mutationFn: () => deleteWorkItem(id.value),
  onSuccess: () => router.push(`/projects/${projectId.value}?tab=work-items`),
  onError: (e) => (error.value = e),
})

const dep = ref({ type: 'blocks' as DependencyType, key: '', lag: 0 })
const addDep = useMutation({
  mutationFn: async () => addDependency(id.value, (await getWorkItemByKey(dep.value.key)).id, dep.value.type, dep.value.type === 'precedes' ? Number(dep.value.lag) : 0),
  onSuccess: () => {
    dep.value = { type: dep.value.type, key: '', lag: 0 }
    return refresh()
  },
  onError: (e) => (error.value = e),
})
const removeDep = useMutation({
  mutationFn: (depId: string) => removeDependency(depId),
  onSuccess: () => refresh(),
  onError: (e) => (error.value = e),
})
const priorities = computed(() => catalog.value.priorities.filter((p) => !p.retired || p.id === wi.value?.priority_id))

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

/** Closes the panel once the page has saved what is pending (RF-32). */
async function close() {
  await pageSection.value?.flush()
  emit('close')
}
// Escape closes the panel unless a menu or a search took it first (RF-42).
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && !e.defaultPrevented && !document.querySelector('[data-testid=suggestions], [data-testid=block-menu]')) void close()
}
onMounted(() => props.panel && window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
defineExpose({ close })
</script>

<template>
  <div class="flex flex-col gap-4">
    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>
    <template v-if="wi">
      <nav class="flex flex-wrap items-center gap-1 text-sm text-muted-foreground">
        <RouterLink :to="`/projects/${wi.project.id}?tab=work-items`" class="hover:underline">{{ wi.project.name }}</RouterLink>
        <template v-for="a in [...(ancestors ?? [])].reverse()" :key="a.id">
          <span>/</span>
          <RouterLink :to="`/work-items/${a.key}`" class="hover:underline">{{ a.key }}</RouterLink>
        </template>
      </nav>
      <div class="flex flex-wrap items-center gap-2">
        <span class="font-mono text-sm text-muted-foreground">{{ wi.key }}</span>
        <Badge variant="outline">{{ types.get(wi.type_id)?.name }}</Badge>
        <Badge :variant="statuses.get(wi.status_id)?.is_closed ? 'secondary' : 'default'">{{ statuses.get(wi.status_id)?.name }}</Badge>
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
      <h1 class="font-heading text-2xl font-bold leading-tight sm:text-3xl">{{ wi.title }}</h1>

      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription class="flex flex-wrap items-center gap-2">
          {{ errorMessage(error) }}
          <span v-if="error instanceof ApiError && error.details.blocked_by">({{ error.details.blocked_by }})</span>
          <Button v-if="conflict" size="sm" variant="outline" @click="editing = false; refresh()">{{ t('work.reload') }}</Button>
        </AlertDescription>
      </Alert>

      <div v-if="writable && !editing && wi.allowed_status_ids.length" class="flex flex-wrap items-center gap-2 text-sm">
        <span class="text-muted-foreground">{{ t('workItems.moveTo') }}</span>
        <Button v-for="s in wi.allowed_status_ids" :key="s" size="sm" variant="outline" data-testid="status-button" @click="changeStatus(s)">
          {{ statuses.get(s)?.name }}
        </Button>
      </div>

      <section class="rounded-lg border bg-background p-4">
        <form v-if="editing" class="grid gap-3 sm:grid-cols-3" @submit.prevent="save.mutate(form)">
          <div class="flex flex-col gap-1.5 sm:col-span-3">
            <Label for="w-title">{{ t('workItems.title') }}</Label>
            <Input id="w-title" v-model="form.title" maxlength="255" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-type">{{ t('workItems.type') }}</Label>
            <NativeSelect id="w-type" v-model="form.type_id">
              <option v-for="ty in enabledTypes" :key="ty!.id" :value="ty!.id">{{ ty!.name }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-status">{{ t('workItems.status') }}</Label>
            <NativeSelect id="w-status" v-model="form.status_id">
              <option v-for="s in statusOptions" :key="s" :value="s">{{ statuses.get(s)?.name }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-priority">{{ t('workItems.priority') }}</Label>
            <NativeSelect id="w-priority" v-model="form.priority_id">
              <option v-for="p in priorities" :key="p.id" :value="p.id">{{ p.name }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-assignee">{{ t('workItems.assignee') }}</Label>
            <NativeSelect id="w-assignee" :model-value="form.assignee_id ?? ''" @update:model-value="form.assignee_id = $event || null">
              <option value="">{{ t('workItems.unassigned') }}</option>
              <option v-if="wi.assignee && !users.some((u) => u.id === wi!.assignee!.id)" :value="wi.assignee.id">{{ wi.assignee.name }}</option>
              <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5 sm:col-span-2">
            <Label for="w-parent">{{ t('workItems.parent') }}</Label>
            <NativeSelect id="w-parent" :model-value="form.parent_id ?? ''" @update:model-value="form.parent_id = $event || null">
              <option value="">{{ t('workItems.noParent') }}</option>
              <option v-for="n in parentOptions" :key="n.id" :value="n.id">{{ '  '.repeat(n.depth) + n.key + ' ' + n.title }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-start">{{ t('workItems.start') }}</Label>
            <Input id="w-start" :model-value="form.start_date ?? ''" type="date" @update:model-value="form.start_date = String($event) || null" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-due">{{ t('workItems.due') }}</Label>
            <Input id="w-due" :model-value="form.due_date ?? ''" type="date" @update:model-value="form.due_date = String($event) || null" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-ratio">{{ t('workItems.doneRatio') }}</Label>
            <Input id="w-ratio" v-model="form.done_ratio" type="number" min="0" max="100" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-weight">{{ t('workItems.weight') }}</Label>
            <Input id="w-weight" v-model="form.weight" type="number" min="0" max="100" required aria-describedby="w-weight-hint" />
            <p id="w-weight-hint" class="text-xs text-muted-foreground">{{ t('workItems.weightHint') }}</p>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-est">{{ t('workItems.estimated') }}</Label>
            <Input id="w-est" :model-value="form.estimated_hours ?? ''" type="number" min="0" max="10000" step="0.25" @update:model-value="form.estimated_hours = $event === '' ? null : Number($event)" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="w-rem">{{ t('workItems.remaining') }}</Label>
            <Input id="w-rem" :model-value="form.remaining_hours ?? ''" type="number" min="0" max="10000" step="0.25" @update:model-value="form.remaining_hours = $event === '' ? null : Number($event)" />
          </div>
          <div v-for="f in itemFields" :key="f.id" class="flex flex-col gap-1.5">
            <Label :for="`wf-${f.id}`">{{ f.name }}{{ f.required ? ' *' : '' }}</Label>
            <AttributeField :id="`wf-${f.id}`" v-model="form.custom_values![f.id]" :attribute="f" :disabled="!!f.retired_at" />
          </div>
          <div class="flex gap-2">
            <Button type="submit" :disabled="save.isPending.value">{{ t('common.save') }}</Button>
            <Button type="button" variant="ghost" @click="editing = false">{{ t('common.cancel') }}</Button>
          </div>
        </form>
        <div v-else class="flex flex-col gap-3">
          <dl class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
            <div><dt class="text-muted-foreground">{{ t('workItems.assignee') }}</dt><dd>{{ wi.assignee?.name ?? t('workItems.unassigned') }}</dd></div>
            <div><dt class="text-muted-foreground">{{ t('workItems.priority') }}</dt><dd>{{ catalog.priorities.find((p) => p.id === wi!.priority_id)?.name }}</dd></div>
            <div><dt class="text-muted-foreground">{{ t('workItems.author') }}</dt><dd>{{ wi.author.name }}</dd></div>
            <div><dt class="text-muted-foreground">{{ t('workItems.parent') }}</dt>
              <dd><RouterLink v-if="wi.parent" :to="`/work-items/${wi.parent.key}`" class="hover:underline">{{ wi.parent.key }}</RouterLink><span v-else>{{ t('common.none') }}</span></dd>
            </div>
            <div><dt class="text-muted-foreground">{{ t('workItems.start') }}</dt><dd>{{ wi.start_date ?? t('common.none') }}</dd></div>
            <div><dt class="text-muted-foreground">{{ t('workItems.due') }}</dt><dd>{{ wi.due_date ?? t('common.none') }}</dd></div>
            <div><dt class="text-muted-foreground">{{ t('workItems.estimated') }}</dt><dd>{{ wi.estimated_hours ?? t('common.none') }}</dd></div>
            <div><dt class="text-muted-foreground">{{ t('workItems.remaining') }}</dt><dd>{{ wi.remaining_hours ?? t('common.none') }}</dd></div>
            <div>
              <dt class="text-muted-foreground">{{ t('workItems.weight') }}</dt>
              <dd>{{ wi.weight }}<span v-if="wi.weight === 0" class="text-muted-foreground"> · {{ t('workItems.tracking') }}</span></dd>
            </div>
            <div v-for="f in itemFields" :key="f.id"><dt class="text-muted-foreground">{{ f.name }}</dt><dd>{{ wi.custom_values[f.id] ?? t('common.none') }}</dd></div>
          </dl>
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
          <div class="flex gap-2">
            <Button v-if="writable" size="sm" @click="form = toForm(wi); editing = true">{{ t('common.edit') }}</Button>
            <Button v-if="wi.can.delete && writable" size="sm" variant="ghost" :disabled="wi.child_count > 0" :title="wi.child_count > 0 ? t('errors.has_children') : ''" @click="remove.mutate()">
              {{ t('workItems.delete') }}
            </Button>
          </div>
        </div>
      </section>

      <PageSection
        :key="wi.id"
        ref="pageSection"
        :owner="{ kind: 'work-items', id: wi.id }"
        :doc="wi.description"
        :page-version="wi.page_version"
        :refs="wi.refs"
        :editable="writable"
        :reload="reloadPage"
        @saved="onPageSaved"
      />

      <div class="grid gap-4" :class="{ 'lg:grid-cols-2': !panel }">
        <section class="rounded-lg border bg-background p-4">
          <h2 class="mb-2 font-medium">{{ t('workItems.milestones') }}</h2>
          <ul class="flex flex-col gap-1 text-sm">
            <li v-for="m in milestones ?? []" :key="m.id" class="flex items-center gap-2" data-testid="wi-milestone">
              <RouterLink :to="`/milestones/${m.id}`" class="hover:underline">{{ m.name }}</RouterLink>
              <Badge v-if="m.inherited" variant="outline">{{ t('milestones.inherited') }}</Badge>
              <span class="text-xs text-muted-foreground">{{ m.due_date }}</span>
            </li>
            <li v-if="milestones && milestones.length === 0" class="text-muted-foreground">{{ t('common.empty') }}</li>
          </ul>
        </section>

        <section class="rounded-lg border bg-background p-4">
          <h2 class="mb-2 font-medium">{{ t('workItems.dependencies') }}</h2>
          <ul class="flex flex-col gap-1 text-sm">
            <li v-for="d in wi.dependencies" :key="d.id" class="flex flex-wrap items-center gap-2" data-testid="dependency">
              <span class="text-muted-foreground">{{ t(`workItems.depTypes.${d.type}.${d.direction}`) }}</span>
              <RouterLink :to="`/work-items/${d.other.key}`" class="hover:underline">{{ d.other.key }} {{ d.other.title }}</RouterLink>
              <span v-if="d.lag_days" class="text-xs text-muted-foreground">{{ t('workItems.lag', { days: d.lag_days }) }}</span>
              <Badge v-if="d.dates_violated" variant="destructive">{{ t('workItems.datesViolated') }}</Badge>
              <Button v-if="writable && d.direction === 'outgoing'" size="sm" variant="ghost" @click="removeDep.mutate(d.id)">{{ t('common.remove') }}</Button>
            </li>
            <li v-if="wi.dependencies.length === 0" class="text-muted-foreground">{{ t('common.empty') }}</li>
          </ul>
          <form v-if="writable" class="mt-2 flex flex-wrap items-center gap-2 text-sm" @submit.prevent="addDep.mutate()">
            <NativeSelect v-model="dep.type" class="w-40" :aria-label="t('workItems.depType')">
              <option v-for="dt in DEPENDENCY_TYPES" :key="dt" :value="dt">{{ t(`workItems.depTypes.${dt}.outgoing`) }}</option>
            </NativeSelect>
            <Input v-model="dep.key" class="w-32" :placeholder="t('milestones.linkPlaceholder')" :aria-label="t('workItems.depTarget')" required />
            <Input v-if="dep.type === 'precedes'" v-model="dep.lag" type="number" min="-365" max="365" class="w-20" :aria-label="t('workItems.lagDays')" />
            <Button type="submit" size="sm" variant="outline">{{ t('common.add') }}</Button>
          </form>
        </section>
      </div>
    </template>
  </div>
</template>

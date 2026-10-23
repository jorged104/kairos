<script setup lang="ts">
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { refDebounced } from '@vueuse/core'
import { ChevronDown, ChevronRight, Plus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { createWorkItem, listMilestones, listWorkItems, projectTree } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { allParents, childrenOf, visibleRows } from '@/features/workitems/tree'
import type { Project, WorkNode } from '@/types/work'

// Work item tree of a project with a filtered list mode (RF-129, RF-130).
const props = defineProps<{ project: Project }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()
const { types, statuses } = useCatalog()

const { data: tree } = useQuery({ queryKey: ['tree', props.project.id], queryFn: () => projectTree(props.project.id) })
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
  props.project.work_item_type_ids.map((id) => types.value.get(id)).filter((x) => x && !x.retired),
)
const writable = computed(() => props.project.can.write_work_items && props.project.lifecycle === 'activo')
const form = ref({ title: '', typeId: '', parentId: '' })
const parentKey = computed(() => tree.value?.find((n) => n.id === form.value.parentId)?.key)
const create = useMutation({
  mutationFn: () =>
    createWorkItem(props.project.id, {
      title: form.value.title,
      type_id: form.value.typeId || enabledTypes.value[0]?.id || '',
      parent_id: form.value.parentId || null,
    }),
  onSuccess: (wi) => {
    if (form.value.parentId) expanded.value = new Set([...expanded.value, form.value.parentId])
    form.value = { title: '', typeId: form.value.typeId, parentId: '' }
    qc.invalidateQueries({ queryKey: ['work-items', props.project.id] })
    return qc.invalidateQueries({ queryKey: ['tree', wi.project.id] })
  },
  onError: (e) => emit('error', e),
})

const typeOf = (n: WorkNode) => types.value.get(n.type_id)
const statusOf = (n: WorkNode) => statuses.value.get(n.status_id)
</script>

<template>
  <div class="flex flex-col gap-3">
    <form v-if="writable" class="flex flex-wrap items-center gap-2 rounded-lg border bg-background p-3" @submit.prevent="create.mutate()">
      <Input v-model="form.title" class="w-72" :placeholder="t('workItems.newTitle')" :aria-label="t('workItems.title')" required />
      <NativeSelect v-model="form.typeId" class="w-40" :aria-label="t('workItems.type')">
        <option v-for="ty in enabledTypes" :key="ty!.id" :value="ty!.id">{{ ty!.name }}</option>
      </NativeSelect>
      <span v-if="parentKey" class="text-sm text-muted-foreground">
        {{ t('workItems.childOf', { key: parentKey }) }}
        <button type="button" class="underline" @click="form.parentId = ''">{{ t('common.remove') }}</button>
      </span>
      <Button type="submit" size="sm" :disabled="create.isPending.value">{{ t('common.create') }}</Button>
    </form>

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

    <div class="overflow-x-auto rounded-lg border bg-background">
      <table class="w-full text-sm">
        <thead class="border-b text-left text-xs text-muted-foreground">
          <tr>
            <th class="p-2">{{ t('workItems.title') }}</th>
            <th class="p-2">{{ t('workItems.status') }}</th>
            <th class="p-2">{{ t('workItems.assignee') }}</th>
            <th class="w-40 p-2">{{ t('workItems.progress') }}</th>
            <th class="p-2">{{ t('workItems.due') }}</th>
            <th v-if="writable" class="p-2" />
          </tr>
        </thead>
        <tbody>
          <template v-if="filtering">
            <tr v-for="n in list?.items ?? []" :key="n.id" class="border-b last:border-0" data-testid="work-item-row">
              <td class="p-2">
                <RouterLink :to="`/work-items/${n.key}`" class="hover:underline">
                  <span class="font-mono text-xs text-muted-foreground">{{ n.key }}</span> {{ n.title }}
                </RouterLink>
              </td>
              <td class="p-2">{{ statusOf(n)?.name }}</td>
              <td class="p-2">{{ n.assignee?.name ?? t('common.none') }}</td>
              <td class="p-2"><ProgressBar :value="n.derived_done_ratio" /></td>
              <td class="p-2 whitespace-nowrap">{{ n.due_date ?? '' }}</td>
              <td v-if="writable" />
            </tr>
          </template>
          <template v-else>
            <tr v-for="r in rows" :key="r.node.id" class="border-b last:border-0" data-testid="work-item-row">
              <td class="p-2">
                <div class="flex items-center gap-1" :style="{ paddingLeft: `${r.level * 1.25}rem` }">
                  <button
                    v-if="r.hasChildren"
                    type="button"
                    class="text-muted-foreground"
                    :aria-label="r.expanded ? t('workItems.collapse') : t('workItems.expand')"
                    :aria-expanded="r.expanded"
                    @click="toggle(r.node.id)"
                  >
                    <ChevronDown v-if="r.expanded" class="size-4" />
                    <ChevronRight v-else class="size-4" />
                  </button>
                  <span v-else class="w-4" />
                  <span class="size-2 shrink-0 rounded-full" :style="{ background: typeOf(r.node)?.color }" :title="typeOf(r.node)?.name" />
                  <RouterLink :to="`/work-items/${r.node.key}`" class="hover:underline">
                    <span class="font-mono text-xs text-muted-foreground">{{ r.node.key }}</span> {{ r.node.title }}
                  </RouterLink>
                </div>
              </td>
              <td class="p-2">
                <span class="inline-flex items-center gap-1">
                  <span class="size-2 rounded-full" :style="{ background: statusOf(r.node)?.color }" />{{ statusOf(r.node)?.name }}
                </span>
              </td>
              <td class="p-2">{{ r.node.assignee?.name ?? t('common.none') }}</td>
              <td class="p-2"><ProgressBar :value="r.node.derived_done_ratio" /></td>
              <td class="p-2 whitespace-nowrap">{{ r.node.due_date ?? '' }}</td>
              <td v-if="writable" class="p-2">
                <button
                  type="button"
                  class="text-muted-foreground hover:text-foreground"
                  :title="t('workItems.addChild')"
                  :aria-label="t('workItems.addChild')"
                  @click="form.parentId = r.node.id"
                >
                  <Plus class="size-4" />
                </button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
      <p v-if="(filtering ? list?.items.length : rows.length) === 0" class="p-4 text-sm text-muted-foreground">{{ t('common.empty') }}</p>
    </div>
  </div>
</template>

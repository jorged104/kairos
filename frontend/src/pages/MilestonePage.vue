<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import {
  changeMilestoneStatus,
  deleteMilestone,
  getMilestone,
  getWorkItemByKey,
  linkMilestoneWorkItem,
  listPhases,
  milestoneWorkItems,
  unlinkMilestoneWorkItem,
  updateMilestone,
  type MilestoneAction,
} from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { SavedPage } from '@/api/pages'
import PageSection from '@/features/pages/PageSection.vue'
import { milestoneVariant } from '@/features/work/Badges'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { useCatalog } from '@/features/work/useCatalog'
import type { Milestone } from '@/types/work'

// Milestone page: progress, effective work items and links (RF-44 to RF-60).
const props = defineProps<{ id: string }>()
const { t } = useI18n()
const router = useRouter()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const { statuses } = useCatalog()

const key = computed(() => ['milestone', props.id])
const { data: milestone, error: loadError } = useQuery({ queryKey: key, queryFn: () => getMilestone(props.id) })
const { data: items } = useQuery({ queryKey: computed(() => ['milestone-items', props.id]), queryFn: () => milestoneWorkItems(props.id) })
const projectId = computed(() => milestone.value?.project.id ?? '')
const { data: phases } = useQuery({
  queryKey: computed(() => ['phases', projectId.value]),
  queryFn: () => listPhases(projectId.value),
  enabled: computed(() => !!projectId.value),
})

const error = ref<unknown>(null)
const conflict = computed(() => error.value instanceof ApiError && error.value.code === 'version_conflict')
function refresh(m?: Milestone) {
  error.value = null
  if (m) qc.setQueryData(key.value, m)
  qc.invalidateQueries({ queryKey: ['milestones', projectId.value] })
  qc.invalidateQueries({ queryKey: ['milestone-items', props.id] })
  return qc.invalidateQueries({ queryKey: key.value })
}

const editing = ref(false)
const form = ref({ name: '', due: '', phase: '' })
function startEdit(m: Milestone) {
  form.value = { name: m.name, due: m.due_date ?? '', phase: m.phase?.id ?? '' }
  editing.value = true
}
// The milestone's page (spec 005, RF-43): an autosave updates the cache.
function onPageSaved(saved: SavedPage) {
  qc.setQueryData<Milestone>(key.value, (old) =>
    old && { ...old, description: saved.page, page_version: saved.page_version, version: saved.version, refs: saved.refs },
  )
}
async function reloadPage() {
  const fresh = await getMilestone(props.id)
  qc.setQueryData(key.value, fresh)
  return fresh
}

const save = useMutation({
  mutationFn: () =>
    updateMilestone(props.id, {
      version: milestone.value!.version,
      name: form.value.name,
      due_date: form.value.due || null,
      phase_id: form.value.phase || null,
      position: milestone.value!.position,
    }),
  onSuccess: (m) => {
    editing.value = false
    return refresh(m)
  },
  onError: (e) => (error.value = e),
})
const completedOn = ref('')
const status = useMutation({
  mutationFn: (action: MilestoneAction) => changeMilestoneStatus(props.id, action, milestone.value!.version, completedOn.value),
  onSuccess: refresh,
  onError: (e) => (error.value = e),
})
const remove = useMutation({
  mutationFn: () => deleteMilestone(props.id),
  onSuccess: () => router.push(`/projects/${projectId.value}?tab=milestones`),
  onError: (e) => (error.value = e),
})

const linkKey = ref('')
const link = useMutation({
  mutationFn: async () => linkMilestoneWorkItem(props.id, (await getWorkItemByKey(linkKey.value)).id),
  onSuccess: () => {
    linkKey.value = ''
    return refresh()
  },
  onError: (e) => (error.value = e),
})
const unlink = useMutation({
  mutationFn: (workItemId: string) => unlinkMilestoneWorkItem(props.id, workItemId),
  onSuccess: () => refresh(),
  onError: (e) => (error.value = e),
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>
    <template v-if="milestone">
      <div class="flex flex-wrap items-center gap-2">
        <RouterLink :to="`/projects/${milestone.project.id}?tab=milestones`" class="font-mono text-sm text-muted-foreground hover:underline">
          {{ milestone.project.identifier }}
        </RouterLink>
        <h1 class="font-heading text-xl font-semibold">{{ milestone.name }}</h1>
        <Badge :variant="milestoneVariant[milestone.status]">{{ t(`milestones.statuses.${milestone.status}`) }}</Badge>
        <Badge v-if="milestone.overdue" variant="destructive">{{ t('milestones.overdue') }}</Badge>
      </div>

      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription class="flex flex-wrap items-center gap-2">
          {{ errorMessage(error) }}
          <Button v-if="conflict" size="sm" variant="outline" @click="refresh()">{{ t('work.reload') }}</Button>
        </AlertDescription>
      </Alert>

      <section class="grid gap-4 rounded-lg border bg-background p-4 sm:grid-cols-3">
        <div class="flex flex-col gap-1 sm:col-span-2">
          <ProgressBar :value="milestone.progress.percent" :label="t('milestones.progress')" />
          <p class="text-sm text-muted-foreground">
            {{ t('milestones.counts', { closed: milestone.progress.closed, total: milestone.progress.total }) }} ·
            {{ t('milestones.hours', { estimated: milestone.progress.estimated_hours, remaining: milestone.progress.remaining_hours }) }}
          </p>
        </div>
        <dl class="text-sm">
          <dt class="text-muted-foreground">{{ t('milestones.due') }}</dt>
          <dd>{{ milestone.due_date ?? t('common.none') }}</dd>
          <template v-if="milestone.completed_on">
            <dt class="text-muted-foreground">{{ t('milestones.completedOn') }}</dt>
            <dd>{{ milestone.completed_on }}</dd>
          </template>
          <template v-if="milestone.phase">
            <dt class="text-muted-foreground">{{ t('milestones.phase') }}</dt>
            <dd>{{ milestone.phase.name }}</dd>
          </template>
        </dl>
      </section>

      <section class="rounded-lg border bg-background p-4">
        <form v-if="editing" class="grid gap-3 sm:grid-cols-3" @submit.prevent="save.mutate()">
          <div class="flex flex-col gap-1.5">
            <Label for="m-name">{{ t('milestones.name') }}</Label>
            <Input id="m-name" v-model="form.name" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="m-due">{{ t('milestones.due') }}</Label>
            <Input id="m-due" v-model="form.due" type="date" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="m-phase">{{ t('milestones.phase') }}</Label>
            <NativeSelect id="m-phase" v-model="form.phase">
              <option value="">{{ t('milestones.noPhase') }}</option>
              <option v-for="ph in phases ?? []" :key="ph.id" :value="ph.id">{{ ph.name }}</option>
            </NativeSelect>
          </div>
          <div class="flex gap-2">
            <Button type="submit" size="sm">{{ t('common.save') }}</Button>
            <Button type="button" size="sm" variant="ghost" @click="editing = false">{{ t('common.cancel') }}</Button>
          </div>
        </form>
        <template v-else>
          <div v-if="milestone.can.manage" class="mt-3 flex flex-wrap items-center gap-2">
            <Button size="sm" @click="startEdit(milestone)">{{ t('common.edit') }}</Button>
            <template v-if="milestone.status === 'abierto'">
              <Input v-model="completedOn" type="date" class="w-40" :aria-label="t('milestones.completedOn')" />
              <Button size="sm" variant="outline" @click="status.mutate('complete')">{{ t('milestones.actions.complete') }}</Button>
              <Button size="sm" variant="outline" @click="status.mutate('cancel')">{{ t('milestones.actions.cancel') }}</Button>
            </template>
            <Button v-else size="sm" variant="outline" @click="status.mutate('reopen')">{{ t('milestones.actions.reopen') }}</Button>
            <Button size="sm" variant="ghost" @click="remove.mutate()">{{ t('milestones.actions.delete') }}</Button>
          </div>
        </template>
      </section>

      <section class="rounded-lg border bg-background p-4">
        <PageSection
          :key="milestone.id"
          :owner="{ kind: 'milestones', id: milestone.id }"
          :doc="milestone.description"
          :page-version="milestone.page_version"
          :refs="milestone.refs"
          :editable="milestone.can.manage"
          :reload="reloadPage"
          @saved="onPageSaved"
        />
      </section>

      <section class="flex flex-col gap-2">
        <h2 class="font-medium">{{ t('milestones.workItems') }}</h2>
        <form v-if="milestone.can.manage" class="flex items-center gap-2" @submit.prevent="link.mutate()">
          <Input v-model="linkKey" class="w-48" :placeholder="t('milestones.linkPlaceholder')" :aria-label="t('milestones.link')" required />
          <Button type="submit" size="sm" variant="outline">{{ t('milestones.link') }}</Button>
        </form>
        <ul class="divide-y rounded-lg border bg-background text-sm">
          <li v-for="it in items ?? []" :key="it.id" class="flex flex-wrap items-center gap-2 p-2" data-testid="milestone-item">
            <RouterLink :to="`/work-items/${it.key}`" class="hover:underline">
              <span class="font-mono text-xs text-muted-foreground">{{ it.key }}</span> {{ it.title }}
            </RouterLink>
            <Badge v-if="it.inherited" variant="outline">{{ t('milestones.inherited') }}</Badge>
            <span class="text-xs text-muted-foreground">{{ statuses.get(it.status_id)?.name }}</span>
            <Button v-if="milestone.can.manage && !it.inherited" size="sm" variant="ghost" class="ml-auto" @click="unlink.mutate(it.id)">
              {{ t('common.remove') }}
            </Button>
          </li>
          <li v-if="items && items.length === 0" class="p-3 text-muted-foreground">{{ t('common.empty') }}</li>
        </ul>
      </section>
    </template>
  </div>
</template>

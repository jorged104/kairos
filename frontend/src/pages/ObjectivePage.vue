<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { ApiError } from '@/api/client'
import {
  changeObjectiveStatus,
  createKeyResult,
  deleteKeyResult,
  getObjective,
  getWorkItemByKey,
  linkKeyResult,
  linkObjective,
  listMilestones,
  listProjects,
  unlinkKeyResult,
  unlinkObjective,
  updateKeyResult,
  type KeyResultInput,
  type ObjectiveAction,
} from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import RichTextView from '@/features/richtext/RichTextView.vue'
import { milestoneVariant, objectiveVariant } from '@/features/work/Badges'
import ProgressBar from '@/features/work/ProgressBar.vue'
import type { KeyResult, Objective } from '@/types/work'

// Objective page: key results, contributing milestones, projects and links (RF-97 to RF-115).
const props = defineProps<{ id: string }>()
const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const key = computed(() => ['objective', props.id])
const { data: objective, error: loadError } = useQuery({ queryKey: key, queryFn: () => getObjective(props.id) })
const manage = computed(() => !!objective.value?.can.manage)
const { data: projects } = useQuery({ queryKey: ['projects', 'for-objectives'], queryFn: () => listProjects({ page_size: 100 }), enabled: manage })

const error = ref<unknown>(null)
const conflict = computed(() => error.value instanceof ApiError && error.value.code === 'version_conflict')
function done(o: Objective) {
  error.value = null
  qc.setQueryData(key.value, o)
  return qc.invalidateQueries({ queryKey: ['objectives'] })
}
// after runs once the change is saved, e.g. to clear its form.
function mutation<A>(fn: (arg: A) => Promise<Objective>, after?: (arg: A) => void) {
  return useMutation({
    mutationFn: fn,
    onSuccess: (o: Objective, arg: A) => {
      after?.(arg)
      return done(o)
    },
    onError: (e) => (error.value = e),
  })
}

const status = mutation((action: ObjectiveAction) => changeObjectiveStatus(props.id, action, objective.value!.version))
const actions = computed((): ObjectiveAction[] => {
  switch (objective.value?.status) {
    case 'borrador':
      return ['activate', 'cancel']
    case 'activo':
      return ['close', 'cancel']
    default:
      return ['reopen']
  }
})

const blankKr = (): KeyResultInput => ({ title: '', unit: '', start_value: 0, target_value: 100, calc_mode: 'derivado' })
const kr = ref<KeyResultInput>(blankKr())
const addKr = mutation(
  () => createKeyResult(props.id, { ...kr.value, start_value: Number(kr.value.start_value), target_value: Number(kr.value.target_value) }),
  () => (kr.value = blankKr()),
)
const removeKr = mutation((id: string) => deleteKeyResult(props.id, id))
const current = ref<Record<string, number>>({})
const updateCurrent = mutation((k: KeyResult) =>
  updateKeyResult(props.id, k.id, {
    version: k.version, title: k.title, unit: k.unit, start_value: k.start_value, target_value: k.target_value,
    calc_mode: k.calc_mode, position: k.position, current_value: Number(current.value[k.id] ?? k.current_value),
  }),
)

// Links: projects and milestones to the objective, work items and milestones to a key result.
const linkProjectId = ref('')
const linkProject = mutation(
  () => linkObjective(props.id, 'projects', linkProjectId.value),
  () => (linkProjectId.value = ''),
)
const unlink = mutation((x: { kind: 'projects' | 'milestones'; id: string }) => unlinkObjective(props.id, x.kind, x.id))
const msProject = ref('')
const { data: projectMilestones } = useQuery({
  queryKey: computed(() => ['milestones', msProject.value]),
  queryFn: () => listMilestones(msProject.value),
  enabled: computed(() => !!msProject.value),
})
const linkMilestoneId = ref('')
const linkMilestone = mutation(
  () => linkObjective(props.id, 'milestones', linkMilestoneId.value),
  () => (linkMilestoneId.value = ''),
)
const krLink = ref<Record<string, { key: string; milestone: string }>>({})
const krFor = (id: string) => (krLink.value[id] ??= { key: '', milestone: '' })
const linkKrWorkItem = mutation(
  async (k: KeyResult) => linkKeyResult(props.id, k.id, 'work-items', (await getWorkItemByKey(krFor(k.id).key)).id),
  (k) => (krFor(k.id).key = ''),
)
const linkKrMilestone = mutation(
  (k: KeyResult) => linkKeyResult(props.id, k.id, 'milestones', krFor(k.id).milestone),
  (k) => (krFor(k.id).milestone = ''),
)
const unlinkKr = mutation((x: { kr: string; kind: 'work_item' | 'milestone'; id: string }) =>
  unlinkKeyResult(props.id, x.kr, x.kind === 'work_item' ? 'work-items' : 'milestones', x.id),
)
</script>

<template>
  <div class="flex flex-col gap-4">
    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>
    <template v-if="objective">
      <div class="flex flex-wrap items-center gap-2">
        <h1 class="font-heading text-xl font-semibold">{{ objective.title }}</h1>
        <Badge :variant="objectiveVariant[objective.status]">{{ t(`objectives.statuses.${objective.status}`) }}</Badge>
        <span class="text-sm text-muted-foreground">
          {{ t(`objectives.scopes.${objective.scope}`) }}{{ objective.scope_team ? ` · ${objective.scope_team.name}` : '' }}{{
            objective.scope_project ? ` · ${objective.scope_project.name}` : ''
          }}
          · {{ objective.period_start }} – {{ objective.period_end }} · {{ objective.owner.name }}
        </span>
      </div>
      <ProgressBar :value="objective.progress" :label="t('objectives.progress')" />

      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription class="flex flex-wrap items-center gap-2">
          {{ errorMessage(error) }}
          <Button v-if="conflict" size="sm" variant="outline" @click="qc.invalidateQueries({ queryKey: key }); error = null">{{ t('work.reload') }}</Button>
        </AlertDescription>
      </Alert>

      <RichTextView :doc="objective.description" />
      <div v-if="objective.can.status" class="flex gap-2">
        <Button v-for="a in actions" :key="a" size="sm" variant="outline" @click="status.mutate(a)">{{ t(`objectives.actions.${a}`) }}</Button>
      </div>

      <section class="flex flex-col gap-2">
        <h2 class="font-medium">{{ t('objectives.keyResults') }}</h2>
        <div v-for="k in objective.key_results" :key="k.id" class="flex flex-col gap-2 rounded-lg border bg-background p-3" data-testid="key-result">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium">{{ k.title }}</span>
            <Badge variant="outline">{{ t(`objectives.modes.${k.calc_mode}`) }}</Badge>
            <span class="text-sm text-muted-foreground">
              {{ k.start_value }} → {{ k.target_value }} {{ k.unit }} · {{ t('objectives.current') }}: {{ Math.round(k.current_value * 100) / 100 }}
            </span>
            <span v-if="k.calc_mode === 'derivado'" class="text-xs text-muted-foreground">
              {{ t('objectives.linkedDone', { done: k.linked_done, total: k.linked_total }) }}
            </span>
            <Button v-if="manage" size="sm" variant="ghost" class="ml-auto" @click="removeKr.mutate(k.id)">{{ t('common.remove') }}</Button>
          </div>
          <ProgressBar :value="k.progress" />
          <form v-if="manage && k.calc_mode === 'manual'" class="flex items-center gap-2" @submit.prevent="updateCurrent.mutate(k)">
            <Input v-model="current[k.id]" type="number" step="any" class="w-32" :placeholder="String(k.current_value)" :aria-label="t('objectives.current')" />
            <Button type="submit" size="sm" variant="outline">{{ t('objectives.updateCurrent') }}</Button>
          </form>
          <ul v-if="k.links.length" class="flex flex-wrap gap-2 text-xs">
            <li v-for="l in k.links" :key="l.id" class="flex items-center gap-1 rounded border px-2 py-0.5">
              <RouterLink :to="l.kind === 'work_item' ? `/work-items/${l.label}` : `/milestones/${l.id}`" class="hover:underline">
                {{ l.kind === 'work_item' ? l.label : `◆ ${l.label}` }} {{ l.name }}
              </RouterLink>
              <button v-if="manage" type="button" :aria-label="t('common.remove')" @click="unlinkKr.mutate({ kr: k.id, kind: l.kind, id: l.id })">×</button>
            </li>
          </ul>
          <div v-if="manage && k.calc_mode === 'derivado'" class="flex flex-wrap items-center gap-2 text-sm">
            <form class="flex items-center gap-2" @submit.prevent="linkKrWorkItem.mutate(k)">
              <Input v-model="krFor(k.id).key" class="w-32" :placeholder="t('milestones.linkPlaceholder')" :aria-label="t('objectives.linkWorkItem')" required />
              <Button type="submit" size="sm" variant="outline">{{ t('objectives.linkWorkItem') }}</Button>
            </form>
            <form v-if="objective.milestones.length" class="flex items-center gap-2" @submit.prevent="linkKrMilestone.mutate(k)">
              <NativeSelect v-model="krFor(k.id).milestone" class="w-48" :aria-label="t('objectives.linkMilestone')" required>
                <option value="" disabled>{{ t('objectives.chooseMilestone') }}</option>
                <option v-for="m in objective.milestones" :key="m.id" :value="m.id">{{ m.project.identifier }} · {{ m.name }}</option>
              </NativeSelect>
              <Button type="submit" size="sm" variant="outline">{{ t('objectives.linkMilestone') }}</Button>
            </form>
          </div>
        </div>
        <form v-if="manage" class="flex flex-wrap items-center gap-2 rounded-lg border bg-background p-3 text-sm" @submit.prevent="addKr.mutate()">
          <Input v-model="kr.title" class="w-56" :placeholder="t('objectives.krTitle')" :aria-label="t('objectives.krTitle')" required />
          <Input v-model="kr.unit" class="w-24" :placeholder="t('objectives.unit')" :aria-label="t('objectives.unit')" />
          <Input v-model="kr.start_value" type="number" step="any" class="w-24" :aria-label="t('objectives.startValue')" />
          <span>→</span>
          <Input v-model="kr.target_value" type="number" step="any" class="w-24" :aria-label="t('objectives.targetValue')" />
          <NativeSelect v-model="kr.calc_mode" class="w-32" :aria-label="t('objectives.mode')">
            <option value="derivado">{{ t('objectives.modes.derivado') }}</option>
            <option value="manual">{{ t('objectives.modes.manual') }}</option>
          </NativeSelect>
          <Button type="submit" size="sm">{{ t('objectives.addKeyResult') }}</Button>
        </form>
      </section>

      <div class="grid gap-4 lg:grid-cols-2">
        <section class="rounded-lg border bg-background p-4">
          <h2 class="mb-2 font-medium">{{ t('objectives.milestones') }}</h2>
          <ul class="flex flex-col gap-1 text-sm">
            <li v-for="m in objective.milestones" :key="m.id" class="flex flex-wrap items-center gap-2" data-testid="objective-milestone">
              <span class="font-mono text-xs text-muted-foreground">{{ m.project.identifier }}</span>
              <RouterLink :to="`/milestones/${m.id}`" class="hover:underline">{{ m.name }}</RouterLink>
              <Badge :variant="milestoneVariant[m.status]">{{ t(`milestones.statuses.${m.status}`) }}</Badge>
              <span class="text-xs text-muted-foreground">{{ m.completed_on ?? m.due_date }}</span>
              <Button v-if="manage" size="sm" variant="ghost" @click="unlink.mutate({ kind: 'milestones', id: m.id })">{{ t('common.remove') }}</Button>
            </li>
          </ul>
          <form v-if="manage" class="mt-2 flex flex-wrap items-center gap-2 text-sm" @submit.prevent="linkMilestone.mutate()">
            <NativeSelect v-model="msProject" class="w-44" :aria-label="t('objectives.project')">
              <option value="">{{ t('objectives.chooseProject') }}</option>
              <option v-for="p in projects?.items ?? []" :key="p.id" :value="p.id">{{ p.identifier }} · {{ p.name }}</option>
            </NativeSelect>
            <NativeSelect v-model="linkMilestoneId" class="w-44" :aria-label="t('objectives.linkMilestone')" required>
              <option value="" disabled>{{ t('objectives.chooseMilestone') }}</option>
              <option v-for="m in projectMilestones ?? []" :key="m.id" :value="m.id">{{ m.name }}</option>
            </NativeSelect>
            <Button type="submit" size="sm" variant="outline">{{ t('objectives.linkMilestone') }}</Button>
          </form>
        </section>

        <section class="rounded-lg border bg-background p-4">
          <h2 class="mb-2 font-medium">{{ t('objectives.projects') }}</h2>
          <ul class="flex flex-col gap-1 text-sm">
            <li v-for="p in objective.projects" :key="p.id" class="flex items-center gap-2" data-testid="objective-project">
              <RouterLink :to="`/projects/${p.id}`" class="hover:underline">{{ p.identifier }} · {{ p.name }}</RouterLink>
              <Button v-if="manage" size="sm" variant="ghost" @click="unlink.mutate({ kind: 'projects', id: p.id })">{{ t('common.remove') }}</Button>
            </li>
          </ul>
          <form v-if="manage" class="mt-2 flex items-center gap-2 text-sm" @submit.prevent="linkProject.mutate()">
            <NativeSelect v-model="linkProjectId" class="w-56" :aria-label="t('objectives.linkProject')" required>
              <option value="" disabled>{{ t('objectives.chooseProject') }}</option>
              <option v-for="p in projects?.items ?? []" :key="p.id" :value="p.id">{{ p.identifier }} · {{ p.name }}</option>
            </NativeSelect>
            <Button type="submit" size="sm" variant="outline">{{ t('objectives.linkProject') }}</Button>
          </form>
        </section>
      </div>
    </template>
  </div>
</template>

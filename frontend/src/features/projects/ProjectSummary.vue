<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  changeLifecycle,
  setProjectFields,
  setProjectTypes,
  setTemplate,
  updateProject,
  type LifecycleAction,
} from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import RichTextEditor from '@/features/richtext/RichTextEditor.vue'
import RichTextView from '@/features/richtext/RichTextView.vue'
import AttributeField from '@/features/users/AttributeField.vue'
import { useCatalog, useCustomFields } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { HEALTHS, type CustomValues, type Project } from '@/types/work'

// Data, lifecycle, enabled types and fields of a project (RF-18 to RF-33).
const props = defineProps<{ project: Project }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()
const { catalog } = useCatalog()
const fields = useCustomFields()
const { users, teams } = usePeople()

const writable = computed(() => props.project.can.manage && props.project.lifecycle === 'activo')
const projectFields = computed(() =>
  fields.value.filter((f) => f.entity === 'project' && props.project.custom_field_ids.includes(f.id)),
)

function toForm(p: Project) {
  return {
    name: p.name,
    teamId: p.team.id,
    ownerId: p.owner.id,
    startDate: p.start_date ?? '',
    endDate: p.end_date ?? '',
    health: p.health,
    explanation: p.health_explanation,
    description: p.description,
    values: { ...p.custom_values } as CustomValues,
  }
}
const editing = ref(false)
const form = ref(toForm(props.project))
watch(
  () => props.project,
  (p) => {
    if (!editing.value) form.value = toForm(p)
  },
)

function done(p: unknown) {
  qc.setQueryData(['project', props.project.id], p)
  return qc.invalidateQueries({ queryKey: ['projects'] })
}
const save = useMutation({
  mutationFn: () => {
    const f = form.value
    return updateProject(props.project.id, {
      version: props.project.version,
      name: f.name,
      description: f.description,
      team_id: f.teamId,
      owner_id: f.ownerId,
      parent_id: props.project.parent?.id ?? null,
      start_date: f.startDate || null,
      end_date: f.endDate || null,
      health: f.health,
      health_explanation: f.explanation,
      custom_values: f.values,
    })
  },
  onSuccess: (p) => {
    editing.value = false
    return done(p)
  },
  onError: (e) => emit('error', e),
})
const lifecycle = useMutation({
  mutationFn: (action: LifecycleAction) => changeLifecycle(props.project.id, action, props.project.version),
  onSuccess: done,
  onError: (e) => emit('error', e),
})
const types = useMutation({
  mutationFn: (ids: string[]) => setProjectTypes(props.project.id, ids),
  onSuccess: done,
  onError: (e) => emit('error', e),
})
const fieldsMutation = useMutation({
  mutationFn: (ids: string[]) => setProjectFields(props.project.id, ids),
  onSuccess: done,
  onError: (e) => emit('error', e),
})
const template = useMutation({
  mutationFn: () => setTemplate(props.project.id, !props.project.is_template),
  onSuccess: done,
  onError: (e) => emit('error', e),
})

function toggle(list: string[], id: string): string[] {
  return list.includes(id) ? list.filter((x) => x !== id) : [...list, id]
}
const actions = computed((): LifecycleAction[] => {
  switch (props.project.lifecycle) {
    case 'activo':
      return ['close', 'archive']
    case 'cerrado':
      return ['reopen', 'archive']
    default:
      return ['unarchive']
  }
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <section class="rounded-lg border bg-background p-4">
      <form v-if="editing" class="grid gap-3 sm:grid-cols-2" @submit.prevent="save.mutate()">
        <div class="flex flex-col gap-1.5">
          <Label for="p-name">{{ t('projects.name') }}</Label>
          <Input id="p-name" v-model="form.name" required />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="p-team">{{ t('projects.team') }}</Label>
          <NativeSelect id="p-team" v-model="form.teamId">
            <option v-for="{ team, depth } in teams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="p-owner">{{ t('projects.owner') }}</Label>
          <NativeSelect id="p-owner" v-model="form.ownerId">
            <option :value="project.owner.id">{{ project.owner.name }}</option>
            <option v-for="u in users.filter((x) => x.id !== project.owner.id)" :key="u.id" :value="u.id">{{ u.name }}</option>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="p-health">{{ t('projects.health') }}</Label>
          <NativeSelect id="p-health" v-model="form.health">
            <option v-for="h in HEALTHS" :key="h" :value="h">{{ t(`projects.healths.${h}`) }}</option>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <Label for="p-explanation">{{ t('projects.healthExplanation') }}</Label>
          <Input id="p-explanation" v-model="form.explanation" maxlength="2000" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="p-start">{{ t('projects.startDate') }}</Label>
          <Input id="p-start" v-model="form.startDate" type="date" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="p-end">{{ t('projects.endDate') }}</Label>
          <Input id="p-end" v-model="form.endDate" type="date" />
        </div>
        <div v-for="f in projectFields" :key="f.id" class="flex flex-col gap-1.5">
          <Label :for="`pf-${f.id}`">{{ f.name }}{{ f.required ? ' *' : '' }}</Label>
          <AttributeField :id="`pf-${f.id}`" v-model="form.values[f.id]" :attribute="f" :disabled="!!f.retired_at" />
        </div>
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <Label>{{ t('projects.description') }}</Label>
          <RichTextEditor v-model="form.description" />
        </div>
        <div class="flex gap-2 sm:col-span-2">
          <Button type="submit" :disabled="save.isPending.value">{{ t('common.save') }}</Button>
          <Button type="button" variant="ghost" @click="editing = false">{{ t('common.cancel') }}</Button>
        </div>
      </form>
      <div v-else class="flex flex-col gap-3">
        <dl class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-3">
          <div><dt class="text-muted-foreground">{{ t('projects.team') }}</dt><dd>{{ project.team.name }}</dd></div>
          <div><dt class="text-muted-foreground">{{ t('projects.owner') }}</dt><dd>{{ project.owner.name }}</dd></div>
          <div>
            <dt class="text-muted-foreground">{{ t('projects.methodology') }}</dt>
            <dd>{{ project.methodology?.name ?? t('common.none') }}</dd>
          </div>
          <div><dt class="text-muted-foreground">{{ t('projects.startDate') }}</dt><dd>{{ project.start_date ?? t('common.none') }}</dd></div>
          <div><dt class="text-muted-foreground">{{ t('projects.endDate') }}</dt><dd>{{ project.end_date ?? t('common.none') }}</dd></div>
          <div>
            <dt class="text-muted-foreground">{{ t('projects.healthExplanation') }}</dt>
            <dd>{{ project.health_explanation || t('common.none') }}</dd>
          </div>
          <div v-for="f in projectFields" :key="f.id">
            <dt class="text-muted-foreground">{{ f.name }}</dt>
            <dd>{{ project.custom_values[f.id] ?? t('common.none') }}</dd>
          </div>
        </dl>
        <RichTextView :doc="project.description" />
        <div v-if="project.can.manage" class="flex flex-wrap gap-2">
          <Button v-if="writable" size="sm" @click="editing = true">{{ t('common.edit') }}</Button>
          <Button v-for="a in actions" :key="a" size="sm" variant="outline" @click="lifecycle.mutate(a)">
            {{ t(`projects.actions.${a}`) }}
          </Button>
          <Button v-if="project.can.manage_template" size="sm" variant="ghost" @click="template.mutate()">
            {{ project.is_template ? t('projects.unmarkTemplate') : t('projects.markTemplate') }}
          </Button>
        </div>
      </div>
    </section>

    <section v-if="project.can.manage" class="grid gap-4 sm:grid-cols-2">
      <div class="rounded-lg border bg-background p-4 text-sm">
        <h2 class="mb-2 font-medium">{{ t('projects.enabledTypes') }}</h2>
        <label v-for="ty in catalog.work_item_types.filter((x) => !x.retired || project.work_item_type_ids.includes(x.id))" :key="ty.id" class="flex items-center gap-2">
          <input
            type="checkbox"
            :checked="project.work_item_type_ids.includes(ty.id)"
            :disabled="!writable || types.isPending.value"
            @change="types.mutate(toggle(project.work_item_type_ids, ty.id))"
          />
          {{ ty.name }}
        </label>
      </div>
      <div class="rounded-lg border bg-background p-4 text-sm">
        <h2 class="mb-2 font-medium">{{ t('projects.enabledFields') }}</h2>
        <p v-if="!fields.length" class="text-muted-foreground">{{ t('common.empty') }}</p>
        <label v-for="f in fields.filter((x) => !x.retired_at || project.custom_field_ids.includes(x.id))" :key="f.id" class="flex items-center gap-2">
          <input
            type="checkbox"
            :checked="project.custom_field_ids.includes(f.id)"
            :disabled="!writable || fieldsMutation.isPending.value"
            @change="fieldsMutation.mutate(toggle(project.custom_field_ids, f.id))"
          />
          {{ f.name }} <span class="text-xs text-muted-foreground">({{ t(`catalog.entities.${f.entity}`) }})</span>
        </label>
      </div>
    </section>
  </div>
</template>

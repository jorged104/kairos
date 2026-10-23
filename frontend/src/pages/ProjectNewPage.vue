<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { createFromTemplate, createProject, listTemplates } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import RichTextEditor from '@/features/richtext/RichTextEditor.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'
import type { Project, RichDoc } from '@/types/work'

// New project, blank or from a template (RF-18 to RF-27, RF-36 to RF-38).
const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const { catalog } = useCatalog()
const { users, ownTeams } = usePeople()

const canUseTemplates = computed(() => auth.can('templates:use'))
const { data: templates } = useQuery({ queryKey: ['templates'], queryFn: listTemplates, enabled: canUseTemplates })

const form = ref({
  templateId: '',
  includeContent: true,
  identifier: '',
  name: '',
  teamId: '',
  ownerId: '',
  methodologyId: '',
  startDate: '',
  endDate: '',
  description: null as RichDoc,
})
const methodologies = computed(() => catalog.value.methodologies.filter((m) => !m.retired))
const template = computed(() => templates.value?.find((x) => x.id === form.value.templateId))

const save = useMutation({
  mutationFn: (): Promise<Project> => {
    const f = form.value
    const common = {
      identifier: f.identifier,
      name: f.name,
      team_id: f.teamId,
      owner_id: f.ownerId || undefined,
      start_date: f.startDate || null,
      end_date: f.endDate || null,
    }
    if (f.templateId) return createFromTemplate({ ...common, template_id: f.templateId, include_content: f.includeContent })
    return createProject({ ...common, methodology_id: f.methodologyId || undefined, description: f.description })
  },
  onSuccess: (p) => router.push(`/projects/${p.id}`),
})
</script>

<template>
  <Card class="mx-auto max-w-3xl">
    <CardHeader>
      <CardTitle>{{ t('projects.new') }}</CardTitle>
    </CardHeader>
    <CardContent>
      <form class="grid gap-4 sm:grid-cols-2" @submit.prevent="save.mutate()">
        <Alert v-if="save.error.value" variant="destructive" role="alert" class="sm:col-span-2">
          <AlertDescription>{{ errorMessage(save.error.value) }}</AlertDescription>
        </Alert>
        <div v-if="canUseTemplates" class="flex flex-col gap-1.5 sm:col-span-2">
          <Label for="template">{{ t('projects.template') }}</Label>
          <NativeSelect id="template" v-model="form.templateId">
            <option value="">{{ t('projects.blank') }}</option>
            <option v-for="tpl in templates ?? []" :key="tpl.id" :value="tpl.id">
              {{ tpl.name }}{{ tpl.methodology ? ` · ${tpl.methodology.name}` : '' }}
            </option>
          </NativeSelect>
          <label v-if="form.templateId" class="flex items-center gap-2 text-sm">
            <input v-model="form.includeContent" type="checkbox" />
            {{ t('projects.includeContent') }}
          </label>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="identifier">{{ t('projects.identifier') }}</Label>
          <Input id="identifier" v-model="form.identifier" maxlength="10" class="uppercase" required />
          <p class="text-xs text-muted-foreground">{{ t('projects.identifierHelp') }}</p>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="name">{{ t('projects.name') }}</Label>
          <Input id="name" v-model="form.name" required />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="team">{{ t('projects.team') }}</Label>
          <NativeSelect id="team" v-model="form.teamId" required>
            <option value="" disabled>{{ t('projects.chooseTeam') }}</option>
            <option v-for="{ team, depth } in ownTeams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="owner">{{ t('projects.owner') }}</Label>
          <NativeSelect id="owner" v-model="form.ownerId">
            <option value="">{{ t('projects.ownerMe') }}</option>
            <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
          </NativeSelect>
        </div>
        <div v-if="!form.templateId" class="flex flex-col gap-1.5">
          <Label for="methodology">{{ t('projects.methodology') }}</Label>
          <NativeSelect id="methodology" v-model="form.methodologyId">
            <option value="">{{ t('common.none') }}</option>
            <option v-for="m in methodologies" :key="m.id" :value="m.id">{{ m.name }}</option>
          </NativeSelect>
        </div>
        <p v-else-if="template?.methodology" class="self-end text-sm text-muted-foreground">
          {{ t('projects.methodology') }}: {{ template.methodology.name }}
        </p>
        <div class="flex flex-col gap-1.5">
          <Label for="start">{{ t('projects.startDate') }}</Label>
          <Input id="start" v-model="form.startDate" type="date" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="end">{{ t('projects.endDate') }}</Label>
          <Input id="end" v-model="form.endDate" type="date" />
        </div>
        <div v-if="!form.templateId" class="flex flex-col gap-1.5 sm:col-span-2">
          <Label>{{ t('projects.description') }}</Label>
          <RichTextEditor v-model="form.description" />
        </div>
        <div class="flex gap-2 sm:col-span-2">
          <Button type="submit" :disabled="save.isPending.value">{{ t('common.create') }}</Button>
          <Button type="button" variant="ghost" @click="router.back()">{{ t('common.cancel') }}</Button>
        </div>
      </form>
    </CardContent>
  </Card>
</template>

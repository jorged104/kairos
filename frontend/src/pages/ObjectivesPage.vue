<script setup lang="ts">
import { keepPreviousData, useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { createObjective, listObjectives, listProjects } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { objectiveVariant } from '@/features/work/Badges'
import ProgressBar from '@/features/work/ProgressBar.vue'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'
import { OBJECTIVE_SCOPES, OBJECTIVE_STATUSES, type ObjectiveScope } from '@/types/work'

// Objectives: list with filters (RF-131) and creation (RF-97, RF-98).
const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const { ownTeams } = usePeople()

const status = ref('')
const scope = ref('')
const from = ref('')
const to = ref('')
const page = ref(1)
watch([status, scope, from, to], () => (page.value = 1))
const query = computed(() => ({ status: status.value, scope: scope.value, from: from.value, to: to.value, page: page.value }))
const { data, error } = useQuery({
  queryKey: ['objectives', query],
  queryFn: () => listObjectives(query.value),
  placeholderData: keepPreviousData,
})
const pages = computed(() => (data.value ? Math.max(1, Math.ceil(data.value.total / data.value.page_size)) : 1))

// Scopes the user can create: organization only for admins (RF-98, matrix).
const canCreate = computed(() => auth.can('objectives:create:all', 'objectives:create:team'))
const scopes = computed(() => OBJECTIVE_SCOPES.filter((s) => s !== 'organizacion' || auth.can('objectives:create:all')))
const { data: projects } = useQuery({ queryKey: ['projects', 'for-objectives'], queryFn: () => listProjects({ page_size: 100 }), enabled: canCreate })
const form = ref({ title: '', scope: 'equipo' as ObjectiveScope, teamId: '', projectId: '', start: '', end: '' })
const create = useMutation({
  mutationFn: () => {
    const f = form.value
    return createObjective({
      title: f.title,
      scope: f.scope,
      scope_team_id: f.scope === 'equipo' ? f.teamId : undefined,
      scope_project_id: f.scope === 'proyecto' ? f.projectId : undefined,
      period_start: f.start,
      period_end: f.end,
    })
  },
  onSuccess: (o) => router.push(`/objectives/${o.id}`),
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="font-heading text-xl font-semibold">{{ t('objectives.title') }}</h1>

    <form v-if="canCreate" class="grid gap-3 rounded-lg border bg-background p-3 sm:grid-cols-3" @submit.prevent="create.mutate()">
      <Alert v-if="create.error.value" variant="destructive" role="alert" class="sm:col-span-3">
        <AlertDescription>{{ errorMessage(create.error.value) }}</AlertDescription>
      </Alert>
      <div class="flex flex-col gap-1.5 sm:col-span-3">
        <Label for="o-title">{{ t('objectives.new') }}</Label>
        <Input id="o-title" v-model="form.title" :placeholder="t('objectives.titlePlaceholder')" required />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="o-scope">{{ t('objectives.scope') }}</Label>
        <NativeSelect id="o-scope" v-model="form.scope">
          <option v-for="s in scopes" :key="s" :value="s">{{ t(`objectives.scopes.${s}`) }}</option>
        </NativeSelect>
      </div>
      <div v-if="form.scope === 'equipo'" class="flex flex-col gap-1.5">
        <Label for="o-team">{{ t('projects.team') }}</Label>
        <NativeSelect id="o-team" v-model="form.teamId" required>
          <option value="" disabled>{{ t('projects.chooseTeam') }}</option>
          <option v-for="{ team, depth } in ownTeams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
        </NativeSelect>
      </div>
      <div v-else-if="form.scope === 'proyecto'" class="flex flex-col gap-1.5">
        <Label for="o-project">{{ t('objectives.project') }}</Label>
        <NativeSelect id="o-project" v-model="form.projectId" required>
          <option value="" disabled>{{ t('objectives.chooseProject') }}</option>
          <option v-for="p in projects?.items ?? []" :key="p.id" :value="p.id">{{ p.identifier }} · {{ p.name }}</option>
        </NativeSelect>
      </div>
      <div v-else />
      <div class="flex items-end gap-2">
        <div class="flex flex-col gap-1.5">
          <Label for="o-start">{{ t('objectives.periodStart') }}</Label>
          <Input id="o-start" v-model="form.start" type="date" required />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="o-end">{{ t('objectives.periodEnd') }}</Label>
          <Input id="o-end" v-model="form.end" type="date" required />
        </div>
      </div>
      <div><Button type="submit" :disabled="create.isPending.value">{{ t('common.create') }}</Button></div>
    </form>

    <div class="grid gap-3 rounded-lg border bg-background p-3 sm:grid-cols-4">
      <NativeSelect v-model="status" :aria-label="t('objectives.status')">
        <option value="">{{ t('objectives.anyStatus') }}</option>
        <option v-for="s in OBJECTIVE_STATUSES" :key="s" :value="s">{{ t(`objectives.statuses.${s}`) }}</option>
      </NativeSelect>
      <NativeSelect v-model="scope" :aria-label="t('objectives.scope')">
        <option value="">{{ t('objectives.anyScope') }}</option>
        <option v-for="s in OBJECTIVE_SCOPES" :key="s" :value="s">{{ t(`objectives.scopes.${s}`) }}</option>
      </NativeSelect>
      <Input v-model="from" type="date" :aria-label="t('objectives.periodStart')" />
      <Input v-model="to" type="date" :aria-label="t('objectives.periodEnd')" />
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <ul class="flex flex-col gap-2">
      <li v-for="o in data?.items ?? []" :key="o.id" class="grid gap-2 rounded-lg border bg-background p-3 sm:grid-cols-[1fr_14rem]" data-testid="objective-row">
        <div class="flex flex-wrap items-center gap-2">
          <RouterLink :to="`/objectives/${o.id}`" class="font-medium hover:underline">{{ o.title }}</RouterLink>
          <Badge :variant="objectiveVariant[o.status]">{{ t(`objectives.statuses.${o.status}`) }}</Badge>
          <span class="text-xs text-muted-foreground">
            {{ t(`objectives.scopes.${o.scope}`) }} · {{ o.period_start }} – {{ o.period_end }} · {{ o.owner.name }}
          </span>
        </div>
        <ProgressBar :value="o.progress" :label="t('objectives.progress')" />
      </li>
    </ul>
    <p v-if="data && data.items.length === 0" class="text-sm text-muted-foreground">{{ t('common.empty') }}</p>
    <div v-if="data" class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pageOf', { page: data.page, pages, total: data.total }) }}</span>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" :disabled="page <= 1" @click="page--">{{ t('common.previous') }}</Button>
        <Button variant="outline" size="sm" :disabled="page >= pages" @click="page++">{{ t('common.next') }}</Button>
      </div>
    </div>
  </div>
</template>

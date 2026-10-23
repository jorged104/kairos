<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { X } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  addLeader,
  createTeam,
  listTeams,
  listUsers,
  removeLeader,
  setTeamArchived,
  updateTeam,
  type TeamInput,
} from '@/api/admin'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { descendants, flatten } from '@/features/teams/tree'
import { useAuthStore } from '@/stores/auth'
import type { Team } from '@/types/api'

const { t } = useI18n()
const auth = useAuthStore()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const canManage = computed(() => auth.can('teams:manage'))

const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams })
// Only users with the lead role can lead a team (RF-91).
const { data: leads } = useQuery({
  queryKey: ['users', 'leads'],
  queryFn: () => listUsers({ role: 'lead', status: 'active', page_size: 100 }),
  enabled: canManage,
})

const showArchived = ref(false)
const rows = computed(() => flatten((teams.value ?? []).filter((tm) => showArchived.value || !tm.archived_at)))
const activeTeams = computed(() => flatten((teams.value ?? []).filter((tm) => !tm.archived_at)))

const error = ref<unknown>(null)
function mutation<A>(fn: (arg: A) => Promise<unknown>) {
  return useMutation({
    mutationFn: fn,
    onMutate: () => (error.value = null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['teams'] }),
    onError: (e) => (error.value = e),
  })
}

// Forms keep strings; empty parent or IdP reference means none.
interface TeamForm {
  name: string
  parent: string
  ref: string
}
const blank = (): TeamForm => ({ name: '', parent: '', ref: '' })
const toInput = (f: TeamForm): TeamInput => ({ name: f.name, parent_id: f.parent || null, external_ref: f.ref || null })

const form = ref<TeamForm>(blank())
const create = mutation(() => createTeam(toInput(form.value)))
async function submitNew() {
  await create.mutateAsync(undefined).catch(() => undefined)
  if (!error.value) form.value = blank()
}

const editing = ref<string | null>(null)
const edit = ref<TeamForm>(blank())
function startEdit(team: Team) {
  editing.value = team.id
  edit.value = { name: team.name, parent: team.parent_id ?? '', ref: team.external_ref ?? '' }
}
const update = mutation(() => updateTeam(editing.value as string, toInput(edit.value)))
async function submitEdit() {
  await update.mutateAsync(undefined).catch(() => undefined)
  if (!error.value) editing.value = null
}
// A team can't hang from itself or one of its subteams (RF-90a).
const parentOptions = computed(() => {
  if (!editing.value) return activeTeams.value
  const excluded = descendants(teams.value ?? [], editing.value)
  return activeTeams.value.filter(({ team }) => !excluded.has(team.id))
})

const archive = mutation((team: Team) => setTeamArchived(team.id, !team.archived_at))
const leaderAdd = mutation(({ team, user }: { team: string; user: string }) => addLeader(team, user))
const leaderRemove = mutation(({ team, user }: { team: string; user: string }) => removeLeader(team, user))
const newLeader = ref<Record<string, string>>({})

</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="font-heading text-xl font-semibold">{{ t('teams.title') }}</h1>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="showArchived" type="checkbox" />
        {{ t('teams.showArchived') }}
      </label>
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <Card v-if="canManage">
      <CardHeader>
        <CardTitle>{{ t('teams.new') }}</CardTitle>
      </CardHeader>
      <CardContent>
        <form class="grid gap-3 sm:grid-cols-4" @submit.prevent="submitNew">
          <div class="flex flex-col gap-1.5">
            <Label for="team-name">{{ t('teams.name') }}</Label>
            <Input id="team-name" v-model="form.name" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="team-parent">{{ t('teams.parent') }}</Label>
            <NativeSelect id="team-parent" v-model="form.parent">
              <option value="">{{ t('teams.noParent') }}</option>
              <option v-for="{ team, depth } in activeTeams" :key="team.id" :value="team.id">
                {{ '  '.repeat(depth) + team.name }}
              </option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="team-ref">{{ t('teams.externalRef') }}</Label>
            <Input id="team-ref" v-model="form.ref" placeholder="/Areas/BI" />
          </div>
          <div class="flex items-end">
            <Button type="submit" :disabled="create.isPending.value">{{ t('common.create') }}</Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <ul class="flex flex-col gap-2">
      <li
        v-for="{ team, depth } in rows"
        :key="team.id"
        class="rounded-lg border bg-background p-3"
        :style="{ marginLeft: `${depth * 1.5}rem` }"
        data-testid="team-row"
      >
        <form v-if="editing === team.id" class="grid gap-2 sm:grid-cols-4" @submit.prevent="submitEdit">
          <Input v-model="edit.name" :aria-label="t('teams.name')" required />
          <NativeSelect v-model="edit.parent" :aria-label="t('teams.parent')">
            <option value="">{{ t('teams.noParent') }}</option>
            <option v-for="o in parentOptions" :key="o.team.id" :value="o.team.id">
              {{ '  '.repeat(o.depth) + o.team.name }}
            </option>
          </NativeSelect>
          <Input v-model="edit.ref" :aria-label="t('teams.externalRef')" />
          <div class="flex gap-2">
            <Button type="submit" size="sm">{{ t('common.save') }}</Button>
            <Button type="button" size="sm" variant="ghost" @click="editing = null">{{ t('common.cancel') }}</Button>
          </div>
        </form>
        <div v-else class="flex flex-wrap items-center gap-2">
          <span class="font-medium">{{ team.name }}</span>
          <Badge v-if="team.archived_at" variant="secondary">{{ t('teams.archived') }}</Badge>
          <span class="text-xs text-muted-foreground">{{ t('teams.members', { count: team.member_count }) }}</span>
          <span v-if="team.external_ref" class="font-mono text-xs text-muted-foreground">{{ team.external_ref }}</span>
          <div v-if="canManage" class="ml-auto flex gap-1">
            <Button size="sm" variant="ghost" @click="startEdit(team)">{{ t('common.edit') }}</Button>
            <Button size="sm" variant="ghost" @click="archive.mutate(team)">
              {{ team.archived_at ? t('teams.unarchive') : t('teams.archive') }}
            </Button>
          </div>
        </div>
        <div class="mt-2 flex flex-wrap items-center gap-2 text-sm">
          <span class="text-muted-foreground">{{ t('teams.leaders') }}:</span>
          <span v-for="l in team.leaders" :key="l.user_id" class="inline-flex items-center gap-1">
            <Badge variant="outline">{{ l.name }}</Badge>
            <Button
              v-if="canManage"
              size="icon-xs"
              variant="ghost"
              :aria-label="t('common.remove')"
              @click="leaderRemove.mutate({ team: team.id, user: l.user_id })"
            >
              <X />
            </Button>
          </span>
          <span v-if="team.leaders.length === 0">{{ t('common.none') }}</span>
          <form
            v-if="canManage && !team.archived_at"
            class="ml-auto flex gap-2"
            @submit.prevent="newLeader[team.id] && leaderAdd.mutate({ team: team.id, user: newLeader[team.id] })"
          >
            <NativeSelect v-model="newLeader[team.id]" class="w-56" :aria-label="t('teams.addLeader')">
              <option value="">{{ t('teams.leaderPlaceholder') }}</option>
              <option v-for="u in leads?.items ?? []" :key="u.id" :value="u.id">{{ u.name }}</option>
            </NativeSelect>
            <Button type="submit" size="sm" variant="outline">{{ t('teams.addLeader') }}</Button>
          </form>
        </div>
      </li>
    </ul>
  </div>
</template>

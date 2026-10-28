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
import { EditableCell, GridTable, QuickAddRow, type CellOption } from '@/components/data-table'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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

// Cells save one field of the team; empty parent or IdP reference means none (RF-128 de la spec 003).
const input = (team: Team): TeamInput => ({ name: team.name, parent_id: team.parent_id, external_ref: team.external_ref })
function saver(team: Team, field: keyof TeamInput) {
  return async (value: string) => {
    await updateTeam(team.id, { ...input(team), [field]: field === 'name' ? value : value || null })
    await qc.invalidateQueries({ queryKey: ['teams'] })
  }
}
// A team can't hang from itself or one of its subteams (RF-90a).
const parentOptions = (team: Team): CellOption[] => {
  const excluded = descendants(teams.value ?? [], team.id)
  return activeTeams.value
    .filter(({ team: o }) => !excluded.has(o.id))
    .map(({ team: o, depth }) => ({ value: o.id, label: '\u00a0\u00a0'.repeat(depth) + o.name }))
}
const parentName = (team: Team) => teams.value?.find((o) => o.id === team.parent_id)?.name

const newParent = ref('')
async function create(name: string) {
  await createTeam({ name, parent_id: newParent.value || null, external_ref: null })
  await qc.invalidateQueries({ queryKey: ['teams'] })
}

const archive = mutation((team: Team) => setTeamArchived(team.id, !team.archived_at))
const leaderAdd = mutation(({ team, user }: { team: string; user: string }) => addLeader(team, user))
const leaderRemove = mutation(({ team, user }: { team: string; user: string }) => removeLeader(team, user))
function pickLeader(team: string, event: Event) {
  const select = event.target as HTMLSelectElement
  if (select.value) leaderAdd.mutate({ team, user: select.value })
  select.value = ''
}

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

    <GridTable :empty="!canManage && rows.length === 0">
      <thead>
        <tr>
          <th>{{ t('teams.name') }}</th>
          <th class="w-48">{{ t('teams.parent') }}</th>
          <th class="w-44">{{ t('teams.externalRef') }}</th>
          <th class="w-28">{{ t('teams.membersColumn') }}</th>
          <th>{{ t('teams.leaders') }}</th>
          <th v-if="canManage" class="w-28" />
        </tr>
      </thead>
      <tbody>
        <tr v-for="{ team, depth } in rows" :key="team.id" class="hover:bg-muted/30" data-testid="team-row">
          <EditableCell
            :value="team.name"
            :label="t('teams.name')"
            :save="saver(team, 'name')"
            :editable="canManage && !team.archived_at"
            :style="{ paddingLeft: `${depth * 1.25 + 0.5}rem` }"
            required
          >
            <span class="font-medium">{{ team.name }}</span>
            <Badge v-if="team.archived_at" variant="secondary">{{ t('teams.archived') }}</Badge>
          </EditableCell>
          <EditableCell
            kind="select"
            :value="team.parent_id ?? ''"
            :label="t('teams.parent')"
            :options="parentOptions(team)"
            :save="saver(team, 'parent_id')"
            :editable="canManage && !team.archived_at"
          >
            <span :class="!team.parent_id && 'text-muted-foreground'">{{ parentName(team) ?? t('teams.noParent') }}</span>
          </EditableCell>
          <EditableCell
            :value="team.external_ref ?? ''"
            :label="t('teams.externalRef')"
            :save="saver(team, 'external_ref')"
            :editable="canManage && !team.archived_at"
            class="font-mono text-xs"
          />
          <td class="bg-muted/20 px-2 text-muted-foreground">{{ team.member_count }}</td>
          <td class="px-2 py-1">
            <div class="flex flex-wrap items-center gap-1">
              <span v-for="l in team.leaders" :key="l.user_id" class="inline-flex items-center gap-0.5 rounded-md bg-muted px-2 py-0.5 text-xs">
                {{ l.name }}
                <button
                  v-if="canManage"
                  type="button"
                  class="text-muted-foreground hover:text-foreground"
                  :aria-label="t('teams.removeLeader', { name: l.name })"
                  @click="leaderRemove.mutate({ team: team.id, user: l.user_id })"
                >
                  <X class="size-3" />
                </button>
              </span>
              <select
                v-if="canManage && !team.archived_at"
                class="h-7 rounded-md border border-dashed bg-transparent px-1 text-xs text-muted-foreground"
                :aria-label="t('teams.addLeader')"
                @change="pickLeader(team.id, $event)"
              >
                <option value="">{{ t('teams.addLeader') }}</option>
                <option v-for="u in leads?.items ?? []" :key="u.id" :value="u.id">{{ u.name }}</option>
              </select>
              <span v-else-if="team.leaders.length === 0" class="text-muted-foreground">{{ t('common.none') }}</span>
            </div>
          </td>
          <td v-if="canManage" class="px-2 text-right">
            <Button size="sm" variant="ghost" @click="archive.mutate(team)">
              {{ team.archived_at ? t('teams.unarchive') : t('teams.archive') }}
            </Button>
          </td>
        </tr>
        <QuickAddRow v-if="canManage" :colspan="6" :label="t('teams.new')" :placeholder="t('teams.name')" :submit="create">
          <NativeSelect v-model="newParent" class="w-48" :aria-label="t('teams.parent')">
            <option value="">{{ t('teams.noParent') }}</option>
            <option v-for="{ team, depth } in activeTeams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
          </NativeSelect>
        </QuickAddRow>
      </tbody>
    </GridTable>
  </div>
</template>

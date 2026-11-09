<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { UserMinus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  PROJECT_ROLES, addMember, listMembers, listSuggestions, removeMember, setCapacity, setMemberRole,
  type Member, type ProjectRole, type Suggestion,
} from '@/api/projectScreens'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { getDashboardPeople } from '@/api/dashboard'
import { localDate } from '@/api/metrics'
import { getProjectTeamSchedule } from '@/api/workload'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import LabelChips from '@/features/work/LabelChips.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import PersonSchedule from '@/features/workload/PersonSchedule.vue'
import { patchWorkItem } from '@/features/workitems/patch'
import { useAuthStore } from '@/stores/auth'
import type { Label, Project } from '@/types/work'

// The project's team for who manages it: roles, capacity and load, and the
// work items to assign with suggestions (spec 008, RF-33 to RF-54), and the
// schedule by person (spec 021, RF-16 to RF-18).
const props = defineProps<{ project: Project }>()
const { t, locale } = useI18n()
const qc = useQueryClient()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const { users } = usePeople()
const { labels } = useCatalog()

const membersKey = computed(() => ['members', props.project.id])
const suggestionsKey = computed(() => ['suggestions', props.project.id])
const { data: members } = useQuery({ queryKey: membersKey, queryFn: () => listMembers(props.project.id) })
const { data: suggestions } = useQuery({ queryKey: suggestionsKey, queryFn: () => listSuggestions(props.project.id) })
const scheduleKey = computed(() => ['team-schedule', props.project.id])
const { data: schedule } = useQuery({ queryKey: scheduleKey, queryFn: () => getProjectTeamSchedule(props.project.id) })
const writable = computed(() => props.project.lifecycle === 'activo')
// A name opens the panel of whom the viewer may see (spec 021, RF-14).
const canSeeOthers = computed(() => auth.can('dashboard:others:all', 'dashboard:others:team'))
const { data: visible } = useQuery({ queryKey: ['dashboard', 'people'], queryFn: getDashboardPeople, enabled: canSeeOthers })
const linkable = (id: string) => id === auth.me?.id || (visible.value ?? []).some((p) => p.id === id)
const today = localDate()
const canCapacity = computed(() => auth.can('users:update:all') || auth.can('users:update:team'))

const error = ref<unknown>(null)
const notice = ref('')
async function run(fn: () => Promise<unknown>, done?: string) {
  error.value = null
  notice.value = ''
  try {
    await fn()
    if (done) notice.value = done
  } catch (e) {
    error.value = e
  }
  await Promise.all([
    qc.invalidateQueries({ queryKey: membersKey.value }),
    qc.invalidateQueries({ queryKey: suggestionsKey.value }),
    qc.invalidateQueries({ queryKey: scheduleKey.value }),
    qc.invalidateQueries({ queryKey: ['overview', props.project.id] }),
    qc.invalidateQueries({ queryKey: ['tree', props.project.id] }),
  ])
}

const changeRole = (m: Member, role: ProjectRole) => run(() => setMemberRole(props.project.id, m.user.id, role))
const remove = (m: Member) => run(() => removeMember(props.project.id, m.user.id))
function changeCapacity(m: Member, value: string) {
  const v = value.trim() === '' ? null : Number(value)
  if (v === m.capacity) return
  void run(() => setCapacity(m.user.id, v))
}

// Adding someone from outside the team.
const newUser = ref('')
const newRole = ref<ProjectRole>('member')
const outsiders = computed(() => users.value.filter((u) => !(members.value ?? []).some((m) => m.user.id === u.id)))
async function add() {
  if (!newUser.value) return
  await run(() => addMember(props.project.id, newUser.value, newRole.value))
  newUser.value = ''
}

// Assigning (RF-53, RF-54).
const workers = computed(() => (members.value ?? []).filter((m) => m.can_work && m.role !== 'observer'))
const assign = (s: Suggestion, userId: string, name: string) =>
  run(() => patchWorkItem(s.id, { assignee_ids: [userId], assignee_id: userId }), t('team.assigned', { key: s.key, name }))
const labelsOf = (ids: string[]): Label[] => ids.map((id) => labels.value.find((l) => l.id === id)).filter((l): l is Label => !!l)
const pct = (m: Member) => (m.capacity ? Math.min(100, Math.round((m.load.total / m.capacity) * 100)) : 0)
const date = (d: string | null) => (d ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short' }).format(new Date(d + 'T00:00:00')) : t('common.none'))
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="team">
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <p v-if="notice" role="status" class="text-sm text-muted-foreground" data-testid="team-notice">{{ notice }}</p>

    <div class="flex flex-wrap items-start gap-4">
      <section class="flex min-w-0 flex-[999_1_640px] flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="team-members">
        <div class="flex flex-wrap items-center gap-2">
          <h2 id="team-members" class="text-base font-semibold">{{ t('team.members') }}</h2>
          <span class="text-sm text-muted-foreground">{{ t('team.count', { n: members?.length ?? 0 }) }}</span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[720px] text-sm">
            <thead>
              <tr class="text-left text-xs text-muted-foreground">
                <th class="py-2 pr-2 font-medium">{{ t('team.person') }}</th>
                <th class="w-44 p-2 font-medium">{{ t('team.role') }}</th>
                <th class="w-28 p-2 font-medium">{{ t('team.capacity') }}</th>
                <th class="w-56 p-2 font-medium">{{ t('team.load') }}</th>
                <th class="w-24 p-2 font-medium">{{ t('team.items') }}</th>
                <th class="w-10" />
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in members ?? []" :key="m.user.id" class="border-t" data-testid="team-member">
                <td class="py-2 pr-2">
                  <span class="font-medium">{{ m.user.name }}</span>
                  <span class="block text-xs text-muted-foreground">
                    {{ m.owner ? t('team.owner') : m.in_team ? t('team.fromTeam') : t('team.added') }}<template v-if="m.status !== 'active'"> · {{ t(`status.${m.status}`) }}</template>
                  </span>
                </td>
                <td class="p-2">
                  <select
                    :value="m.role"
                    :disabled="m.owner || !writable"
                    class="h-8 w-full rounded-md border bg-background px-2 text-sm disabled:opacity-60"
                    :aria-label="t('team.roleOf', { name: m.user.name })"
                    data-testid="team-role"
                    @change="changeRole(m, ($event.target as HTMLSelectElement).value as ProjectRole)"
                  >
                    <option v-for="r in PROJECT_ROLES" :key="r" :value="r" :disabled="r !== 'observer' && !m.can_work">{{ t(`team.roles.${r}`) }}</option>
                  </select>
                </td>
                <td class="p-2">
                  <input
                    v-if="canCapacity"
                    type="number"
                    min="0"
                    max="1000"
                    :value="m.capacity ?? ''"
                    class="h-8 w-20 rounded-md border bg-background px-2 text-sm"
                    :aria-label="t('team.capacityOf', { name: m.user.name })"
                    data-testid="team-capacity"
                    @change="changeCapacity(m, ($event.target as HTMLInputElement).value)"
                  />
                  <span v-else class="text-muted-foreground">{{ m.capacity ?? t('common.none') }}</span>
                </td>
                <td class="p-2">
                  <span class="flex items-center gap-2">
                    <span class="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                      <span class="block h-full rounded-full" :class="m.load.overloaded ? 'bg-orange-500' : 'bg-primary'" :style="{ width: `${pct(m)}%` }" />
                    </span>
                    <span class="w-24 text-xs" :class="m.load.overloaded ? 'font-semibold text-orange-700 dark:text-orange-400' : 'text-muted-foreground'" data-testid="team-load">
                      {{ m.capacity === null ? m.load.total : `${m.load.total} / ${m.capacity}` }}<template v-if="m.load.project !== m.load.total"> ({{ t('team.here', { n: m.load.project }) }})</template>
                    </span>
                  </span>
                </td>
                <td class="p-2 text-muted-foreground">{{ m.load.items }}</td>
                <td class="p-2">
                  <Button
                    v-if="m.explicit && !m.in_team && !m.owner && writable"
                    size="icon"
                    variant="ghost"
                    :aria-label="t('team.remove', { name: m.user.name })"
                    data-testid="team-remove"
                    @click="remove(m)"
                  >
                    <UserMinus aria-hidden="true" />
                  </Button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <form v-if="writable" class="flex flex-wrap items-center gap-2 border-t pt-3" @submit.prevent="add">
          <select v-model="newUser" class="h-9 min-w-56 flex-1 rounded-md border bg-background px-2 text-sm" :aria-label="t('team.addPerson')" data-testid="team-add-user">
            <option value="">{{ t('team.addPerson') }}</option>
            <option v-for="u in outsiders" :key="u.id" :value="u.id">{{ u.name }}</option>
          </select>
          <select v-model="newRole" class="h-9 rounded-md border bg-background px-2 text-sm" :aria-label="t('team.role')" data-testid="team-add-role">
            <option v-for="r in PROJECT_ROLES" :key="r" :value="r">{{ t(`team.roles.${r}`) }}</option>
          </select>
          <Button type="submit" size="sm" :disabled="!newUser" data-testid="team-add">{{ t('common.add') }}</Button>
        </form>
        <p class="text-xs text-muted-foreground">{{ t('team.loadHint') }}</p>
      </section>

      <section class="flex min-w-0 flex-[1_1_380px] flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="team-pending" data-testid="team-pending">
        <div class="flex items-center gap-2">
          <h2 id="team-pending" class="text-base font-semibold">{{ t('team.pending') }}</h2>
          <span class="rounded-full bg-muted px-2 text-xs font-semibold">{{ suggestions?.length ?? 0 }}</span>
        </div>
        <p class="text-sm text-muted-foreground">{{ t('team.pendingHint') }}</p>
        <p v-if="suggestions && !suggestions.length" class="rounded-lg bg-muted p-4 text-center text-sm">{{ t('team.allAssigned') }}</p>
        <article v-for="s in suggestions ?? []" :key="s.id" class="flex flex-col gap-2 rounded-lg border p-3" data-testid="team-suggestion">
          <p class="text-sm"><span class="font-mono text-xs text-muted-foreground">{{ s.key }}</span> <span class="font-medium">{{ s.title }}</span></p>
          <p class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            {{ t('team.itemMeta', { weight: s.weight, due: date(s.due_date) }) }}
            <LabelChips v-if="s.label_ids.length" :labels="labelsOf(s.label_ids)" :max="2" />
          </p>
          <div v-if="writable" class="flex flex-wrap items-center gap-2">
            <Button v-for="p in s.people" :key="p.id" size="sm" variant="outline" class="rounded-full" data-testid="team-assign" @click="assign(s, p.id, p.name)">
              {{ t('team.assignTo', { name: p.name.split(' ')[0] }) }}
            </Button>
            <select
              class="h-8 rounded-md border bg-background px-2 text-sm text-muted-foreground"
              :aria-label="t('team.otherPerson')"
              data-testid="team-assign-other"
              @change="(e) => { const id = (e.target as HTMLSelectElement).value; const m = workers.find((w) => w.user.id === id); if (m) assign(s, m.user.id, m.user.name) }"
            >
              <option value="">{{ t('team.otherPerson') }}</option>
              <option v-for="w in workers" :key="w.user.id" :value="w.user.id">{{ w.user.name }}</option>
            </select>
          </div>
        </article>
      </section>
    </div>

    <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="team-schedule" data-testid="team-schedule">
      <div class="flex flex-col gap-1">
        <h2 id="team-schedule" class="text-base font-semibold">{{ t('team.schedule') }}</h2>
        <p class="text-sm text-muted-foreground">{{ t('team.scheduleHint') }}</p>
      </div>
      <PersonSchedule v-if="schedule" :rows="schedule" :today="today" :linkable="linkable" />
    </section>
  </div>
</template>

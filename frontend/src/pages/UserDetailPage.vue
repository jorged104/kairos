<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import {
  addRole,
  addTeam,
  changeStatus,
  getUser,
  listAttributes,
  listTeams,
  removeRole,
  removeTeam,
  setPassword,
  updateUser,
  type StatusAction,
} from '@/api/admin'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { flatten } from '@/features/teams/tree'
import AssignmentBadge from '@/features/users/AssignmentBadge.vue'
import AttributeField from '@/features/users/AttributeField.vue'
import { ROLES, type AttributeValue, type Role, type User } from '@/types/api'

const props = defineProps<{ id: string }>()
const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const { data: user, error: loadError } = useQuery({ queryKey: ['user', () => props.id], queryFn: () => getUser(props.id) })
const { data: attributes } = useQuery({ queryKey: ['attributes'], queryFn: listAttributes })
const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams })

const can = computed(() => user.value?.can)
const activeAttributes = computed(() => (attributes.value ?? []).filter((a) => !a.retired_at))
const retiredAttributes = computed(() => (attributes.value ?? []).filter((a) => a.retired_at))

const name = ref('')
const email = ref('')
const values = ref<Record<string, AttributeValue>>({})
watch(
  user,
  (u) => {
    if (!u) return
    name.value = u.name
    email.value = u.email ?? ''
    values.value = { ...(u.attributes ?? {}) }
  },
  { immediate: true },
)

const error = ref<unknown>(null)
const notice = ref('')

// Every change answers with the updated user, which replaces the cached one.
function run<A>(fn: (arg: A) => Promise<User | void>, done?: string) {
  return useMutation({
    mutationFn: fn,
    onMutate: () => {
      error.value = null
      notice.value = ''
    },
    onSuccess: async (u) => {
      if (u) qc.setQueryData(['user', props.id], u)
      else await qc.invalidateQueries({ queryKey: ['user', props.id] })
      await qc.invalidateQueries({ queryKey: ['users'] })
      if (done) notice.value = done
    },
    onError: (e) => (error.value = e),
  })
}

const save = run(() => updateUser(props.id, { name: name.value, email: email.value, attributes: values.value }), t('common.saved'))
const status = run((action: StatusAction) => changeStatus(props.id, action))
const roleAdd = run((role: Role) => addRole(props.id, role))
const roleRemove = run((role: Role) => removeRole(props.id, role))
const teamAdd = run((teamId: string) => addTeam(props.id, teamId))
const teamRemove = run((teamId: string) => removeTeam(props.id, teamId))
const newPassword = ref('')
const password = run(() => setPassword(props.id, newPassword.value), t('users.detail.passwordSet'))

const newRole = ref('')
const newTeam = ref('')
const availableTeams = computed(() =>
  flatten((teams.value ?? []).filter((tm) => !tm.archived_at && !user.value?.teams.some((ut) => ut.id === tm.id))),
)

const statusActions = computed<StatusAction[]>(() => {
  const u = user.value
  if (!u?.can) return []
  if (u.status === 'offboarded') return u.can.offboard ? ['restore'] : []
  const out: StatusAction[] = []
  if (u.can.set_status) out.push(u.status === 'active' ? 'deactivate' : 'reactivate')
  if (u.can.offboard) out.push('offboard')
  return out
})

function display(v: AttributeValue | undefined): string {
  if (v === null || v === undefined) return t('attributes.empty')
  if (typeof v === 'boolean') return v ? t('common.yes') : t('common.no')
  return String(v)
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <RouterLink to="/users" class="text-sm text-muted-foreground hover:underline">← {{ t('common.back') }}</RouterLink>

    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>

    <template v-if="user">
      <div class="flex flex-wrap items-center gap-3">
        <h1 class="font-heading text-xl font-semibold">{{ user.name }}</h1>
        <Badge :variant="user.status === 'active' ? 'outline' : 'secondary'" data-testid="user-status">
          {{ t(`status.${user.status}`) }}
        </Badge>
        <div class="ml-auto flex gap-2">
          <Button
            v-for="action in statusActions"
            :key="action"
            :variant="action === 'offboard' || action === 'deactivate' ? 'destructive' : 'outline'"
            size="sm"
            :disabled="status.isPending.value"
            @click="status.mutate(action)"
          >
            {{ t(`users.actions.${action}`) }}
          </Button>
        </div>
      </div>

      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>
      <Alert v-if="notice" role="status">
        <AlertDescription>{{ notice }}</AlertDescription>
      </Alert>

      <p v-if="!user.full_profile" class="text-sm text-muted-foreground">{{ t('users.detail.basicOnly') }}</p>

      <div class="grid gap-4 lg:grid-cols-2">
        <Card v-if="user.full_profile">
          <CardHeader>
            <CardTitle>{{ t('users.detail.profile') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <form class="flex flex-col gap-3" @submit.prevent="save.mutate(undefined)">
              <div class="flex flex-col gap-1.5">
                <Label for="name">{{ t('auth.fields.name') }}</Label>
                <Input id="name" v-model="name" :disabled="!can?.edit" required />
              </div>
              <div class="flex flex-col gap-1.5">
                <Label for="email">{{ t('auth.fields.email') }}</Label>
                <Input id="email" v-model="email" type="email" :disabled="!can?.edit" required />
              </div>
              <template v-if="activeAttributes.length">
                <h3 class="mt-2 font-medium">{{ t('users.detail.attributes') }}</h3>
                <div v-for="a in activeAttributes" :key="a.id" class="flex flex-col gap-1.5">
                  <Label :for="`attr-${a.id}`">{{ a.name }}{{ a.required ? ' *' : '' }}</Label>
                  <AttributeField :id="`attr-${a.id}`" v-model="values[a.id]" :attribute="a" :disabled="!can?.edit" />
                </div>
              </template>
              <div v-if="can?.edit" class="flex justify-end">
                <Button type="submit" :disabled="save.isPending.value">{{ t('common.save') }}</Button>
              </div>
            </form>
            <!-- Retired attributes: read-only history for administrators (RF-105). -->
            <div v-if="user.retired_attributes" class="mt-4 flex flex-col gap-1 text-sm">
              <h3 class="font-medium">{{ t('users.detail.retiredAttributes') }}</h3>
              <p v-for="a in retiredAttributes" :key="a.id">
                <span class="text-muted-foreground">{{ a.name }}:</span> {{ display(user.retired_attributes[a.id]) }}
              </p>
            </div>
          </CardContent>
        </Card>

        <div class="flex flex-col gap-4">
          <Card v-if="user.roles">
            <CardHeader>
              <CardTitle>{{ t('users.detail.roles') }}</CardTitle>
              <CardDescription>{{ t('users.detail.sourceHint') }}</CardDescription>
            </CardHeader>
            <CardContent class="flex flex-col gap-3">
              <div class="flex flex-wrap gap-2">
                <span v-for="r in user.roles" :key="r.role" class="inline-flex items-center gap-1">
                  <AssignmentBadge :label="t(`roles.${r.role}`)" :source="r.source" />
                  <Button
                    v-if="can?.assign_roles"
                    variant="ghost"
                    size="icon-xs"
                    :aria-label="t('common.remove')"
                    @click="roleRemove.mutate(r.role)"
                  >
                    <X />
                  </Button>
                </span>
              </div>
              <form v-if="can?.assign_roles" class="flex gap-2" @submit.prevent="newRole && roleAdd.mutate(newRole as Role)">
                <NativeSelect v-model="newRole" :aria-label="t('users.detail.addRole')">
                  <option value="">{{ t('users.detail.addRole') }}</option>
                  <option v-for="r in ROLES" :key="r" :value="r">{{ t(`roles.${r}`) }}</option>
                </NativeSelect>
                <Button type="submit" variant="outline" size="sm" :disabled="!newRole">{{ t('common.add') }}</Button>
              </form>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{{ t('users.detail.teams') }}</CardTitle>
            </CardHeader>
            <CardContent class="flex flex-col gap-3">
              <div class="flex flex-wrap gap-2">
                <span v-for="tm in user.teams" :key="tm.id" class="inline-flex items-center gap-1">
                  <AssignmentBadge :label="tm.name" :source="user.full_profile ? tm.source : undefined" />
                  <Button
                    v-if="can?.manage_teams && tm.source === 'manual'"
                    variant="ghost"
                    size="icon-xs"
                    :aria-label="t('common.remove')"
                    @click="teamRemove.mutate(tm.id)"
                  >
                    <X />
                  </Button>
                </span>
                <span v-if="user.teams.length === 0" class="text-sm text-muted-foreground">{{ t('common.none') }}</span>
              </div>
              <form v-if="can?.manage_teams" class="flex gap-2" @submit.prevent="newTeam && teamAdd.mutate(newTeam)">
                <NativeSelect v-model="newTeam" :aria-label="t('users.detail.addTeam')">
                  <option value="">{{ t('users.detail.addTeam') }}</option>
                  <option v-for="{ team, depth } in availableTeams" :key="team.id" :value="team.id">
                    {{ '  '.repeat(depth) + team.name }}
                  </option>
                </NativeSelect>
                <Button type="submit" variant="outline" size="sm" :disabled="!newTeam">{{ t('common.add') }}</Button>
              </form>
            </CardContent>
          </Card>

          <Card v-if="user.full_profile">
            <CardHeader>
              <CardTitle>{{ t('users.detail.password') }}</CardTitle>
              <CardDescription>
                {{ user.has_password ? t('users.detail.hasPassword') : t('users.detail.noPassword') }}
              </CardDescription>
            </CardHeader>
            <CardContent v-if="can?.set_password">
              <form class="flex gap-2" @submit.prevent="password.mutate(undefined)">
                <Input v-model="newPassword" type="password" autocomplete="new-password" :aria-label="t('auth.fields.newPassword')" required />
                <Button type="submit" variant="outline" :disabled="password.isPending.value">
                  {{ t('users.detail.setPassword') }}
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      </div>
    </template>
  </div>
</template>

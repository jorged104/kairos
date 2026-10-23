<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { createUser, listAttributes, listTeams } from '@/api/admin'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { flatten, ledScope } from '@/features/teams/tree'
import AttributeField from '@/features/users/AttributeField.vue'
import { useAuthStore } from '@/stores/auth'
import { ROLES, type AttributeValue, type Role } from '@/types/api'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const isAdmin = computed(() => auth.can('users:create:all'))
const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams })
const { data: attributes } = useQuery({ queryKey: ['attributes'], queryFn: listAttributes })

// A lead creates users only in the teams it leads (RF-67) and with a basic role (RF-64).
const teamOptions = computed(() => {
  const all = flatten((teams.value ?? []).filter((tm) => !tm.archived_at))
  if (isAdmin.value || !auth.me) return all
  const scope = ledScope(teams.value ?? [], auth.me.id)
  return all.filter(({ team }) => scope.has(team.id))
})
const roleOptions = computed<Role[]>(() => (isAdmin.value ? ROLES : ['member', 'viewer']))
const activeAttributes = computed(() => (attributes.value ?? []).filter((a) => !a.retired_at))

const name = ref('')
const email = ref('')
const role = ref<Role | ''>('')
const teamId = ref('')
const password = ref('')
const values = ref<Record<string, AttributeValue>>({})

const create = useMutation({
  mutationFn: () =>
    createUser({
      name: name.value,
      email: email.value,
      role: role.value || undefined,
      team_id: teamId.value || undefined,
      password: password.value || undefined,
      attributes: values.value,
    }),
  onSuccess: async (user) => {
    await qc.invalidateQueries({ queryKey: ['users'] })
    await router.push(`/users/${user.id}`)
  },
})
</script>

<template>
  <Card class="mx-auto max-w-2xl">
    <CardHeader>
      <CardTitle>{{ t('users.form.title') }}</CardTitle>
    </CardHeader>
    <CardContent>
      <form class="grid gap-3 sm:grid-cols-2" @submit.prevent="create.mutate()">
        <Alert v-if="create.error.value" variant="destructive" role="alert" class="sm:col-span-2">
          <AlertDescription>{{ errorMessage(create.error.value) }}</AlertDescription>
        </Alert>
        <div class="flex flex-col gap-1.5">
          <Label for="name">{{ t('auth.fields.name') }}</Label>
          <Input id="name" v-model="name" required />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="email">{{ t('auth.fields.email') }}</Label>
          <Input id="email" v-model="email" type="email" required />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="role">{{ t('users.form.role') }}</Label>
          <NativeSelect id="role" v-model="role">
            <option value="">{{ t('users.form.defaultRole') }}</option>
            <option v-for="r in roleOptions" :key="r" :value="r">{{ t(`roles.${r}`) }}</option>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="team">{{ t('users.form.team') }}</Label>
          <NativeSelect id="team" v-model="teamId" :required="!isAdmin">
            <option value="">{{ t('users.form.noTeam') }}</option>
            <option v-for="{ team, depth } in teamOptions" :key="team.id" :value="team.id">
              {{ '  '.repeat(depth) + team.name }}
            </option>
          </NativeSelect>
        </div>
        <div v-if="auth.can('passwords:set')" class="flex flex-col gap-1.5 sm:col-span-2">
          <Label for="password">{{ t('users.form.initialPassword') }}</Label>
          <Input id="password" v-model="password" type="password" autocomplete="new-password" />
          <p class="text-xs text-muted-foreground">{{ t('auth.passwordHint') }}</p>
        </div>
        <template v-if="activeAttributes.length">
          <h2 class="mt-2 font-medium sm:col-span-2">{{ t('users.form.attributes') }}</h2>
          <div v-for="a in activeAttributes" :key="a.id" class="flex flex-col gap-1.5">
            <Label :for="`attr-${a.id}`">{{ a.name }}{{ a.required ? ' *' : '' }}</Label>
            <AttributeField :id="`attr-${a.id}`" v-model="values[a.id]" :attribute="a" />
          </div>
        </template>
        <div class="flex justify-end gap-2 sm:col-span-2">
          <Button type="button" variant="ghost" @click="router.back()">{{ t('common.cancel') }}</Button>
          <Button type="submit" :disabled="create.isPending.value">{{ t('common.create') }}</Button>
        </div>
      </form>
    </CardContent>
  </Card>
</template>

<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { changeOwnPassword, fetchAuthConfig } from '@/api/auth'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import AssignmentBadge from '@/features/users/AssignmentBadge.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const { data: config } = useQuery({ queryKey: ['auth', 'config'], queryFn: fetchAuthConfig })

const current = ref('')
const next = ref('')
const error = ref<unknown>(null)
const done = ref(false)

async function submit() {
  error.value = null
  done.value = false
  try {
    await changeOwnPassword(current.value, next.value)
    current.value = next.value = ''
    done.value = true
  } catch (e) {
    error.value = e
  }
}
</script>

<template>
  <div v-if="auth.me" class="mx-auto flex max-w-2xl flex-col gap-4">
    <h1 class="font-heading text-xl font-semibold">{{ t('account.title') }}</h1>
    <Card>
      <CardHeader>
        <CardTitle>{{ auth.me.name }}</CardTitle>
        <CardDescription>
          {{ auth.me.email }}
          <template v-if="auth.me.auth_method">
            · {{ t('account.method', { method: t(`account.methods.${auth.me.auth_method}`) }) }}
          </template>
        </CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-3 text-sm">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-muted-foreground">{{ t('account.roles') }}:</span>
          <AssignmentBadge v-for="r in auth.me.roles" :key="r.role" :label="t(`roles.${r.role}`)" :source="r.source" />
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-muted-foreground">{{ t('account.teams') }}:</span>
          <AssignmentBadge v-for="tm in auth.me.teams" :key="tm.id" :label="tm.name" :source="tm.source" />
          <span v-if="auth.me.teams.length === 0">{{ t('common.none') }}</span>
        </div>
      </CardContent>
    </Card>

    <Card v-if="config?.local_enabled">
      <CardHeader>
        <CardTitle>{{ t('account.password.title') }}</CardTitle>
        <CardDescription v-if="!auth.me.has_password">{{ t('account.password.noLocal') }}</CardDescription>
      </CardHeader>
      <CardContent v-if="auth.me.has_password">
        <form class="flex flex-col gap-3" @submit.prevent="submit">
          <Alert v-if="error" variant="destructive" role="alert">
            <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
          </Alert>
          <Alert v-if="done" role="status">
            <AlertDescription>{{ t('account.password.done') }}</AlertDescription>
          </Alert>
          <div class="flex flex-col gap-1.5">
            <Label for="current">{{ t('auth.fields.currentPassword') }}</Label>
            <Input id="current" v-model="current" type="password" autocomplete="current-password" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="next">{{ t('auth.fields.newPassword') }}</Label>
            <Input id="next" v-model="next" type="password" autocomplete="new-password" required />
            <p class="text-xs text-muted-foreground">{{ t('auth.passwordHint') }}</p>
          </div>
          <div class="flex justify-end">
            <Button type="submit">{{ t('account.password.submit') }}</Button>
          </div>
        </form>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import { checkLink, resetPassword, type LinkInfo } from '@/api/accounts'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'

// Creating the password of an invitation or setting a new one (spec 019,
// RF-14, RF-16 to RF-18, RF-29, RF-30). The secret comes in the fragment
// (#t=…), which no server receives; it leaves the address bar at once.
const { t } = useI18n()
const router = useRouter()
const errorMessage = useErrorMessage()

const token = ref('')
const link = ref<LinkInfo | null>(null)
const invalid = ref(false)
const password = ref('')
const repeat = ref('')
const saving = ref(false)
const error = ref<unknown>(null)

onMounted(async () => {
  token.value = new URLSearchParams(location.hash.slice(1)).get('t') ?? ''
  history.replaceState(history.state, '', location.pathname)
  if (!token.value) {
    invalid.value = true
    return
  }
  try {
    link.value = await checkLink(token.value)
  } catch (e) {
    if (e instanceof ApiError && e.code === 'invalid_link') invalid.value = true
    else error.value = e
  }
})

async function submit() {
  error.value = null
  if (password.value !== repeat.value) {
    error.value = new ApiError('passwords_differ', 400)
    return
  }
  saving.value = true
  try {
    await resetPassword(token.value, password.value)
    await router.push({ name: 'login', query: { password: 'set' } })
  } catch (e) {
    if (e instanceof ApiError && e.code === 'invalid_link') invalid.value = true
    else error.value = e
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <main class="mx-auto flex min-h-svh max-w-sm flex-col justify-center p-4">
    <Card>
      <CardHeader>
        <CardTitle>{{ link?.purpose === 'invite' ? t('accounts.password.inviteTitle') : t('accounts.password.resetTitle') }}</CardTitle>
        <CardDescription v-if="link">{{ t('accounts.password.for', { name: link.name, email: link.email }) }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <div v-if="invalid" class="flex flex-col gap-3" data-testid="link-invalid">
          <Alert variant="destructive" role="alert">
            <AlertDescription>{{ t('accounts.password.invalid') }}</AlertDescription>
          </Alert>
          <RouterLink to="/forgot-password" class="text-sm underline">{{ t('accounts.password.askAgain') }}</RouterLink>
        </div>
        <template v-else-if="link">
          <Alert v-if="error" variant="destructive" role="alert">
            <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
          </Alert>
          <form class="flex flex-col gap-3" data-testid="password-form" @submit.prevent="submit">
            <div class="flex flex-col gap-1.5">
              <Label for="password">{{ t('auth.fields.newPassword') }}</Label>
              <Input id="password" v-model="password" type="password" autocomplete="new-password" required data-testid="password-new" />
              <p class="text-xs text-muted-foreground">{{ t('auth.passwordHint') }}</p>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="repeat">{{ t('accounts.password.repeat') }}</Label>
              <Input id="repeat" v-model="repeat" type="password" autocomplete="new-password" required data-testid="password-repeat" />
            </div>
            <Button type="submit" :disabled="saving" data-testid="password-save">{{ t('accounts.password.save') }}</Button>
          </form>
        </template>
        <Alert v-else-if="error" variant="destructive" role="alert">
          <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
        </Alert>
      </CardContent>
    </Card>
  </main>
</template>

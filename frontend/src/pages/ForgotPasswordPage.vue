<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { forgotPassword } from '@/api/accounts'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'

// «¿Olvidaste tu contraseña?»: the same answer whether the address has an
// account or not (spec 019, RF-22 to RF-27).
const { t } = useI18n()
const errorMessage = useErrorMessage()
const email = ref('')
const sending = ref(false)
const done = ref(false)
const error = ref<unknown>(null)

async function submit() {
  sending.value = true
  error.value = null
  try {
    await forgotPassword(email.value.trim())
    done.value = true
  } catch (e) {
    error.value = e
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <main class="mx-auto flex min-h-svh max-w-sm flex-col justify-center p-4">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('accounts.forgot.title') }}</CardTitle>
        <CardDescription>{{ t('accounts.forgot.description') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <Alert v-if="done" role="status" data-testid="forgot-done">
          <AlertDescription>{{ t('accounts.forgot.done') }}</AlertDescription>
        </Alert>
        <Alert v-if="error" variant="destructive" role="alert">
          <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
        </Alert>
        <form v-if="!done" class="flex flex-col gap-3" data-testid="forgot-form" @submit.prevent="submit">
          <div class="flex flex-col gap-1.5">
            <Label for="email">{{ t('auth.fields.email') }}</Label>
            <Input id="email" v-model="email" type="email" autocomplete="username" required data-testid="forgot-email" />
          </div>
          <Button type="submit" :disabled="sending">{{ t('accounts.forgot.submit') }}</Button>
        </form>
        <RouterLink to="/login" class="text-center text-sm text-muted-foreground underline">{{ t('accounts.backToLogin') }}</RouterLink>
      </CardContent>
    </Card>
  </main>
</template>

<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { fetchAuthConfig, login, ssoStartUrl } from '@/api/auth'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { safeRedirect } from '@/router'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const { data: config } = useQuery({ queryKey: ['auth', 'config'], queryFn: fetchAuthConfig })

const email = ref('')
const password = ref('')
const submitting = ref(false)
// Errors come from the API or from the SSO callback (?error=code).
const error = ref<unknown>(typeof route.query.error === 'string' ? route.query.error : null)

const redirect = computed(() => safeRedirect(route.query.redirect))

async function submit() {
  submitting.value = true
  error.value = null
  try {
    auth.set(await login(email.value, password.value))
    await router.push(redirect.value) // RF-122
  } catch (e) {
    error.value = e
  } finally {
    submitting.value = false
  }
}

function sso() {
  window.location.assign(ssoStartUrl(redirect.value))
}
</script>

<template>
  <main class="mx-auto flex min-h-svh max-w-sm flex-col justify-center p-4">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('auth.login.title') }}</CardTitle>
        <CardDescription>{{ t('auth.login.description') }}</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <Alert v-if="error" variant="destructive" role="alert">
          <AlertDescription data-testid="login-error">{{ errorMessage(error) }}</AlertDescription>
        </Alert>

        <form v-if="config?.local_enabled" class="flex flex-col gap-3" data-testid="local-login" @submit.prevent="submit">
          <div class="flex flex-col gap-1.5">
            <Label for="email">{{ t('auth.fields.email') }}</Label>
            <Input id="email" v-model="email" type="email" autocomplete="username" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="password">{{ t('auth.fields.password') }}</Label>
            <Input id="password" v-model="password" type="password" autocomplete="current-password" required />
          </div>
          <Button type="submit" :disabled="submitting">{{ t('auth.login.submit') }}</Button>
        </form>

        <Separator v-if="config?.local_enabled && config?.oidc_enabled" />

        <Button v-if="config?.oidc_enabled" variant="outline" data-testid="sso-login" @click="sso">
          {{ t('auth.login.sso') }}
        </Button>

        <p v-if="config?.registration_enabled" class="text-center text-sm text-muted-foreground" data-testid="register-link">
          {{ t('auth.login.noAccount') }}
          <RouterLink :to="{ name: 'register', query: route.query.redirect ? { redirect } : {} }" class="underline">
            {{ t('auth.login.register') }}
          </RouterLink>
        </p>
      </CardContent>
    </Card>
  </main>
</template>

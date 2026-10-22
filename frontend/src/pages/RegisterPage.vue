<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { register } from '@/api/auth'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { safeRedirect } from '@/router'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const name = ref('')
const email = ref('')
const password = ref('')
const submitting = ref(false)
const error = ref<unknown>(null)

async function submit() {
  submitting.value = true
  error.value = null
  try {
    auth.set(await register(name.value, email.value, password.value))
    await router.push(safeRedirect(route.query.redirect))
  } catch (e) {
    error.value = e
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <main class="mx-auto flex min-h-svh max-w-sm flex-col justify-center p-4">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('auth.register.title') }}</CardTitle>
        <CardDescription>{{ t('auth.register.description') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="flex flex-col gap-3" @submit.prevent="submit">
          <Alert v-if="error" variant="destructive" role="alert">
            <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
          </Alert>
          <div class="flex flex-col gap-1.5">
            <Label for="name">{{ t('auth.fields.name') }}</Label>
            <Input id="name" v-model="name" autocomplete="name" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="email">{{ t('auth.fields.email') }}</Label>
            <Input id="email" v-model="email" type="email" autocomplete="email" required />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="password">{{ t('auth.fields.password') }}</Label>
            <Input id="password" v-model="password" type="password" autocomplete="new-password" required />
            <p class="text-xs text-muted-foreground">{{ t('auth.passwordHint') }}</p>
          </div>
          <Button type="submit" :disabled="submitting">{{ t('auth.register.submit') }}</Button>
          <RouterLink :to="{ name: 'login', query: route.query }" class="text-center text-sm underline">
            {{ t('auth.register.backToLogin') }}
          </RouterLink>
        </form>
      </CardContent>
    </Card>
  </main>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { saveMail, testMail, type SystemSettings } from '@/api/system'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The outgoing mail through the organization's SMTP server, and a test mail
// to the administrator that says where it failed (spec 019, RF-1 to RF-9).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const fromSettings = () => {
  const m = props.settings.mail
  return { enabled: m.enabled, host: m.host, port: m.port, security: m.security, username: m.username, password: '', from_email: m.from_email, from_name: m.from_name }
}
const form = ref(fromSettings())
watch(() => props.settings.mail, () => (form.value = fromSettings()))
const error = ref<unknown>(null)
const saved = ref(false)
const busy = ref(false)
const tested = ref<'ok' | string | null>(null)

async function save() {
  error.value = null
  saved.value = false
  tested.value = null
  busy.value = true
  try {
    emit('saved', await saveMail({ ...form.value, port: Number(form.value.port) }))
    saved.value = true
  } catch (e) {
    error.value = e
  } finally {
    busy.value = false
  }
}

async function test() {
  tested.value = null
  error.value = null
  busy.value = true
  try {
    await testMail()
    tested.value = 'ok'
  } catch (e) {
    if (e instanceof ApiError && e.code === 'mail_test_failed') tested.value = e.details.step ?? 'send'
    else error.value = e
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="flex max-w-xl flex-col gap-4" data-testid="mail-settings" @submit.prevent="save">
    <Alert v-if="error" variant="destructive" role="alert" data-testid="mail-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-if="saved" role="status">
      <AlertDescription>{{ t('common.saved') }}</AlertDescription>
    </Alert>
    <Alert v-if="tested === 'ok'" role="status" data-testid="mail-test-ok">
      <AlertDescription>{{ t('system.mail.testOk') }}</AlertDescription>
    </Alert>
    <Alert v-else-if="tested" variant="destructive" role="alert" data-testid="mail-test-failed">
      <AlertDescription>{{ t('system.mail.testFailed', { step: t(`system.mail.steps.${tested}`) }) }}</AlertDescription>
    </Alert>
    <label class="flex items-start gap-2 text-sm">
      <input v-model="form.enabled" type="checkbox" class="mt-1" data-testid="mail-enabled" />
      <span>
        <span class="font-medium">{{ t('system.mail.enabled') }}</span>
        <span class="block text-xs text-muted-foreground">{{ t('system.mail.enabledHint') }}</span>
      </span>
    </label>
    <div class="grid gap-3 sm:grid-cols-[1fr_7rem]">
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.mail.host') }}
        <Input v-model="form.host" placeholder="smtp.example.com" data-testid="mail-host" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.mail.port') }}
        <Input v-model="form.port" type="number" min="1" max="65535" required data-testid="mail-port" />
      </label>
    </div>
    <label class="flex flex-col gap-1 text-sm">
      {{ t('system.mail.security') }}
      <NativeSelect v-model="form.security" data-testid="mail-security">
        <option v-for="s in ['starttls', 'tls', 'none'] as const" :key="s" :value="s">{{ t(`system.mail.securities.${s}`) }}</option>
      </NativeSelect>
    </label>
    <div class="grid gap-3 sm:grid-cols-2">
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.mail.username') }}
        <Input v-model="form.username" autocomplete="off" data-testid="mail-username" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.mail.password') }}
        <Input
          v-model="form.password"
          type="password"
          autocomplete="new-password"
          :placeholder="settings.mail.password_set ? t('system.secretKept') : ''"
          data-testid="mail-password"
        />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.mail.fromEmail') }}
        <Input v-model="form.from_email" type="email" placeholder="kairos@example.com" data-testid="mail-from" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.mail.fromName') }}
        <Input v-model="form.from_name" maxlength="100" data-testid="mail-from-name" />
      </label>
    </div>
    <div class="flex flex-wrap gap-2">
      <Button type="submit" :disabled="busy" data-testid="mail-save">{{ t('common.save') }}</Button>
      <Button type="button" variant="outline" :disabled="busy || !settings.mail.host" data-testid="mail-test" @click="test">{{ t('system.mail.test') }}</Button>
    </div>
    <p class="text-xs text-muted-foreground">{{ t('system.mail.testHint') }}</p>
  </form>
</template>

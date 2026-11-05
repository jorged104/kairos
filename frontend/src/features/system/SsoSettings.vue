<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveSSO, ssoTestUrl, type SystemSettings } from '@/api/system'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The SSO connection and its test login (spec 015, RF-24 to RF-32).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t, d } = useI18n()
const errorMessage = useErrorMessage()

const fromSettings = () => ({
  issuer: props.settings.sso.issuer,
  discovery_url: props.settings.sso.discovery_url,
  client_id: props.settings.sso.client_id,
  client_secret: '',
})
const form = ref(fromSettings())
watch(() => props.settings.sso, () => (form.value = fromSettings()))
// While SSO is the only way in, it cannot change (RF-32).
const inUse = computed(() => props.settings.access.mode === 'oidc')
const error = ref<unknown>(null)
const saved = ref(false)
const copied = ref(false)
const busy = ref(false)

async function save() {
  error.value = null
  saved.value = false
  busy.value = true
  try {
    emit('saved', await saveSSO(form.value))
    saved.value = true
  } catch (e) {
    error.value = e
  } finally {
    busy.value = false
  }
}
async function copy() {
  try {
    await navigator.clipboard.writeText(props.settings.sso.redirect_url)
    copied.value = true
  } catch {
    // Without the clipboard the field can still be selected by hand.
  }
}
const test = () => window.location.assign(ssoTestUrl)
const testResult = computed(() => props.settings.sso.test)
</script>

<template>
  <div class="flex max-w-xl flex-col gap-4" data-testid="sso-settings">
    <Alert v-if="inUse" role="status" data-testid="sso-in-use">
      <AlertDescription>{{ t('system.sso.inUse') }}</AlertDescription>
    </Alert>
    <label class="flex flex-col gap-1 text-sm">
      {{ t('system.sso.redirect') }}
      <span class="flex gap-2">
        <Input :model-value="settings.sso.redirect_url" readonly class="font-mono text-xs" data-testid="sso-redirect" />
        <Button type="button" variant="outline" @click="copy">{{ copied ? t('system.sso.copied') : t('system.sso.copy') }}</Button>
      </span>
      <span class="text-xs text-muted-foreground">{{ t('system.sso.redirectHint') }}</span>
    </label>
    <form class="flex flex-col gap-3" @submit.prevent="save">
      <Alert v-if="error" variant="destructive" role="alert" data-testid="sso-error">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>
      <Alert v-if="saved" role="status">
        <AlertDescription>{{ t('system.sso.saved') }}</AlertDescription>
      </Alert>
      <fieldset :disabled="inUse" class="flex flex-col gap-3">
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.sso.issuer') }}
          <Input v-model="form.issuer" type="url" required placeholder="https://idp.example.com/realms/kairos" data-testid="sso-issuer" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.sso.discovery') }}
          <Input v-model="form.discovery_url" type="url" data-testid="sso-discovery" />
          <span class="text-xs text-muted-foreground">{{ t('system.sso.discoveryHint') }}</span>
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.sso.clientId') }}
          <Input v-model="form.client_id" required data-testid="sso-client-id" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.sso.clientSecret') }}
          <Input
            v-model="form.client_secret"
            type="password"
            autocomplete="new-password"
            :required="!settings.sso.client_secret_set"
            :placeholder="settings.sso.client_secret_set ? t('system.secretKept') : ''"
            data-testid="sso-client-secret"
          />
        </label>
        <div>
          <Button type="submit" :disabled="busy" data-testid="sso-save">{{ t('common.save') }}</Button>
        </div>
      </fieldset>
    </form>
    <div class="flex flex-col gap-2 rounded-lg border p-3" data-testid="sso-test">
      <p class="text-sm font-medium">{{ t('system.sso.test') }}</p>
      <p class="text-xs text-muted-foreground">{{ t('system.sso.testHint') }}</p>
      <p v-if="testResult?.ok" class="text-sm" data-testid="sso-test-ok">
        {{ t('system.sso.testOk', { who: testResult.name || testResult.email, email: testResult.email, at: d(new Date(testResult.at), 'long') }) }}
        <span v-if="!settings.sso.tested" class="block text-xs text-destructive">{{ t('system.sso.testStale') }}</span>
      </p>
      <p v-else-if="testResult" class="text-sm text-destructive" data-testid="sso-test-failed">{{ t('system.sso.testFailed', { error: testResult.error }) }}</p>
      <div>
        <Button type="button" variant="outline" :disabled="!settings.sso.configured" data-testid="sso-test-start" @click="test">{{ t('system.sso.testStart') }}</Button>
      </div>
    </div>
  </div>
</template>

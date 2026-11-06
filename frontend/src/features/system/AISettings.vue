<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { saveAI, testAI, type SystemSettings } from '@/api/system'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The AI service of the reports, compatible with OpenAI's API, and a test
// that says where it failed (spec 020, RF-1 to RF-14).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const fromSettings = () => {
  const a = props.settings.ai
  return { enabled: a.enabled, base_url: a.base_url, model: a.model, api_key: '' }
}
const form = ref(fromSettings())
watch(() => props.settings.ai, () => (form.value = fromSettings()))
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
    emit('saved', await saveAI({ ...form.value }))
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
    await testAI()
    tested.value = 'ok'
  } catch (e) {
    if (e instanceof ApiError && e.code === 'ai_test_failed') tested.value = e.details.step ?? 'connect'
    else error.value = e
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="flex max-w-xl flex-col gap-4" data-testid="ai-settings" @submit.prevent="save">
    <Alert role="note" data-testid="ai-privacy">
      <AlertDescription>{{ t('system.ai.privacy') }}</AlertDescription>
    </Alert>
    <Alert v-if="!settings.ai.agent_available" variant="destructive" role="status" data-testid="ai-no-agent">
      <AlertDescription>{{ t('system.ai.noAgent') }}</AlertDescription>
    </Alert>
    <Alert v-if="error" variant="destructive" role="alert" data-testid="ai-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-if="saved" role="status">
      <AlertDescription>{{ t('common.saved') }}</AlertDescription>
    </Alert>
    <Alert v-if="tested === 'ok'" role="status" data-testid="ai-test-ok">
      <AlertDescription>{{ t('system.ai.testOk') }}</AlertDescription>
    </Alert>
    <Alert v-else-if="tested" variant="destructive" role="alert" data-testid="ai-test-failed">
      <AlertDescription>{{ t('system.ai.testFailed', { step: t(`system.ai.steps.${tested}`) }) }}</AlertDescription>
    </Alert>
    <label class="flex items-start gap-2 text-sm">
      <input v-model="form.enabled" type="checkbox" class="mt-1" data-testid="ai-enabled" />
      <span>
        <span class="font-medium">{{ t('system.ai.enabled') }}</span>
        <span class="block text-xs text-muted-foreground">{{ t('system.ai.enabledHint') }}</span>
      </span>
    </label>
    <label class="flex flex-col gap-1 text-sm">
      {{ t('system.ai.baseUrl') }}
      <Input v-model="form.base_url" type="url" placeholder="https://api.example.com/v1" data-testid="ai-base-url" />
      <span class="text-xs text-muted-foreground">{{ t('system.ai.baseUrlHint') }}</span>
    </label>
    <div class="grid gap-3 sm:grid-cols-2">
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.ai.model') }}
        <Input v-model="form.model" maxlength="200" data-testid="ai-model" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('system.ai.apiKey') }}
        <Input
          v-model="form.api_key"
          type="password"
          autocomplete="new-password"
          :placeholder="settings.ai.api_key_set ? t('system.secretKept') : t('system.ai.apiKeyOptional')"
          data-testid="ai-api-key"
        />
      </label>
    </div>
    <div class="flex flex-wrap gap-2">
      <Button type="submit" :disabled="busy" data-testid="ai-save">{{ t('common.save') }}</Button>
      <Button type="button" variant="outline" :disabled="busy || !settings.ai.base_url || !settings.ai.model" data-testid="ai-test" @click="test">
        {{ t('system.ai.test') }}
      </Button>
    </div>
    <p class="text-xs text-muted-foreground">{{ t('system.ai.testHint') }}</p>
  </form>
</template>

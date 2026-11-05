<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveStorage, type SystemSettings } from '@/api/system'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// Where the files go: the database or an S3, checked before saving and fixed
// once there are files (spec 015, RF-33 to RF-38).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const fromSettings = () => {
  const s = props.settings.storage
  return { kind: s.kind, endpoint: s.endpoint, bucket: s.bucket, region: s.region, access_key: s.access_key, secret_key: '', use_ssl: s.use_ssl }
}
const form = ref(fromSettings())
watch(() => props.settings.storage, () => (form.value = fromSettings()))
const error = ref<unknown>(null)
const saved = ref(false)
const busy = ref(false)

async function save() {
  error.value = null
  saved.value = false
  busy.value = true
  try {
    emit('saved', await saveStorage(form.value))
    saved.value = true
  } catch (e) {
    error.value = e
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="flex max-w-xl flex-col gap-4" data-testid="storage-settings" @submit.prevent="save">
    <Alert v-if="settings.storage.locked" role="status" data-testid="storage-locked">
      <AlertDescription>{{ t('system.storage.locked', { n: settings.storage.files_count }, settings.storage.files_count) }}</AlertDescription>
    </Alert>
    <Alert v-if="error" variant="destructive" role="alert" data-testid="storage-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-if="saved" role="status">
      <AlertDescription>{{ t('system.storage.saved') }}</AlertDescription>
    </Alert>
    <fieldset class="flex flex-col gap-2" :disabled="settings.storage.locked">
      <legend class="mb-1 text-sm font-medium">{{ t('system.storage.kind') }}</legend>
      <label v-for="k in ['db', 's3'] as const" :key="k" class="flex items-center gap-2 text-sm">
        <input v-model="form.kind" type="radio" name="kind" :value="k" :data-testid="`kind-${k}`" />
        {{ t(`system.storage.kinds.${k}`) }}
      </label>
    </fieldset>
    <template v-if="form.kind === 's3'">
      <div class="grid gap-3 sm:grid-cols-2">
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.storage.endpoint') }}
          <Input v-model="form.endpoint" required placeholder="s3.example.com:9000" :disabled="settings.storage.locked" data-testid="s3-endpoint" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.storage.bucket') }}
          <Input v-model="form.bucket" required :disabled="settings.storage.locked" data-testid="s3-bucket" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.storage.region') }}
          <Input v-model="form.region" data-testid="s3-region" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('system.storage.accessKey') }}
          <Input v-model="form.access_key" required autocomplete="off" data-testid="s3-access-key" />
        </label>
        <label class="flex flex-col gap-1 text-sm sm:col-span-2">
          {{ t('system.storage.secretKey') }}
          <Input
            v-model="form.secret_key"
            type="password"
            autocomplete="new-password"
            :required="!settings.storage.secret_key_set"
            :placeholder="settings.storage.secret_key_set ? t('system.secretKept') : ''"
            data-testid="s3-secret-key"
          />
        </label>
      </div>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="form.use_ssl" type="checkbox" data-testid="s3-ssl" />
        {{ t('system.storage.useSsl') }}
      </label>
      <p class="text-xs text-muted-foreground">{{ t('system.storage.checkHint') }}</p>
    </template>
    <div>
      <Button type="submit" :disabled="busy" data-testid="storage-save">{{ t('common.save') }}</Button>
    </div>
  </form>
</template>

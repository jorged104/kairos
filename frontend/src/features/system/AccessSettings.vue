<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { saveAccess, type AuthMode, type SystemSettings } from '@/api/system'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'

// Access mode, open registration and default role (spec 015, RF-11 to RF-23).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()

const form = ref({ ...props.settings.access })
watch(
  () => props.settings.access,
  (a) => (form.value = { ...a }),
)
const modes: AuthMode[] = ['local', 'both', 'oidc']
const error = ref<unknown>(null)
const saved = ref(false)
// Without emergency access, only SSO needs an explicit «yes» (RF-23).
const needsAccept = ref(false)
const accept = ref(false)

async function save() {
  error.value = null
  saved.value = false
  try {
    emit('saved', await saveAccess({ ...form.value, accept_no_emergency: accept.value }))
    saved.value = true
    needsAccept.value = accept.value = false
  } catch (e) {
    if (e instanceof ApiError && e.code === 'no_emergency_access') needsAccept.value = true
    else error.value = e
  }
}
</script>

<template>
  <form class="flex max-w-xl flex-col gap-4" data-testid="access-settings" @submit.prevent="save">
    <Alert v-if="error" variant="destructive" role="alert" data-testid="access-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-if="saved" role="status">
      <AlertDescription>{{ t('common.saved') }}</AlertDescription>
    </Alert>
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 text-sm font-medium">{{ t('system.access.mode') }}</legend>
      <label v-for="m in modes" :key="m" class="flex items-start gap-2 text-sm">
        <input v-model="form.mode" type="radio" name="mode" :value="m" class="mt-1" :data-testid="`mode-${m}`" />
        <span>
          <span class="font-medium">{{ t(`system.access.modes.${m}`) }}</span>
          <span class="block text-xs text-muted-foreground">{{ t(`system.access.modeHints.${m}`) }}</span>
        </span>
      </label>
      <p v-if="form.mode === 'oidc' && settings.access.mode !== 'oidc' && !settings.sso.tested" class="text-xs text-destructive" data-testid="mode-needs-test">
        {{ t('system.access.needsTest') }}
      </p>
    </fieldset>
    <div v-if="needsAccept" class="flex flex-col gap-2 rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm" data-testid="no-emergency">
      <p>{{ t('system.access.noEmergency') }}</p>
      <label class="flex items-center gap-2">
        <input v-model="accept" type="checkbox" data-testid="accept-no-emergency" />
        {{ t('system.access.acceptNoEmergency') }}
      </label>
    </div>
    <label class="flex items-center gap-2 text-sm" :class="form.mode === 'oidc' && 'opacity-60'">
      <input v-model="form.registration_enabled" type="checkbox" :disabled="form.mode === 'oidc'" data-testid="registration" />
      {{ t('system.access.registration') }}
    </label>
    <label class="flex max-w-xs flex-col gap-1 text-sm">
      {{ t('system.access.defaultRole') }}
      <NativeSelect v-model="form.default_role" data-testid="default-role">
        <option value="">{{ t('system.access.noRole') }}</option>
        <option v-for="r in ['viewer', 'member', 'lead']" :key="r" :value="r">{{ t(`roles.${r}`) }}</option>
      </NativeSelect>
    </label>
    <div>
      <Button type="submit" :disabled="needsAccept && !accept" data-testid="access-save">{{ t('common.save') }}</Button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { getSystemSettings, type SystemSettings } from '@/api/system'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import AccessSettings from '@/features/system/AccessSettings.vue'
import AppearanceSettings from '@/features/system/AppearanceSettings.vue'
import McpSettings from '@/features/system/McpSettings.vue'
import SsoSettings from '@/features/system/SsoSettings.vue'
import StorageSettings from '@/features/system/StorageSettings.vue'

// Administration of the system: access, SSO, storage and appearance
// (spec 015). The tab lives in the URL, so the SSO test comes back to it.
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const client = useQueryClient()
const errorMessage = useErrorMessage()

const tabs = ['access', 'sso', 'storage', 'appearance', 'mcp'] as const
type Tab = (typeof tabs)[number]
const tab = computed<Tab>(() => (tabs as readonly string[]).includes(String(route.query.tab)) ? (route.query.tab as Tab) : 'access')
const select = (t: Tab) => router.replace({ query: { ...route.query, tab: t } })

const { data, error } = useQuery({ queryKey: ['system', 'settings'], queryFn: getSystemSettings })
function saved(s: SystemSettings) {
  client.setQueryData(['system', 'settings'], s)
  void client.invalidateQueries({ queryKey: ['auth', 'config'] })
}
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="system-page">
    <h1 class="font-heading text-xl font-semibold">{{ t('system.title') }}</h1>
    <div class="flex flex-wrap gap-1 border-b" role="tablist" :aria-label="t('system.title')">
      <button
        v-for="x in tabs"
        :key="x"
        type="button"
        role="tab"
        class="-mb-px border-b-2 px-3 py-2 text-sm"
        :class="tab === x ? 'border-primary font-medium text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
        :aria-selected="tab === x"
        :data-testid="`tab-${x}`"
        @click="select(x)"
      >
        {{ t(`system.tabs.${x}`) }}
      </button>
    </div>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <template v-if="data">
      <AccessSettings v-if="tab === 'access'" :settings="data" @saved="saved" />
      <SsoSettings v-else-if="tab === 'sso'" :settings="data" @saved="saved" />
      <StorageSettings v-else-if="tab === 'storage'" :settings="data" @saved="saved" />
      <McpSettings v-else-if="tab === 'mcp'" :settings="data" @saved="saved" />
      <AppearanceSettings v-else :settings="data" @saved="saved" />
    </template>
  </div>
</template>

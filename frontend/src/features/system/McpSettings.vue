<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getAllMcpConnections, revokeMcpConnection } from '@/api/mcp'
import { saveMcp, type SystemSettings } from '@/api/system'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import McpConnectionList from '@/features/mcp/McpConnectionList.vue'

// The switch of the AI connections and everybody's connections (spec 016,
// RF-2 to RF-7, RF-23, RF-24).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t } = useI18n()
const client = useQueryClient()
const errorMessage = useErrorMessage()
const error = ref<unknown>(null)

async function toggle(enabled: boolean) {
  error.value = null
  try {
    emit('saved', await saveMcp(enabled))
    void client.invalidateQueries({ queryKey: ['mcp'] })
  } catch (e) {
    error.value = e
  }
}

const { data, error: loadError } = useQuery({ queryKey: ['mcp', 'all'], queryFn: getAllMcpConnections })
const revoke = useMutation({
  mutationFn: revokeMcpConnection,
  onSuccess: () => client.invalidateQueries({ queryKey: ['mcp'] }),
})
</script>

<template>
  <div class="flex max-w-3xl flex-col gap-4" data-testid="mcp-settings">
    <Alert v-if="error || loadError || revoke.error.value" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error ?? loadError ?? revoke.error.value) }}</AlertDescription>
    </Alert>
    <label class="flex items-start gap-2 text-sm">
      <input
        type="checkbox"
        class="mt-1"
        :checked="props.settings.mcp.enabled"
        data-testid="mcp-enabled"
        @change="toggle(($event.target as HTMLInputElement).checked)"
      />
      <span>
        <span class="font-medium">{{ t('system.mcp.enabled') }}</span>
        <span class="block text-xs text-muted-foreground">{{ t('system.mcp.enabledHint') }}</span>
      </span>
    </label>
    <section class="flex flex-col gap-2">
      <h2 class="text-sm font-medium">{{ t('system.mcp.connections') }}</h2>
      <McpConnectionList v-if="data" :items="data.items" owners @revoke="revoke.mutate($event)" />
    </section>
  </div>
</template>

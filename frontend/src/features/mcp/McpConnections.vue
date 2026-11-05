<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Check, Copy } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createMcpConnection, getMyMcpConnections, MCP_DAYS, revokeMyMcpConnection, type McpCreated } from '@/api/mcp'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import McpConnectionList from './McpConnectionList.vue'

// The AI connections of the person: create one, copy its URL once, revoke
// (spec 016, RF-8 to RF-20).
const { t } = useI18n()
const client = useQueryClient()
const errorMessage = useErrorMessage()

const { data, error: loadError } = useQuery({ queryKey: ['mcp', 'mine'], queryFn: getMyMcpConnections })
const full = computed(() => !!data.value && data.value.items.length >= data.value.max)

const name = ref('')
const days = ref('90')
const created = ref<McpCreated | null>(null)
const copied = ref(false)

const create = useMutation({
  mutationFn: () => createMcpConnection({ name: name.value.trim(), days: Number(days.value) as (typeof MCP_DAYS)[number] }),
  onSuccess: (c) => {
    created.value = c
    copied.value = false
    name.value = ''
    void client.invalidateQueries({ queryKey: ['mcp'] })
  },
})
const revoke = useMutation({
  mutationFn: revokeMyMcpConnection,
  onSuccess: () => client.invalidateQueries({ queryKey: ['mcp'] }),
})

async function copy() {
  if (!created.value) return
  try {
    await navigator.clipboard.writeText(created.value.url)
    copied.value = true
  } catch {
    // Without clipboard access the URL stays selectable in the field.
  }
}
</script>

<template>
  <Card data-testid="mcp-connections">
    <CardHeader>
      <CardTitle>{{ t('mcp.title') }}</CardTitle>
      <CardDescription>{{ t('mcp.description') }}</CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-4 text-sm">
      <Alert v-if="loadError" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
      </Alert>
      <template v-if="data">
        <Alert v-if="!data.enabled" role="status" data-testid="mcp-disabled">
          <AlertDescription>{{ t('mcp.disabled') }}</AlertDescription>
        </Alert>
        <ol class="list-decimal space-y-1 pl-5 text-muted-foreground" data-testid="mcp-steps">
          <li>{{ t('mcp.steps.create') }}</li>
          <li>{{ t('mcp.steps.copy') }}</li>
          <li>{{ t('mcp.steps.paste') }}</li>
        </ol>

        <div v-if="created" class="flex flex-col gap-2 rounded-lg border border-primary/40 p-3" data-testid="mcp-created">
          <p class="font-medium">{{ t('mcp.created', { name: created.name }) }}</p>
          <div class="flex gap-2">
            <Input :model-value="created.url" readonly class="font-mono text-xs" data-testid="mcp-url" @focus="($event.target as HTMLInputElement).select()" />
            <Button type="button" variant="outline" size="sm" data-testid="mcp-copy" @click="copy">
              <Check v-if="copied" class="size-4" aria-hidden="true" />
              <Copy v-else class="size-4" aria-hidden="true" />
              {{ copied ? t('mcp.copied') : t('mcp.copy') }}
            </Button>
          </div>
          <p class="text-xs text-destructive" data-testid="mcp-warning">{{ t('mcp.warning') }}</p>
          <div>
            <Button type="button" size="sm" variant="ghost" data-testid="mcp-done" @click="created = null">{{ t('mcp.done') }}</Button>
          </div>
        </div>

        <form v-if="data.enabled" class="flex flex-wrap items-end gap-2" data-testid="mcp-form" @submit.prevent="create.mutate()">
          <label class="flex min-w-48 flex-1 flex-col gap-1">
            {{ t('mcp.name') }}
            <Input v-model="name" required maxlength="100" :placeholder="t('mcp.namePlaceholder')" data-testid="mcp-name" />
          </label>
          <label class="flex flex-col gap-1">
            {{ t('mcp.expires') }}
            <NativeSelect v-model="days" class="w-36" data-testid="mcp-days">
              <option v-for="d in MCP_DAYS" :key="d" :value="String(d)">{{ t('mcp.days', { n: d }) }}</option>
            </NativeSelect>
          </label>
          <Button type="submit" :disabled="create.isPending.value || full || !name.trim()" data-testid="mcp-create">{{ t('mcp.create') }}</Button>
        </form>
        <p v-if="full" class="text-xs text-muted-foreground">{{ t('mcp.full', { n: data.max }) }}</p>
        <Alert v-if="create.error.value || revoke.error.value" variant="destructive" role="alert">
          <AlertDescription>{{ errorMessage(create.error.value ?? revoke.error.value) }}</AlertDescription>
        </Alert>

        <McpConnectionList :items="data.items" @revoke="revoke.mutate($event)" />
      </template>
    </CardContent>
  </Card>
</template>

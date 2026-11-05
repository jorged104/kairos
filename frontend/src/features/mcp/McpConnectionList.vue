<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { McpConnection } from '@/api/mcp'
import { Button } from '@/components/ui/button'

// The live connections, with the owner when they are everybody's (RF-19,
// RF-23). Revoking asks once more on the same row.
defineProps<{ items: McpConnection[]; owners?: boolean }>()
const emit = defineEmits<{ revoke: [string] }>()
const { t, d } = useI18n()
const confirming = ref<string | null>(null)

function revoke(id: string) {
  if (confirming.value !== id) {
    confirming.value = id
    return
  }
  confirming.value = null
  emit('revoke', id)
}
</script>

<template>
  <p v-if="items.length === 0" class="text-sm text-muted-foreground" data-testid="mcp-empty">{{ t('mcp.empty') }}</p>
  <ul v-else class="divide-y rounded-lg border text-sm" data-testid="mcp-list">
    <li v-for="c in items" :key="c.id" class="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 py-2" data-testid="mcp-connection">
      <div class="flex min-w-0 flex-1 flex-col">
        <span class="font-medium">{{ c.name }}</span>
        <span v-if="owners && c.user" class="text-xs text-muted-foreground">{{ c.user.name }} · {{ c.user.email }}</span>
        <span class="text-xs text-muted-foreground">
          {{ t('mcp.createdAt', { at: d(c.created_at, 'long') }) }} ·
          {{ c.last_used_at ? t('mcp.lastUsed', { at: d(c.last_used_at, 'long') }) : t('mcp.neverUsed') }}
          <template v-if="c.last_client"> ({{ c.last_client }})</template>
          · {{ t('mcp.expiresAt', { at: d(c.expires_at, 'long') }) }}
        </span>
      </div>
      <Button
        type="button"
        size="sm"
        :variant="confirming === c.id ? 'destructive' : 'outline'"
        :data-testid="`mcp-revoke-${c.id}`"
        @click="revoke(c.id)"
      >
        {{ confirming === c.id ? t('mcp.revokeConfirm') : t('mcp.revoke') }}
      </Button>
    </li>
  </ul>
</template>

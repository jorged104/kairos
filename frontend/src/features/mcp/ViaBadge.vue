<script setup lang="ts">
import { Bot } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import type { Via } from '@/api/mcp'

// «vía IA»: the change came through an AI connection (spec 016, RF-44).
defineProps<{ via: Via }>()
const { t } = useI18n()
</script>

<template>
  <span
    class="inline-flex items-center gap-1 rounded-full border px-1.5 text-xs text-muted-foreground"
    :title="via.client ? t('mcp.viaClient', { name: via.name, client: via.client }) : via.name"
    data-testid="via-ai"
  >
    <Bot class="size-3" aria-hidden="true" />
    {{ t('mcp.via', { name: via.name }) }}
  </span>
</template>

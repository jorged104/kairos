<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'
import type { Source } from '@/types/api'

defineProps<{ label: string; source?: Source }>()
const { t } = useI18n()
</script>

<template>
  <Badge variant="secondary" class="gap-1">
    {{ label }}
    <!-- The origin of every role and team is visible (RF-86). -->
    <span v-if="source" class="text-muted-foreground" :data-source="source">· {{ t(`sources.${source}`) }}</span>
  </Badge>
</template>

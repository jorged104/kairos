<script setup lang="ts">
import { CircleCheck, CircleQuestionMark, CircleX, LoaderCircle } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ReadinessState, ServiceStatus } from '@/api/health'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

const props = defineProps<{ state?: ReadinessState }>()
const { t } = useI18n()

const rows = computed(() => [
  { key: 'api', label: t('health.api.label'), status: props.state?.api },
  { key: 'database', label: t('health.database.label'), status: props.state?.database },
])

const errorMessage = computed(() => {
  if (props.state?.api === 'unavailable') return t('health.error.apiUnavailable')
  if (props.state?.database === 'unavailable') return t('health.error.databaseUnavailable')
  return null
})

const badge = {
  available: { variant: 'secondary', icon: CircleCheck },
  unavailable: { variant: 'destructive', icon: CircleX },
  unknown: { variant: 'outline', icon: CircleQuestionMark },
} as const

// Literal keys so the catalog test can check every one of them.
function statusText(status?: ServiceStatus) {
  switch (status) {
    case 'available':
      return t('health.status.available')
    case 'unavailable':
      return t('health.status.unavailable')
    case 'unknown':
      return t('health.status.unknown')
    default:
      return t('health.status.checking')
  }
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('health.title') }}</CardTitle>
      <CardDescription>{{ t('health.description') }}</CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-4">
      <dl class="flex flex-col gap-3">
        <div v-for="row in rows" :key="row.key" class="flex items-center justify-between">
          <dt class="font-medium">{{ row.label }}</dt>
          <dd :data-testid="`status-${row.key}`" :data-status="row.status ?? 'checking'">
            <Badge v-if="row.status" :variant="badge[row.status].variant">
              <component :is="badge[row.status].icon" data-icon="inline-start" />
              {{ statusText(row.status) }}
            </Badge>
            <Badge v-else variant="outline">
              <LoaderCircle class="animate-spin" data-icon="inline-start" />
              {{ statusText() }}
            </Badge>
          </dd>
        </div>
      </dl>
      <Alert v-if="errorMessage" variant="destructive" role="alert">
        <CircleX />
        <AlertTitle>{{ t('health.error.title') }}</AlertTitle>
        <AlertDescription>{{ errorMessage }}</AlertDescription>
      </Alert>
    </CardContent>
  </Card>
</template>

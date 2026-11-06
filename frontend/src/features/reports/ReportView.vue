<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Pencil, Trash2 } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { deleteReport, getReport } from '@/api/reports'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import RichTextView from '@/features/richtext/RichTextView.vue'

// A saved report, rendered from its Markdown without HTML (spec 020, RF-31,
// RF-32); editing and deleting for whoever manages the project (RF-34, RF-68).
const props = defineProps<{ reportId: string }>()
const { t, d } = useI18n()
const route = useRoute()
const router = useRouter()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const { data: report, error } = useQuery({ queryKey: ['report', props.reportId], queryFn: () => getReport(props.reportId) })
const back = { query: { ...route.query, report: undefined, edit: undefined } }
const armed = ref(false)
const remove = useMutation({
  mutationFn: () => deleteReport(props.reportId),
  onSuccess: async () => {
    await qc.invalidateQueries({ queryKey: ['reports'] })
    await router.replace(back)
  },
})
function askDelete() {
  if (!armed.value) {
    armed.value = true
    return
  }
  remove.mutate()
}
</script>

<template>
  <article class="flex flex-col gap-4" data-testid="report-view">
    <div class="flex flex-wrap items-center gap-2">
      <Button variant="ghost" size="sm" as-child>
        <RouterLink :to="back"><ArrowLeft aria-hidden="true" />{{ t('reports.back') }}</RouterLink>
      </Button>
      <span class="flex-1" />
      <template v-if="report?.can.manage">
        <Button size="sm" variant="outline" as-child data-testid="report-edit">
          <RouterLink :to="{ query: { ...route.query, edit: '1' } }"><Pencil aria-hidden="true" />{{ t('common.edit') }}</RouterLink>
        </Button>
        <Button size="sm" :variant="armed ? 'destructive' : 'outline'" :disabled="remove.isPending.value" data-testid="report-delete" @click="askDelete">
          <Trash2 aria-hidden="true" />{{ armed ? t('reports.confirmDelete') : t('common.delete') }}
        </Button>
      </template>
    </div>
    <Alert v-if="error || remove.error.value" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error || remove.error.value) }}</AlertDescription>
    </Alert>
    <template v-if="report">
      <header class="flex flex-col gap-1">
        <h2 class="font-heading text-xl font-semibold" data-testid="report-title">{{ report.title }}</h2>
        <p class="text-xs text-muted-foreground">
          {{ report.template_name || t('reports.noTemplate') }} · {{ t('reports.by', { name: report.author.name }) }} ·
          {{ t('reports.updated', { date: d(new Date(report.updated_at), 'long'), name: report.updated_by.name }) }}
        </p>
      </header>
      <div class="rounded-lg border p-5" data-testid="report-content">
        <RichTextView :doc="report.doc" />
      </div>
    </template>
  </article>
</template>

<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { FileText, Sparkles } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { listReports } from '@/api/reports'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { Project } from '@/types/work'
import ReportEditor from './ReportEditor.vue'
import ReportView from './ReportView.vue'

// The «Informes» tab of a project (spec 020, RF-28 to RF-34): the saved
// reports, the most recently changed first; one open by ?report=<id>, a new
// one by ?report=new and its edition by &edit=1.
const props = defineProps<{ project: Project }>()
const { t, d } = useI18n()
const route = useRoute()
const errorMessage = useErrorMessage()

const { data, error } = useQuery({
  queryKey: computed(() => ['reports', props.project.id]),
  queryFn: () => listReports(props.project.id),
})
const open = computed(() => (typeof route.query.report === 'string' ? route.query.report : null))
const editing = computed(() => route.query.edit === '1')
const to = (report?: string, edit = false) => ({ query: { ...route.query, report, edit: edit ? '1' : undefined } })
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="reports-tab">
    <ReportEditor v-if="open === 'new' && data?.can.generate" :project="project" :can="data.can" />
    <template v-else-if="open && open !== 'new'">
      <ReportEditor v-if="editing" :key="`edit-${open}`" :project="project" :report-id="open" :can="data?.can ?? { manage: false, generate: false }" />
      <ReportView v-else :key="open" :report-id="open" />
    </template>
    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-2">
        <p class="text-sm text-muted-foreground">{{ t('reports.hint') }}</p>
        <Button v-if="data?.can.generate" as-child data-testid="report-new">
          <RouterLink :to="to('new')"><Sparkles aria-hidden="true" />{{ t('reports.new') }}</RouterLink>
        </Button>
      </div>
      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>
      <p v-if="data && !data.items.length" class="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground" data-testid="reports-empty">
        {{ data.can.generate ? t('reports.emptyGenerate') : t('reports.empty') }}
      </p>
      <ul v-else-if="data" class="flex flex-col divide-y rounded-lg border">
        <li v-for="r in data.items" :key="r.id" data-testid="report-row">
          <RouterLink :to="to(r.id)" class="flex flex-wrap items-center gap-3 px-4 py-3 hover:bg-muted/50">
            <FileText class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm font-medium">{{ r.title }}</span>
              <span class="block text-xs text-muted-foreground">
                {{ r.template_name || t('reports.noTemplate') }} · {{ t('reports.by', { name: r.author.name }) }}
              </span>
            </span>
            <time class="text-xs text-muted-foreground" :datetime="r.updated_at">{{ d(new Date(r.updated_at), 'long') }}</time>
          </RouterLink>
        </li>
      </ul>
    </template>
  </div>
</template>

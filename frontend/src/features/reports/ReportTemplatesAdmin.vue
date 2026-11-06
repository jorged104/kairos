<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Archive, Pencil, Plus } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  allReportTemplates,
  archiveReportTemplate,
  createReportTemplate,
  updateReportTemplate,
  type ReportTemplate,
} from '@/api/reports'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The kinds of report and their prompts, for the system administrator
// (spec 020, RF-18 to RF-27).
const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const { data, error: loadError } = useQuery({ queryKey: ['report-templates', 'all'], queryFn: allReportTemplates })
const editing = ref<string | 'new' | null>(null)
const form = ref({ name: '', description: '', prompt: '' })
const error = ref<unknown>(null)
const armed = ref<string | null>(null)

function open(x: ReportTemplate | null) {
  error.value = null
  editing.value = x ? x.id : 'new'
  form.value = { name: x?.name ?? '', description: x?.description ?? '', prompt: x?.prompt ?? '' }
}

const refresh = () => qc.invalidateQueries({ queryKey: ['report-templates'] })

const save = useMutation({
  mutationFn: () => {
    const body = { name: form.value.name.trim(), description: form.value.description.trim(), prompt: form.value.prompt.trim() }
    return editing.value === 'new' ? createReportTemplate(body) : updateReportTemplate(String(editing.value), body)
  },
  onSuccess: () => {
    editing.value = null
    void refresh()
  },
  onError: (e) => (error.value = e),
})

const archive = useMutation({
  mutationFn: (id: string) => archiveReportTemplate(id),
  onSuccess: () => {
    armed.value = null
    void refresh()
  },
  onError: (e) => (error.value = e),
})

function askArchive(id: string) {
  if (armed.value !== id) {
    armed.value = id
    return
  }
  archive.mutate(id)
}
</script>

<template>
  <section class="flex max-w-3xl flex-col gap-3" data-testid="report-templates-admin">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <h2 class="text-base font-semibold">{{ t('reports.templates.title') }}</h2>
        <p class="text-xs text-muted-foreground">{{ t('reports.templates.hint') }}</p>
      </div>
      <Button size="sm" variant="outline" data-testid="report-template-new" @click="open(null)">
        <Plus aria-hidden="true" />{{ t('reports.templates.new') }}
      </Button>
    </div>
    <Alert v-if="loadError || error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError || error) }}</AlertDescription>
    </Alert>

    <form v-if="editing" class="flex flex-col gap-3 rounded-lg border p-4" data-testid="report-template-form" @submit.prevent="save.mutate()">
      <label class="flex flex-col gap-1 text-sm">
        {{ t('reports.templates.name') }}
        <Input v-model="form.name" required maxlength="100" data-testid="report-template-name" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('reports.templates.description') }}
        <Input v-model="form.description" maxlength="500" data-testid="report-template-description" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('reports.templates.prompt') }}
        <textarea
          v-model="form.prompt"
          required
          maxlength="20000"
          rows="10"
          class="rounded-md border bg-background px-3 py-2 font-mono text-xs"
          data-testid="report-template-prompt"
        />
        <span class="text-xs text-muted-foreground">{{ t('reports.templates.promptHint') }}</span>
      </label>
      <div class="flex gap-2">
        <Button type="submit" size="sm" :disabled="save.isPending.value" data-testid="report-template-save">{{ t('common.save') }}</Button>
        <Button type="button" size="sm" variant="ghost" @click="editing = null">{{ t('common.cancel') }}</Button>
      </div>
    </form>

    <ul class="flex flex-col divide-y rounded-lg border">
      <li v-for="x in data?.items ?? []" :key="x.id" class="flex flex-wrap items-center gap-2 px-4 py-3" data-testid="report-template">
        <div class="min-w-0 flex-1">
          <p class="flex items-center gap-2 text-sm font-medium">
            {{ x.name }}
            <Badge v-if="x.archived" variant="outline">{{ t('reports.templates.archived') }}</Badge>
          </p>
          <p v-if="x.description" class="truncate text-xs text-muted-foreground">{{ x.description }}</p>
        </div>
        <template v-if="!x.archived">
          <Button size="sm" variant="ghost" :aria-label="t('common.edit')" @click="open(x)"><Pencil aria-hidden="true" /></Button>
          <Button size="sm" :variant="armed === x.id ? 'destructive' : 'outline'" :data-testid="`report-template-archive-${x.id}`" @click="askArchive(x.id)">
            <Archive aria-hidden="true" />{{ armed === x.id ? t('reports.templates.confirmArchive') : t('reports.templates.archive') }}
          </Button>
        </template>
      </li>
    </ul>
  </section>
</template>

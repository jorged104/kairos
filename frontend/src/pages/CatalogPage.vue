<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import CustomFieldsAdmin from '@/features/catalog/CustomFieldsAdmin.vue'
import MethodologiesAdmin from '@/features/catalog/MethodologiesAdmin.vue'
import NamedCatalog from '@/features/catalog/NamedCatalog.vue'
import WorkflowEditor from '@/features/catalog/WorkflowEditor.vue'
import { useCatalog, useCustomFields } from '@/features/work/useCatalog'

// Administration of the spec 004 catalog (RF-1 to RF-17).
const { t } = useI18n()
const errorMessage = useErrorMessage()
const { catalog } = useCatalog()
const fields = useCustomFields()
const error = ref<unknown>(null)
const saved = ref(false)
function onError(e: unknown) {
  saved.value = false
  error.value = e
}
function onSaved() {
  error.value = null
  saved.value = true
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="font-heading text-xl font-semibold">{{ t('catalog.title') }}</h1>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-else-if="saved" role="status">
      <AlertDescription>{{ t('common.saved') }}</AlertDescription>
    </Alert>
    <div class="grid gap-4 lg:grid-cols-2">
      <MethodologiesAdmin :items="catalog.methodologies" @error="onError" />
      <div class="flex flex-col gap-4">
        <NamedCatalog kind="work-item-types" :title="t('catalog.types')" :items="catalog.work_item_types" weighted @error="onError" />
        <NamedCatalog kind="statuses" :title="t('catalog.statuses')" :items="catalog.statuses" closable defaultable @error="onError" />
        <NamedCatalog kind="priorities" :title="t('catalog.priorities')" :items="catalog.priorities" defaultable @error="onError" />
      </div>
    </div>
    <WorkflowEditor :catalog="catalog" @error="onError" @saved="onSaved" />
    <CustomFieldsAdmin :fields="fields" :types="catalog.work_item_types.filter((x) => !x.retired)" @error="onError" />
  </div>
</template>

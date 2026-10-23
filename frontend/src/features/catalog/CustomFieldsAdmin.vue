<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createCustomField, setCustomFieldRetired } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { AttributeType } from '@/types/api'
import type { CustomField, CustomFieldEntity, WorkItemType } from '@/types/work'

// Custom fields of projects and work items (RF-12 to RF-17).
defineProps<{ fields: CustomField[]; types: WorkItemType[] }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()

const TYPES: AttributeType[] = ['text', 'number', 'date', 'boolean', 'select']
interface Form {
  entity: CustomFieldEntity
  name: string
  type: AttributeType
  required: boolean
  options: string
  typeIds: string[]
}
const blank = (): Form => ({ entity: 'work_item', name: '', type: 'text', required: false, options: '', typeIds: [] })
const form = ref<Form>(blank())

const done = () => qc.invalidateQueries({ queryKey: ['custom-fields'] })
const create = useMutation({
  mutationFn: () => {
    const f = form.value
    const options = f.options
      .split(',')
      .map((o) => o.trim())
      .filter(Boolean)
      .map((value) => ({ value, retired: false }))
    return createCustomField({
      entity: f.entity,
      name: f.name,
      type: f.type,
      required: f.required,
      options,
      work_item_type_ids: f.entity === 'work_item' ? f.typeIds : [],
    })
  },
  onSuccess: () => {
    form.value = blank()
    return done()
  },
  onError: (e) => emit('error', e),
})
const retire = useMutation({
  mutationFn: (f: CustomField) => setCustomFieldRetired(f.id, !f.retired_at),
  onSuccess: done,
  onError: (e) => emit('error', e),
})
const entityLabel = (e: CustomFieldEntity) => t(`catalog.entities.${e}`)
const typeLabel = (ty: AttributeType) => t(`attributes.types.${ty}`)
</script>

<template>
  <section class="flex flex-col gap-2 rounded-lg border bg-background p-3">
    <h2 class="font-medium">{{ t('catalog.customFields') }}</h2>
    <ul class="flex flex-col gap-1 text-sm">
      <li v-for="f in fields" :key="f.id" class="flex flex-wrap items-center gap-2" data-testid="custom-field-row">
        <span :class="{ 'text-muted-foreground line-through': f.retired_at }">{{ f.name }}</span>
        <Badge variant="secondary">{{ entityLabel(f.entity) }}</Badge>
        <Badge variant="outline">{{ typeLabel(f.type) }}</Badge>
        <Badge v-if="f.required" variant="outline">{{ t('catalog.required') }}</Badge>
        <span v-if="f.options.length" class="text-xs text-muted-foreground">{{ f.options.map((o) => o.value).join(', ') }}</span>
        <Button size="sm" variant="ghost" @click="retire.mutate(f)">
          {{ f.retired_at ? t('catalog.restore') : t('catalog.retire') }}
        </Button>
      </li>
    </ul>
    <form class="flex flex-wrap items-center gap-2 border-t pt-2 text-sm" @submit.prevent="create.mutate()">
      <NativeSelect v-model="form.entity" class="w-36" :aria-label="t('catalog.entity')">
        <option value="project">{{ entityLabel('project') }}</option>
        <option value="work_item">{{ entityLabel('work_item') }}</option>
      </NativeSelect>
      <Input v-model="form.name" class="w-40" :placeholder="t('catalog.name')" :aria-label="t('catalog.name')" required />
      <NativeSelect v-model="form.type" class="w-32" :aria-label="t('catalog.fieldType')">
        <option v-for="ty in TYPES" :key="ty" :value="ty">{{ typeLabel(ty) }}</option>
      </NativeSelect>
      <Input
        v-if="form.type === 'select'"
        v-model="form.options"
        class="w-48"
        :placeholder="t('catalog.optionsHelp')"
        :aria-label="t('catalog.optionsHelp')"
      />
      <label class="flex items-center gap-1"><input v-model="form.required" type="checkbox" />{{ t('catalog.required') }}</label>
      <Button type="submit" size="sm">{{ t('common.add') }}</Button>
      <div v-if="form.entity === 'work_item'" class="flex w-full flex-wrap gap-2 text-xs text-muted-foreground">
        {{ t('catalog.limitTypes') }}
        <label v-for="ty in types" :key="ty.id" class="flex items-center gap-1">
          <input v-model="form.typeIds" type="checkbox" :value="ty.id" />{{ ty.name }}
        </label>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createCatalogItem, setCatalogRetired, updateCatalogItem, type CatalogInput, type CatalogKind } from '@/api/work'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// One list of the catalog: work item types, statuses or priorities (RF-3 to RF-10).
interface Item {
  id: string
  name: string
  color: string
  position: number
  retired: boolean
  is_closed?: boolean
  is_default?: boolean
}

const props = defineProps<{
  kind: CatalogKind
  title: string
  items: Item[]
  closable?: boolean
  defaultable?: boolean
}>()
const emit = defineEmits<{ error: [unknown] }>()

const { t } = useI18n()
const qc = useQueryClient()

const blank = (): CatalogInput => ({ name: '', color: '#64748b', position: props.items.length + 1, is_closed: false, is_default: false })
const form = ref<CatalogInput>(blank())
const editing = ref<string | null>(null)
const edit = ref<CatalogInput>(blank())

const done = () => qc.invalidateQueries({ queryKey: ['catalog'] })
const save = useMutation({
  mutationFn: (input: { id?: string; body: CatalogInput }) =>
    input.id ? updateCatalogItem(props.kind, input.id, input.body) : createCatalogItem(props.kind, input.body),
  onSuccess: done,
  onError: (e) => emit('error', e),
})
const retire = useMutation({
  mutationFn: (item: Item) => setCatalogRetired(props.kind, item.id, !item.retired),
  onSuccess: done,
  onError: (e) => emit('error', e),
})

async function submitNew() {
  await save.mutateAsync({ body: form.value }).catch(() => undefined)
  if (save.isSuccess.value) form.value = blank()
}
function startEdit(item: Item) {
  editing.value = item.id
  edit.value = { name: item.name, color: item.color, position: item.position, is_closed: item.is_closed, is_default: item.is_default }
}
async function submitEdit() {
  await save.mutateAsync({ id: editing.value as string, body: edit.value }).catch(() => undefined)
  if (save.isSuccess.value) editing.value = null
}
</script>

<template>
  <section class="flex flex-col gap-2 rounded-lg border bg-background p-3">
    <h2 class="font-medium">{{ title }}</h2>
    <ul class="flex flex-col gap-1 text-sm">
      <li v-for="item in items" :key="item.id" class="flex flex-wrap items-center gap-2" :data-testid="`${kind}-row`">
        <form v-if="editing === item.id" class="flex flex-wrap items-center gap-2" @submit.prevent="submitEdit">
          <Input v-model="edit.name" class="w-40" :aria-label="t('catalog.name')" required />
          <input v-model="edit.color" type="color" :aria-label="t('catalog.color')" />
          <Input v-model.number="edit.position" type="number" min="0" class="w-16" :aria-label="t('catalog.position')" />
          <label v-if="closable" class="flex items-center gap-1"><input v-model="edit.is_closed" type="checkbox" />{{ t('catalog.closed') }}</label>
          <label v-if="defaultable" class="flex items-center gap-1"><input v-model="edit.is_default" type="checkbox" />{{ t('catalog.default') }}</label>
          <Button type="submit" size="sm">{{ t('common.save') }}</Button>
          <Button type="button" size="sm" variant="ghost" @click="editing = null">{{ t('common.cancel') }}</Button>
        </form>
        <template v-else>
          <span class="size-3 rounded-full" :style="{ background: item.color }" />
          <span :class="{ 'text-muted-foreground line-through': item.retired }">{{ item.name }}</span>
          <Badge v-if="item.is_closed" variant="secondary">{{ t('catalog.closed') }}</Badge>
          <Badge v-if="item.is_default" variant="outline">{{ t('catalog.default') }}</Badge>
          <Button size="sm" variant="ghost" @click="startEdit(item)">{{ t('common.edit') }}</Button>
          <Button size="sm" variant="ghost" @click="retire.mutate(item)">
            {{ item.retired ? t('catalog.restore') : t('catalog.retire') }}
          </Button>
        </template>
      </li>
    </ul>
    <form class="flex flex-wrap items-center gap-2 border-t pt-2" @submit.prevent="submitNew">
      <Input v-model="form.name" class="w-40" :placeholder="t('catalog.name')" :aria-label="t('catalog.name')" required />
      <input v-model="form.color" type="color" :aria-label="t('catalog.color')" />
      <label v-if="closable" class="flex items-center gap-1 text-sm"><input v-model="form.is_closed" type="checkbox" />{{ t('catalog.closed') }}</label>
      <label v-if="defaultable" class="flex items-center gap-1 text-sm"><input v-model="form.is_default" type="checkbox" />{{ t('catalog.default') }}</label>
      <Button type="submit" size="sm">{{ t('common.add') }}</Button>
    </form>
  </section>
</template>

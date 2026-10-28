<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createCatalogItem, setCatalogRetired, updateCatalogItem, type CatalogInput, type CatalogKind } from '@/api/work'
import { EditableCell, GridTable, QuickAddRow } from '@/components/data-table'
import { Button } from '@/components/ui/button'

// One list of the catalog: work item types, statuses or priorities (RF-3 to RF-10),
// edited cell by cell and with a quick row to add (RF-139, RF-140).
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
const done = () => qc.invalidateQueries({ queryKey: ['catalog'] })

const input = (item: Item): CatalogInput => ({
  name: item.name, color: item.color, position: item.position, is_closed: item.is_closed, is_default: item.is_default,
})
type Field = 'name' | 'color' | 'position' | 'is_closed' | 'is_default'
function saver(item: Item, field: Field) {
  return async (value: string) => {
    const v = field === 'position' ? Number(value) : field === 'is_closed' || field === 'is_default' ? value === 'true' : value
    await updateCatalogItem(props.kind, item.id, { ...input(item), [field]: v })
    await done()
  }
}

const newColor = ref('#64748b')
async function create(name: string) {
  await createCatalogItem(props.kind, { name, color: newColor.value, position: props.items.length + 1, is_closed: false, is_default: false })
  await done()
}

const retire = useMutation({
  mutationFn: (item: Item) => setCatalogRetired(props.kind, item.id, !item.retired),
  onSuccess: done,
  onError: (e) => emit('error', e),
})
const columns = 4 + (props.closable ? 1 : 0) + (props.defaultable ? 1 : 0)
</script>

<template>
  <section class="flex flex-col gap-2">
    <h2 class="font-medium">{{ title }}</h2>
    <GridTable>
      <thead>
        <tr>
          <th>{{ t('catalog.name') }}</th>
          <th class="w-36">{{ t('catalog.color') }}</th>
          <th class="w-20">{{ t('catalog.position') }}</th>
          <th v-if="closable" class="w-24">{{ t('catalog.closed') }}</th>
          <th v-if="defaultable" class="w-24">{{ t('catalog.default') }}</th>
          <th class="w-28" />
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="item.id" class="hover:bg-muted/30" :data-testid="`${kind}-row`">
          <EditableCell :value="item.name" :label="t('catalog.name')" :save="saver(item, 'name')" :editable="!item.retired" required>
            <span :class="{ 'text-muted-foreground line-through': item.retired }">{{ item.name }}</span>
          </EditableCell>
          <EditableCell kind="color" :value="item.color" :label="t('catalog.color')" :save="saver(item, 'color')" :editable="!item.retired" />
          <EditableCell kind="number" :value="String(item.position)" :label="t('catalog.position')" :save="saver(item, 'position')" :editable="!item.retired" required />
          <EditableCell
            v-if="closable"
            kind="boolean"
            :value="String(!!item.is_closed)"
            :label="t('catalog.closed')"
            :save="saver(item, 'is_closed')"
            :editable="!item.retired"
          />
          <EditableCell
            v-if="defaultable"
            kind="boolean"
            :value="String(!!item.is_default)"
            :label="t('catalog.default')"
            :save="saver(item, 'is_default')"
            :editable="!item.retired"
          />
          <td class="px-2 text-right">
            <Button size="sm" variant="ghost" @click="retire.mutate(item)">
              {{ item.retired ? t('catalog.restore') : t('catalog.retire') }}
            </Button>
          </td>
        </tr>
        <QuickAddRow :colspan="columns" :label="t(`catalog.new.${kind}`)" :placeholder="t('catalog.name')" :submit="create">
          <input v-model="newColor" type="color" class="size-6 cursor-pointer rounded border-0 bg-transparent p-0" :aria-label="t('catalog.color')" />
        </QuickAddRow>
      </tbody>
    </GridTable>
  </section>
</template>

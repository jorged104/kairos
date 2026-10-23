<script setup lang="ts">
import { useMutation, useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getTransitions, setTransitions } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import { ROLES, type Role } from '@/types/api'
import type { Catalog, Transition } from '@/types/work'
import { cellsFrom, transitionsFrom } from './workflow'

// Allowed status changes per type (RF-6). Each cell is "not allowed", "every
// role" or one role. An empty grid allows every change (RF-87).
const props = defineProps<{ catalog: Catalog }>()
const emit = defineEmits<{ error: [unknown]; saved: [] }>()
const { t } = useI18n()

const types = computed(() => props.catalog.work_item_types.filter((x) => !x.retired))
const statuses = computed(() => props.catalog.statuses.filter((x) => !x.retired))
const typeId = ref('')
watch(types, (list) => (typeId.value ||= list[0]?.id ?? ''), { immediate: true })

const { data } = useQuery({
  queryKey: ['transitions', typeId],
  queryFn: () => getTransitions(typeId.value),
  enabled: computed(() => !!typeId.value),
})
const cells = ref<Record<string, string>>({})
watch(data, (ts) => (cells.value = cellsFrom(ts ?? [])), { immediate: true })
const cellKey = (from: string, to: string) => `${from}>${to}`

const save = useMutation({
  mutationFn: () => setTransitions(typeId.value, transitionsFrom(cells.value) as Transition[]),
  onSuccess: () => emit('saved'),
  onError: (e) => emit('error', e),
})
const roleLabel = (r: Role) => t(`roles.${r}`)
</script>

<template>
  <section class="flex flex-col gap-2 rounded-lg border bg-background p-3">
    <div class="flex flex-wrap items-center gap-2">
      <h2 class="font-medium">{{ t('catalog.workflow') }}</h2>
      <NativeSelect v-model="typeId" class="w-48" :aria-label="t('catalog.type')">
        <option v-for="ty in types" :key="ty.id" :value="ty.id">{{ ty.name }}</option>
      </NativeSelect>
    </div>
    <p class="text-xs text-muted-foreground">{{ t('catalog.workflowHelp') }}</p>
    <div class="overflow-x-auto">
      <table class="text-xs">
        <thead>
          <tr>
            <th class="p-1 text-left">{{ t('catalog.fromTo') }}</th>
            <th v-for="to in statuses" :key="to.id" class="p-1">{{ to.name }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="from in statuses" :key="from.id">
            <th class="p-1 text-left font-medium">{{ from.name }}</th>
            <td v-for="to in statuses" :key="to.id" class="p-1">
              <NativeSelect
                v-if="from.id !== to.id"
                v-model="cells[cellKey(from.id, to.id)]"
                class="h-7 w-24"
                :aria-label="from.name + ' → ' + to.name"
              >
                <option value="">—</option>
                <option value="all">{{ t('catalog.everyRole') }}</option>
                <option v-for="r in ROLES" :key="r" :value="r">{{ roleLabel(r) }}</option>
              </NativeSelect>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div>
      <Button size="sm" :disabled="save.isPending.value" @click="save.mutate()">{{ t('common.save') }}</Button>
    </div>
  </section>
</template>

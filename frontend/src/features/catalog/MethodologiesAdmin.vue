<script setup lang="ts">
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createCatalogItem, createPhaseDefinition, setCatalogRetired, updateCatalogItem, type PhaseInput } from '@/api/work'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Methodology } from '@/types/work'

// Methodologies and their phases (RF-1, RF-2).
const props = defineProps<{ items: Methodology[] }>()
const emit = defineEmits<{ error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()

function mutation<A>(fn: (arg: A) => Promise<unknown>) {
  return useMutation({
    mutationFn: fn,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['catalog'] }),
    onError: (e) => emit('error', e),
  })
}

const name = ref('')
const create = mutation(() => createCatalogItem('methodologies', { name: name.value, position: props.items.length + 1 }))
async function submit() {
  await create.mutateAsync(undefined).catch(() => undefined)
  if (create.isSuccess.value) name.value = ''
}

const renaming = ref<Record<string, string>>({})
const rename = mutation((m: Methodology) => updateCatalogItem('methodologies', m.id, { name: renaming.value[m.id], position: m.position }))
async function submitRename(m: Methodology) {
  await rename.mutateAsync(m).catch(() => undefined)
  if (rename.isSuccess.value) delete renaming.value[m.id]
}

const retire = mutation((x: { kind: 'methodologies' | 'phase-definitions'; id: string; retired: boolean }) =>
  setCatalogRetired(x.kind, x.id, !x.retired),
)

const phase = ref<Record<string, PhaseInput>>({})
function phaseFor(id: string): PhaseInput {
  if (!phase.value[id]) phase.value[id] = { name: '', position: 0, has_start_gate: false, has_finish_gate: false }
  return phase.value[id]
}
const addPhase = mutation((m: Methodology) => createPhaseDefinition(m.id, { ...phaseFor(m.id), position: m.phases.length + 1 }))
async function submitPhase(m: Methodology) {
  await addPhase.mutateAsync(m).catch(() => undefined)
  if (addPhase.isSuccess.value) delete phase.value[m.id]
}
</script>

<template>
  <section class="flex flex-col gap-3 rounded-lg border bg-background p-3">
    <h2 class="font-medium">{{ t('catalog.methodologies') }}</h2>
    <div v-for="m in items" :key="m.id" class="rounded-md border p-2" data-testid="methodology-row">
      <div class="flex flex-wrap items-center gap-2">
        <form v-if="renaming[m.id] !== undefined" class="flex items-center gap-2" @submit.prevent="submitRename(m)">
          <Input v-model="renaming[m.id]" class="w-48" :aria-label="t('catalog.name')" required />
          <Button type="submit" size="sm">{{ t('common.save') }}</Button>
        </form>
        <template v-else>
          <span class="font-medium" :class="{ 'text-muted-foreground line-through': m.retired }">{{ m.name }}</span>
          <Button size="sm" variant="ghost" @click="renaming[m.id] = m.name">{{ t('common.edit') }}</Button>
        </template>
        <Button size="sm" variant="ghost" @click="retire.mutate({ kind: 'methodologies', id: m.id, retired: m.retired })">
          {{ m.retired ? t('catalog.restore') : t('catalog.retire') }}
        </Button>
      </div>
      <ol class="mt-2 flex flex-col gap-1 pl-4 text-sm">
        <li v-for="ph in m.phases" :key="ph.id" class="flex flex-wrap items-center gap-2">
          <span :class="{ 'text-muted-foreground line-through': ph.retired }">{{ ph.position }}. {{ ph.name }}</span>
          <Badge v-if="ph.has_start_gate" variant="outline">{{ t('catalog.startGate') }}</Badge>
          <Badge v-if="ph.has_finish_gate" variant="outline">{{ t('catalog.finishGate') }}</Badge>
          <Button size="sm" variant="ghost" @click="retire.mutate({ kind: 'phase-definitions', id: ph.id, retired: ph.retired })">
            {{ ph.retired ? t('catalog.restore') : t('catalog.retire') }}
          </Button>
        </li>
      </ol>
      <form class="mt-2 flex flex-wrap items-center gap-2 pl-4 text-sm" @submit.prevent="submitPhase(m)">
        <Input
          v-model="phaseFor(m.id).name"
          class="w-48"
          :placeholder="t('catalog.newPhase')"
          :aria-label="t('catalog.newPhase')"
          required
        />
        <label class="flex items-center gap-1"><input v-model="phaseFor(m.id).has_start_gate" type="checkbox" />{{ t('catalog.startGate') }}</label>
        <label class="flex items-center gap-1"><input v-model="phaseFor(m.id).has_finish_gate" type="checkbox" />{{ t('catalog.finishGate') }}</label>
        <Button type="submit" size="sm" variant="outline">{{ t('common.add') }}</Button>
      </form>
    </div>
    <form class="flex items-center gap-2 border-t pt-2" @submit.prevent="submit">
      <Input v-model="name" class="w-48" :placeholder="t('catalog.name')" :aria-label="t('catalog.name')" required />
      <Button type="submit" size="sm">{{ t('common.add') }}</Button>
    </form>
  </section>
</template>

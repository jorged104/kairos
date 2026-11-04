<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Trash2 } from '@lucide/vue'
import { createLevel, deleteLevel, listLevels, updateLevel } from '@/api/gamification'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// The table of levels: name and minimum XP; the first one always starts at 0
// (spec 014, RF-30 to RF-34).
const { t } = useI18n()
const client = useQueryClient()
const errorMessage = useErrorMessage()
const { data } = useQuery({ queryKey: ['points', 'levels'], queryFn: listLevels })

type Draft = { id: string; number: number; name: string; min_xp: number; first: boolean }
const drafts = ref<Draft[]>([])
watch(
  data,
  (ls) => (drafts.value = (ls ?? []).map((l) => ({ id: l.id, number: l.number, name: l.name, min_xp: l.min_xp, first: l.min_xp === 0 }))),
  { immediate: true },
)
const added = ref({ name: '', min_xp: 0 })
const error = ref<unknown>(null)

async function run(fn: () => Promise<unknown>) {
  error.value = null
  try {
    await fn()
    // Levels move everybody's level, wherever it shows (RF-35).
    await client.invalidateQueries({ queryKey: ['points'] })
    return true
  } catch (e) {
    error.value = e
    return false
  }
}
const save = (d: Draft) => run(() => updateLevel(d.id, { name: d.name, min_xp: Number(d.min_xp) }))
const remove = (d: Draft) => run(() => deleteLevel(d.id))
async function add() {
  if (await run(() => createLevel({ name: added.value.name, min_xp: Number(added.value.min_xp) }))) added.value = { name: '', min_xp: 0 }
}
</script>

<template>
  <div class="flex flex-col gap-3" data-testid="levels-admin">
    <Alert v-if="error" variant="destructive" role="alert" data-testid="levels-error">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <ul class="flex max-w-xl flex-col gap-2">
      <li v-for="d in drafts" :key="d.id" class="flex items-center gap-2" data-testid="level-row">
        <span class="w-8 text-right text-sm font-semibold tabular-nums text-muted-foreground">{{ d.number }}</span>
        <Input v-model="d.name" maxlength="40" class="h-8 flex-1" :aria-label="t('gamification.levels.name')" @change="save(d)" />
        <Input
          v-model.number="d.min_xp"
          type="number"
          min="0"
          class="h-8 w-28"
          :disabled="d.first"
          :title="d.first ? t('gamification.levels.firstLocked') : undefined"
          :aria-label="t('gamification.levels.minXp')"
          data-testid="level-min"
          @change="save(d)"
        />
        <Button v-if="!d.first" variant="ghost" size="icon" :aria-label="t('common.delete')" data-testid="level-delete" @click="remove(d)">
          <Trash2 class="size-4" />
        </Button>
        <span v-else class="size-9" />
      </li>
    </ul>
    <form class="flex max-w-xl items-end gap-2" data-testid="level-add" @submit.prevent="add">
      <label class="flex flex-1 flex-col gap-1 text-xs text-muted-foreground">
        {{ t('gamification.levels.name') }}
        <Input v-model="added.name" required maxlength="40" class="h-8" data-testid="level-add-name" />
      </label>
      <label class="flex flex-col gap-1 text-xs text-muted-foreground">
        {{ t('gamification.levels.minXp') }}
        <Input v-model.number="added.min_xp" type="number" min="1" required class="h-8 w-28" data-testid="level-add-min" />
      </label>
      <Button type="submit" variant="outline" data-testid="level-add-submit">{{ t('gamification.levels.add') }}</Button>
    </form>
  </div>
</template>

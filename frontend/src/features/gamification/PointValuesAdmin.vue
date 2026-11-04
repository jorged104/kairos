<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPointValues, setPointValues, type PointValue } from '@/api/gamification'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'

// What closing a work item of each size is worth (spec 014, RF-24 to RF-27).
const { t } = useI18n()
const client = useQueryClient()
const errorMessage = useErrorMessage()
const { data } = useQuery({ queryKey: ['points', 'values'], queryFn: getPointValues })

type Row = { key: string; sizeId: string | null; name: string; retired: boolean } & PointValue
const rows = ref<Row[]>([])
watch(
  data,
  (v) => {
    if (!v) return
    rows.value = [
      { key: 'none', sizeId: null, name: t('gamification.values.noSize'), retired: false, ...v.no_size },
      ...v.sizes.map((s) => ({ key: s.size_id, sizeId: s.size_id, name: s.name, retired: s.retired, xp: s.xp, coins: s.coins })),
    ]
  },
  { immediate: true },
)
const error = ref<unknown>(null)
const saved = ref(false)
const busy = ref(false)

async function save() {
  error.value = null
  saved.value = false
  busy.value = true
  try {
    const none = rows.value.find((r) => r.sizeId === null)!
    await setPointValues({
      no_size: { xp: Number(none.xp), coins: Number(none.coins) },
      sizes: rows.value.filter((r) => r.sizeId).map((r) => ({ size_id: r.sizeId!, xp: Number(r.xp), coins: Number(r.coins) })),
    })
    saved.value = true
    await client.invalidateQueries({ queryKey: ['points', 'values'] })
  } catch (e) {
    error.value = e
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="flex flex-col gap-3" data-testid="point-values" @submit.prevent="save">
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-if="saved" role="status">
      <AlertDescription>{{ t('gamification.values.saved') }}</AlertDescription>
    </Alert>
    <table class="w-full max-w-md text-sm">
      <thead>
        <tr class="text-left text-xs text-muted-foreground">
          <th class="py-1 font-medium">{{ t('gamification.values.size') }}</th>
          <th class="py-1 font-medium">{{ t('gamification.history.xp') }}</th>
          <th class="py-1 font-medium">{{ t('gamification.history.coins') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.key" class="border-t" data-testid="point-value-row">
          <td class="py-1.5 pr-2">
            {{ r.name }}
            <span v-if="r.retired" class="text-xs text-muted-foreground">· {{ t('gamification.values.retired') }}</span>
          </td>
          <td class="py-1.5 pr-2">
            <Input v-model.number="r.xp" type="number" min="0" max="1000" class="h-8 w-24" :aria-label="`${r.name}: ${t('gamification.history.xp')}`" data-testid="point-value-xp" />
          </td>
          <td class="py-1.5">
            <Input v-model.number="r.coins" type="number" min="0" max="1000" class="h-8 w-24" :aria-label="`${r.name}: ${t('gamification.history.coins')}`" data-testid="point-value-coins" />
          </td>
        </tr>
      </tbody>
    </table>
    <p class="text-xs text-muted-foreground">{{ t('gamification.values.hint') }}</p>
    <div>
      <Button type="submit" :disabled="busy" data-testid="point-values-save">{{ t('common.save') }}</Button>
    </div>
  </form>
</template>

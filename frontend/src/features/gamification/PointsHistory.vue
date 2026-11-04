<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { getMovements, type Movement } from '@/api/gamification'
import { Button } from '@/components/ui/button'

// The movements of a person, newest first, 50 per page (spec 014, RF-39).
// Without userId, the caller's own.
const props = defineProps<{ userId?: string }>()
const { t, d } = useI18n()

const page = ref(1)
watch(
  () => props.userId,
  () => (page.value = 1),
)
const { data } = useQuery({
  queryKey: computed(() => ['points', 'movements', props.userId ?? 'me', page.value]),
  queryFn: () => getMovements({ userId: props.userId, page: page.value }),
  placeholderData: keepPreviousData,
})
const pages = computed(() => (data.value ? Math.max(1, Math.ceil(data.value.total / data.value.page_size)) : 1))
const signed = (v: number) => (v > 0 ? `+${v}` : String(v))
const reason = (m: Movement) => t(`gamification.reasons.${m.reason}`)
</script>

<template>
  <div class="flex flex-col gap-2" data-testid="points-history">
    <p v-if="data && !data.items.length" class="text-sm text-muted-foreground" data-testid="points-history-empty">{{ t('gamification.history.empty') }}</p>
    <table v-else-if="data" class="w-full text-sm">
      <thead>
        <tr class="text-left text-xs text-muted-foreground">
          <th class="py-1 font-medium">{{ t('gamification.history.date') }}</th>
          <th class="py-1 font-medium">{{ t('gamification.history.reason') }}</th>
          <th class="py-1 text-right font-medium">{{ t('gamification.history.xp') }}</th>
          <th class="py-1 text-right font-medium">{{ t('gamification.history.coins') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="m in data.items" :key="m.id" class="border-t align-top" data-testid="points-movement">
          <td class="py-1.5 pr-2 whitespace-nowrap text-muted-foreground">{{ d(new Date(m.created_at), 'long') }}</td>
          <td class="py-1.5 pr-2">
            <span>{{ reason(m) }}</span>
            <RouterLink v-if="m.work_item" :to="`/work-items/${m.work_item.key}`" class="ml-1 hover:underline" data-testid="points-movement-link">
              {{ m.work_item.key }} · {{ m.work_item.title }}
            </RouterLink>
            <span v-if="m.note" class="block text-xs text-muted-foreground">{{ m.note }}</span>
          </td>
          <td class="py-1.5 text-right tabular-nums" :class="m.xp < 0 && 'text-destructive'">{{ m.xp ? signed(m.xp) : '—' }}</td>
          <td class="py-1.5 text-right tabular-nums" :class="m.coins < 0 && 'text-destructive'">{{ m.coins ? signed(m.coins) : '—' }}</td>
        </tr>
      </tbody>
    </table>
    <div v-if="pages > 1" class="flex items-center justify-end gap-2 text-xs">
      <Button variant="outline" size="sm" :disabled="page <= 1" data-testid="points-prev" @click="page--">{{ t('gamification.history.prev') }}</Button>
      <span class="tabular-nums">{{ t('gamification.history.page', { page, pages }) }}</span>
      <Button variant="outline" size="sm" :disabled="page >= pages" data-testid="points-next" @click="page++">{{ t('gamification.history.next') }}</Button>
    </div>
  </div>
</template>

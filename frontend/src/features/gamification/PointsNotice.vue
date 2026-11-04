<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { X } from '@lucide/vue'
import { getMovements, getPoints } from '@/api/gamification'

// Tells the person what a close just gave them and whether they went up a
// level (spec 014, RF-41, RF-42). Points arrive from the server a moment after
// the close, so the balance is read every few seconds while the tab is visible.
const POLL_MS = 5_000
const SHOW_MS = 6_000

const { t, n } = useI18n()
const client = useQueryClient()
const { data } = useQuery({ queryKey: ['points', 'me'], queryFn: () => getPoints(), refetchInterval: POLL_MS })

type Notice = { xp: number; coins: number; level: string | null }
const notice = ref<Notice | null>(null)
let seen: { seq: number; level: number } | null = null
let timer: ReturnType<typeof setTimeout> | undefined

watch(data, async (p) => {
  if (!p) return
  // The first reading only sets the starting point.
  if (!seen) {
    seen = { seq: p.last_seq, level: p.level.number }
    return
  }
  if (p.last_seq <= seen.seq) return
  const before = seen
  seen = { seq: p.last_seq, level: p.level.number }
  void client.invalidateQueries({ queryKey: ['points', 'movements'] })
  try {
    const page = await getMovements({ afterSeq: before.seq })
    const closes = page.items.filter((m) => m.reason === 'close')
    const xp = closes.reduce((s, m) => s + m.xp, 0)
    const coins = closes.reduce((s, m) => s + m.coins, 0)
    if (!xp && !coins) return
    notice.value = { xp, coins, level: p.level.number > before.level ? p.level.name : null }
    clearTimeout(timer)
    timer = setTimeout(() => (notice.value = null), SHOW_MS)
  } catch {
    // Without the detail there is nothing to say; the balance is right anyway.
  }
})
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="pointer-events-none fixed right-4 bottom-4 z-50 flex flex-col items-end" aria-live="polite">
    <div
      v-if="notice"
      class="pointer-events-auto flex items-start gap-3 rounded-xl border bg-background px-4 py-3 text-sm shadow-lg"
      data-testid="points-notice"
    >
      <div>
        <p class="font-semibold" data-testid="points-notice-gain">
          {{ t('gamification.notice.gain', { xp: n(notice.xp), coins: t('gamification.coins', { n: n(notice.coins) }, notice.coins) }) }}
        </p>
        <p v-if="notice.level" class="text-primary" data-testid="points-notice-level">{{ t('gamification.notice.levelUp', { name: notice.level }) }}</p>
      </div>
      <button type="button" class="text-muted-foreground hover:text-foreground" :aria-label="t('common.close')" @click="notice = null">
        <X class="size-4" />
      </button>
    </div>
  </div>
</template>

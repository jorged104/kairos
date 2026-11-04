<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PointsSummary } from '@/api/gamification'

// Level, XP, coins and the way to the next level (spec 014, RF-37, RF-38).
const props = defineProps<{ points: PointsSummary }>()
const { t, n } = useI18n()

const progress = computed(() => {
  const { xp, level, next } = props.points
  if (!next) return 100
  const span = next.min_xp - level.min_xp
  return span > 0 ? Math.min(100, Math.max(0, (100 * (xp - level.min_xp)) / span)) : 0
})
const missing = computed(() => (props.points.next ? Math.max(0, props.points.next.min_xp - props.points.xp) : 0))
</script>

<template>
  <div class="flex flex-col gap-2" data-testid="level-progress">
    <div class="flex items-baseline justify-between gap-2">
      <p class="text-sm font-semibold" data-testid="level-name">
        {{ t('gamification.level', { number: points.level.number, name: points.level.name }) }}
      </p>
      <p class="flex gap-3 text-xs text-muted-foreground tabular-nums">
        <span data-testid="points-xp">{{ t('gamification.xp', { n: n(points.xp) }) }}</span>
        <span data-testid="points-coins">{{ t('gamification.coins', { n: n(points.coins) }, points.coins) }}</span>
      </p>
    </div>
    <template v-if="points.next">
      <div
        class="h-2 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        :aria-valuenow="Math.round(progress)"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-label="t('gamification.toNext', { xp: n(missing), name: points.next.name })"
      >
        <div class="h-full rounded-full bg-primary" :style="{ width: `${progress}%` }" />
      </div>
      <p class="text-xs text-muted-foreground" data-testid="level-missing">{{ t('gamification.toNext', { xp: n(missing), name: points.next.name }) }}</p>
    </template>
    <p v-else class="text-xs font-medium text-primary" data-testid="level-max">{{ t('gamification.maxLevel') }}</p>
  </div>
</template>

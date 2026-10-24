<script setup lang="ts">
// A plain progress bar with its percentage.
defineProps<{ value: number; label?: string }>()
</script>

<template>
  <div class="flex items-center gap-2" role="progressbar" :aria-valuenow="value" aria-valuemin="0" aria-valuemax="100" :aria-label="label">
    <div class="h-2 w-full min-w-16 overflow-hidden rounded-full bg-muted">
      <div class="h-full rounded-full bg-primary transition-all" :style="{ width: `${Math.min(100, Math.max(0, value))}%` }" />
    </div>
    <span class="w-10 text-right text-xs tabular-nums text-muted-foreground">{{ value }} %</span>
  </div>
</template>

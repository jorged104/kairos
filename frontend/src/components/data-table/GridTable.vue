<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { onGridKeydown } from './gridNav'

// Frame of an editable table: grid lines, sticky look of the headers and
// arrow keys between editable cells.
defineProps<{ empty?: boolean }>()
const { t } = useI18n()
</script>

<template>
  <div class="overflow-x-auto rounded-lg border bg-background">
    <table
      class="w-full border-collapse text-sm [&_td]:border-b [&_td]:border-r [&_td:last-child]:border-r-0 [&_th]:h-9 [&_th]:border-r [&_th]:border-b [&_th]:bg-muted/60 [&_th]:px-2 [&_th]:text-left [&_th]:text-xs [&_th]:font-medium [&_th]:whitespace-nowrap [&_th]:text-muted-foreground [&_th:last-child]:border-r-0 [&_tr:last-child>td]:border-b-0"
      @keydown="onGridKeydown"
    >
      <slot />
    </table>
    <p v-if="empty" class="border-t p-4 text-sm text-muted-foreground">
      <slot name="empty">{{ t('common.empty') }}</slot>
    </p>
  </div>
</template>

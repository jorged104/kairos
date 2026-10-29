<script setup lang="ts">
import WorkItemSheet from '@/features/workitems/WorkItemSheet.vue'

// The full page of a work item, at its own URL (spec 005, RF-39).
defineProps<{ wikey: string }>()
</script>

<template>
  <WorkItemSheet :wikey="wikey" />
</template>

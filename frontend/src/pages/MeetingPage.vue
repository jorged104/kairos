<script setup lang="ts">
import MeetingSheet from '@/features/meetings/MeetingSheet.vue'

// The full page of a meeting, at its own address (spec 018, RF-42).
defineProps<{ id: string }>()
</script>

<template>
  <Transition name="sheet" mode="out-in" appear>
    <MeetingSheet :key="id" :id="id" />
  </Transition>
</template>

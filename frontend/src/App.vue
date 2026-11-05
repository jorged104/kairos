<script setup lang="ts">
import { RouterView } from 'vue-router'
import { useAppearance } from '@/composables/useAppearance'

useAppearance()
</script>

<template>
  <RouterView />
</template>

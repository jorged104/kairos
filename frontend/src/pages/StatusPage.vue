<script setup lang="ts">
import HealthStatus from '@/features/health/HealthStatus.vue'
import { useReadiness } from '@/features/health/useReadiness'

const { data } = useReadiness()
</script>

<template>
  <main class="mx-auto flex min-h-svh max-w-xl flex-col justify-center p-4">
    <HealthStatus :state="data" />
  </main>
</template>

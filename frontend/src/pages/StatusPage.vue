<script setup lang="ts">
import HealthStatus from '@/features/health/HealthStatus.vue'
import { useReadiness } from '@/features/health/useReadiness'

const { data } = useReadiness()
</script>

<template>
  <div class="mx-auto max-w-xl py-8">
    <HealthStatus :state="data" />
  </div>
</template>

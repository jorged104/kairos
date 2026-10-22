<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <main class="mx-auto flex min-h-svh max-w-xl flex-col justify-center gap-6 p-4">
    <h1 class="text-2xl font-semibold">{{ t('health.title') }}</h1>
  </main>
</template>

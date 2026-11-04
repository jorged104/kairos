<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import LevelsAdmin from '@/features/gamification/LevelsAdmin.vue'
import PersonPoints from '@/features/gamification/PersonPoints.vue'
import PointValuesAdmin from '@/features/gamification/PointValuesAdmin.vue'

// Administration of the gamification: points per size, levels and the points
// of a person (spec 014).
const { t } = useI18n()
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="gamification-admin">
    <h1 class="font-heading text-xl font-semibold">{{ t('gamification.admin.title') }}</h1>
    <Card>
      <CardHeader>
        <CardTitle>{{ t('gamification.values.title') }}</CardTitle>
        <CardDescription>{{ t('gamification.values.description') }}</CardDescription>
      </CardHeader>
      <CardContent><PointValuesAdmin /></CardContent>
    </Card>
    <Card>
      <CardHeader>
        <CardTitle>{{ t('gamification.levels.title') }}</CardTitle>
        <CardDescription>{{ t('gamification.levels.description') }}</CardDescription>
      </CardHeader>
      <CardContent><LevelsAdmin /></CardContent>
    </Card>
    <Card>
      <CardHeader>
        <CardTitle>{{ t('gamification.person.title') }}</CardTitle>
        <CardDescription>{{ t('gamification.person.description') }}</CardDescription>
      </CardHeader>
      <CardContent><PersonPoints /></CardContent>
    </Card>
  </div>
</template>

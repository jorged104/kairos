<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getProjectSchedule, setBaseline } from '@/api/schedule'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { Project } from '@/types/work'

// The baseline of a project: current version, earlier ones and setting it
// again with a reason (spec 009, RF-1 to RF-6).
const props = defineProps<{ project: Project }>()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const qc = useQueryClient()

const { data } = useQuery({ queryKey: computed(() => ['schedule', props.project.id]), queryFn: () => getProjectSchedule(props.project.id) })
const canSet = computed(() => props.project.can.manage && props.project.lifecycle === 'activo')
const asking = ref(false)
const reason = ref('')
const date = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(d))

const save = useMutation({
  mutationFn: () => setBaseline(props.project.id, reason.value.trim() || null),
  onSuccess: () => {
    asking.value = false
    reason.value = ''
    qc.invalidateQueries({ queryKey: ['schedule', props.project.id] })
    qc.invalidateQueries({ queryKey: ['overview', props.project.id] })
  },
})
function start() {
  // The first one needs no reason (RF-2).
  if (data.value?.baseline) asking.value = true
  else save.mutate()
}
</script>

<template>
  <section class="flex flex-col gap-2 rounded-xl border bg-background p-4 text-sm" aria-labelledby="ov-baseline" data-testid="baseline">
    <div class="flex flex-wrap items-center gap-2">
      <h2 id="ov-baseline" class="text-base font-semibold">{{ t('schedule.baseline.title') }}</h2>
      <Button v-if="canSet && !asking" size="sm" variant="outline" class="ml-auto" :disabled="save.isPending.value" data-testid="baseline-set" @click="start">
        {{ data?.baseline ? t('schedule.baseline.reset') : t('schedule.baseline.set') }}
      </Button>
    </div>
    <Alert v-if="save.error.value" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(save.error.value) }}</AlertDescription>
    </Alert>
    <template v-if="data">
      <p v-if="!data.baseline" class="text-muted-foreground">{{ t('schedule.baseline.none') }}</p>
      <template v-else>
        <p data-testid="baseline-current">
          {{ t('schedule.baseline.current', { version: data.baseline.version, date: date(data.baseline.created_at), author: data.baseline.author.name }) }}
        </p>
        <p class="text-muted-foreground">{{ data.baseline.reason || t('schedule.baseline.noReason') }}</p>
      </template>
      <form v-if="asking" class="flex flex-wrap items-center gap-2" @submit.prevent="save.mutate()">
        <Input
          v-model="reason"
          class="min-w-56 flex-1"
          maxlength="500"
          required
          :placeholder="t('schedule.baseline.reasonPlaceholder')"
          :aria-label="t('schedule.baseline.reason')"
          data-testid="baseline-reason"
        />
        <Button type="submit" size="sm" :disabled="!reason.trim() || save.isPending.value" data-testid="baseline-confirm">{{ t('schedule.baseline.confirm') }}</Button>
        <Button type="button" size="sm" variant="ghost" @click="asking = false">{{ t('common.cancel') }}</Button>
      </form>
      <details v-if="data.versions.length" class="text-muted-foreground">
        <summary class="cursor-pointer">{{ t('schedule.baseline.versions') }} ({{ data.versions.length }})</summary>
        <ul class="mt-1 flex flex-col gap-1" data-testid="baseline-versions">
          <li v-for="v in data.versions" :key="v.version">
            {{ t('schedule.baseline.current', { version: v.version, date: date(v.created_at), author: v.author.name }) }}
            — {{ v.reason || t('schedule.baseline.noReason') }}
          </li>
        </ul>
      </details>
    </template>
  </section>
</template>

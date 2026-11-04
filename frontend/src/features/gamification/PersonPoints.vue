<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adjustPoints, getPoints } from '@/api/gamification'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import LevelProgress from '@/features/gamification/LevelProgress.vue'
import PointsHistory from '@/features/gamification/PointsHistory.vue'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'

// A person's points for the manager: summary, history and a manual
// adjustment with its reason (spec 014, RF-18 to RF-23, RF-40).
const { t } = useI18n()
const auth = useAuthStore()
const client = useQueryClient()
const errorMessage = useErrorMessage()
const { users } = usePeople()

const userId = ref('')
const self = computed(() => userId.value === auth.me?.id)
const { data: points } = useQuery({
  queryKey: computed(() => ['points', 'user', userId.value]),
  queryFn: () => getPoints(userId.value),
  enabled: computed(() => !!userId.value),
})
const form = ref({ xp: 0, coins: 0, note: '' })
const error = ref<unknown>(null)
const done = ref(false)

async function submit() {
  error.value = null
  done.value = false
  try {
    await adjustPoints(userId.value, { xp: Number(form.value.xp), coins: Number(form.value.coins), note: form.value.note })
    form.value = { xp: 0, coins: 0, note: '' }
    done.value = true
    await client.invalidateQueries({ queryKey: ['points'] })
  } catch (e) {
    error.value = e
  }
}
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="person-points">
    <label class="flex max-w-xs flex-col gap-1 text-xs text-muted-foreground">
      {{ t('gamification.person.pick') }}
      <NativeSelect v-model="userId" data-testid="person-pick" @change="done = false">
        <option value="">{{ t('gamification.person.none') }}</option>
        <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
      </NativeSelect>
    </label>
    <template v-if="userId">
      <LevelProgress v-if="points" :points="points" class="max-w-md" />
      <form class="flex max-w-xl flex-col gap-2 rounded-lg border p-3" data-testid="adjust-form" @submit.prevent="submit">
        <p class="text-sm font-semibold">{{ t('gamification.person.adjust') }}</p>
        <p v-if="self" class="text-xs text-muted-foreground" data-testid="adjust-self">{{ t('errors.points_adjust_self') }}</p>
        <Alert v-if="error" variant="destructive" role="alert" data-testid="adjust-error">
          <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
        </Alert>
        <Alert v-if="done" role="status">
          <AlertDescription>{{ t('gamification.person.done') }}</AlertDescription>
        </Alert>
        <div class="flex flex-wrap gap-2">
          <label class="flex flex-col gap-1 text-xs text-muted-foreground">
            {{ t('gamification.history.xp') }}
            <Input v-model.number="form.xp" type="number" class="h-8 w-28" data-testid="adjust-xp" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-muted-foreground">
            {{ t('gamification.history.coins') }}
            <Input v-model.number="form.coins" type="number" class="h-8 w-28" data-testid="adjust-coins" />
          </label>
          <label class="flex min-w-48 flex-1 flex-col gap-1 text-xs text-muted-foreground">
            {{ t('gamification.person.note') }}
            <Input v-model="form.note" required minlength="3" maxlength="500" class="h-8" data-testid="adjust-note" />
          </label>
        </div>
        <div>
          <Button type="submit" :disabled="self" data-testid="adjust-submit">{{ t('gamification.person.submit') }}</Button>
        </div>
      </form>
      <PointsHistory :user-id="userId" />
    </template>
  </div>
</template>

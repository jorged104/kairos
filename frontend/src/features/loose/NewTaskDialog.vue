<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { User, Users, X } from '@lucide/vue'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { listTeams } from '@/api/admin'
import { getDashboardPeople } from '@/api/dashboard'
import { createLoose, type LooseMode } from '@/api/loose'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { flatten, ledScope } from '@/features/teams/tree'
import { useCatalog } from '@/features/work/useCatalog'
import { useAuthStore } from '@/stores/auth'

// «Nueva tarea» without a project (spec 022, RF-1 to RF-9): for oneself, for
// a person one leads, or for a team, where one person takes it or each member
// does their own. The new task opens in the side panel.
const props = defineProps<{ team?: boolean }>()
const open = defineModel<boolean>({ default: false })
const { t } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const { catalog } = useCatalog()

const all = computed(() => auth.can('loose:assign:all'))
const leads = computed(() => auth.can('loose:assign:team'))
const others = computed(() => all.value || leads.value)
const { data: people } = useQuery({ queryKey: ['dashboard', 'people'], queryFn: getDashboardPeople, enabled: computed(() => open.value && others.value) })
const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams, enabled: computed(() => open.value && others.value) })
// The teams one may give a task to (RF-5).
const teamOptions = computed(() => {
  const list = teams.value ?? []
  const scope = all.value ? null : ledScope(list, auth.me?.id ?? '')
  return flatten(list.filter((tm) => !tm.archived_at && (!scope || scope.has(tm.id))))
})

const title = ref('')
const to = ref<'person' | 'team'>('person')
const userId = ref('')
const teamId = ref('')
const mode = ref<LooseMode>('claim')
const due = ref('')
const weight = ref('')
const priority = ref('')
const error = ref<unknown>(null)
const busy = ref(false)
const titleEl = ref<HTMLInputElement | null>(null)

watch(open, async (v) => {
  if (!v) return
  title.value = due.value = weight.value = priority.value = teamId.value = ''
  userId.value = auth.me?.id ?? ''
  to.value = props.team ? 'team' : 'person'
  mode.value = 'claim'
  error.value = null
  await nextTick()
  titleEl.value?.focus()
})

const submitLabel = computed(() => t(`loose.submit.${to.value === 'person' ? 'person' : mode.value}`))
const ready = computed(() => title.value.trim() && (to.value === 'person' ? !!userId.value : !!teamId.value))

async function submit() {
  if (!ready.value || busy.value) return
  busy.value = true
  error.value = null
  try {
    const created = await createLoose({
      title: title.value.trim(),
      due_date: due.value || null,
      weight: weight.value === '' ? null : Number(weight.value),
      priority_id: priority.value || null,
      ...(to.value === 'person' ? { user_id: userId.value } : { team_id: teamId.value, mode: mode.value }),
    })
    for (const k of [['dashboard'], ['claimable'], ['team-tasks'], ['my-team']]) void qc.invalidateQueries({ queryKey: k })
    open.value = false
    void router.replace({ query: { ...route.query, item: created.key } })
  } catch (e) {
    error.value = e
  } finally {
    busy.value = false
  }
}

const tab = (on: boolean) =>
  on ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
const card = (on: boolean) => (on ? 'border-foreground ring-1 ring-foreground bg-muted/40' : 'border-border hover:bg-muted/30')
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/40 p-4 sm:p-10" data-testid="loose-dialog-backdrop" @click.self="open = false">
      <form
        role="dialog"
        aria-modal="true"
        aria-labelledby="loose-title"
        class="flex w-full max-w-xl flex-col gap-5 rounded-2xl border bg-background p-6 shadow-xl"
        data-testid="loose-dialog"
        @submit.prevent="submit"
        @keydown.esc.prevent="open = false"
      >
        <div class="flex items-start gap-2">
          <div class="flex flex-col gap-1">
            <h2 id="loose-title" class="text-lg font-semibold">{{ t('loose.newTask') }}</h2>
            <p class="text-sm text-muted-foreground">{{ t('loose.dialogHint') }}</p>
          </div>
          <Button type="button" size="icon" variant="ghost" class="ml-auto" :aria-label="t('common.close')" @click="open = false"><X aria-hidden="true" /></Button>
        </div>

        <label class="flex flex-col gap-1.5 text-sm font-medium">
          {{ t('loose.title') }}
          <input ref="titleEl" v-model="title" required maxlength="500" class="h-10 rounded-md border bg-background px-3 text-sm font-normal" data-testid="loose-title" />
        </label>

        <div v-if="others" class="flex flex-col gap-1.5">
          <span class="text-sm font-medium">{{ t('loose.to') }}</span>
          <div role="group" :aria-label="t('loose.to')" class="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1">
            <button type="button" :aria-pressed="to === 'person'" class="flex h-9 items-center justify-center gap-2 rounded-md text-sm font-medium" :class="tab(to === 'person')" data-testid="loose-to-person" @click="to = 'person'">
              <User class="size-4" aria-hidden="true" />{{ t('loose.toPerson') }}
            </button>
            <button type="button" :aria-pressed="to === 'team'" class="flex h-9 items-center justify-center gap-2 rounded-md text-sm font-medium" :class="tab(to === 'team')" data-testid="loose-to-team" @click="to = 'team'">
              <Users class="size-4" aria-hidden="true" />{{ t('loose.toTeam') }}
            </button>
          </div>
        </div>

        <div v-if="to === 'person' && others" class="flex flex-col gap-1.5">
          <label for="loose-person" class="text-sm font-medium">{{ t('loose.person') }}</label>
          <NativeSelect id="loose-person" v-model="userId" class="h-10" data-testid="loose-person">
            <option :value="auth.me?.id ?? ''">{{ t('loose.me') }}</option>
            <option v-for="p in people ?? []" :key="p.id" :value="p.id">{{ p.name }}</option>
          </NativeSelect>
          <p class="text-xs text-muted-foreground">{{ t('loose.personHint') }}</p>
        </div>

        <template v-if="to === 'team'">
          <div class="flex flex-col gap-1.5">
            <label for="loose-team" class="text-sm font-medium">{{ t('loose.team') }}</label>
            <NativeSelect id="loose-team" v-model="teamId" class="h-10" data-testid="loose-team">
              <option value="">{{ t('loose.chooseTeam') }}</option>
              <option v-for="x in teamOptions" :key="x.team.id" :value="x.team.id">{{ ' '.repeat(x.depth * 2) }}{{ x.team.name }}</option>
            </NativeSelect>
          </div>
          <fieldset class="flex flex-col gap-2">
            <legend class="mb-1.5 text-sm font-medium">{{ t('loose.who') }}</legend>
            <label v-for="m in ['claim', 'each'] as const" :key="m" class="flex cursor-pointer gap-3 rounded-lg border p-3" :class="card(mode === m)">
              <input v-model="mode" type="radio" name="loose-mode" :value="m" class="mt-0.5 size-4 accent-primary" :data-testid="`loose-mode-${m}`" />
              <span class="flex flex-col gap-0.5">
                <span class="text-sm font-semibold">{{ t(`loose.modes.${m}`) }}</span>
                <span class="text-sm text-muted-foreground">{{ t(`loose.modeHints.${m}`) }}</span>
              </span>
            </label>
          </fieldset>
        </template>

        <div class="grid gap-3 sm:grid-cols-3">
          <label class="flex flex-col gap-1.5 text-sm font-medium">
            {{ t('loose.due') }}
            <input v-model="due" type="date" class="h-10 rounded-md border bg-background px-3 text-sm font-normal" data-testid="loose-due" />
          </label>
          <label class="flex flex-col gap-1.5 text-sm font-medium">
            {{ t('loose.weight') }}
            <input v-model="weight" type="number" min="0" max="100" class="h-10 rounded-md border bg-background px-3 text-sm font-normal" data-testid="loose-weight" />
          </label>
          <label class="flex flex-col gap-1.5 text-sm font-medium">
            {{ t('loose.priority') }}
            <NativeSelect v-model="priority" class="h-10 font-normal" data-testid="loose-priority">
              <option value="">{{ t('loose.defaultPriority') }}</option>
              <option v-for="p in catalog.priorities.filter((x) => !x.retired)" :key="p.id" :value="p.id">{{ p.name }}</option>
            </NativeSelect>
          </label>
        </div>

        <p class="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">{{ t('loose.pointsHint') }}</p>

        <Alert v-if="error" variant="destructive" role="alert">
          <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
        </Alert>

        <div class="flex justify-end gap-2">
          <Button type="button" variant="outline" @click="open = false">{{ t('common.cancel') }}</Button>
          <Button type="submit" :disabled="!ready || busy" data-testid="loose-submit">{{ submitLabel }}</Button>
        </div>
      </form>
    </div>
  </Teleport>
</template>

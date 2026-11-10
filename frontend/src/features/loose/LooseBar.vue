<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { claimTask, moveToProject, releaseTask } from '@/api/loose'
import { listProjects } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import type { WorkItem } from '@/types/work'

// What a work item without a project adds to its page: whose it is, the
// progress of its team, take it, give it back, or move it to a project
// (spec 022, RF-19 to RF-24, RF-37).
const props = defineProps<{ item: WorkItem }>()
const emit = defineEmits<{ changed: [WorkItem]; error: [unknown] }>()
const { t } = useI18n()
const qc = useQueryClient()

const loose = computed(() => props.item.loose!)
const label = computed(() =>
  loose.value.team ? t('loose.teamChip', { team: loose.value.team.name, mode: t(`loose.modes.${loose.value.mode}`) }) : t('loose.noProject'),
)
const pct = computed(() => (loose.value.total ? Math.round((loose.value.done / loose.value.total) * 100) : 0))

const moving = ref(false)
const target = ref('')
const { data: projects } = useQuery({
  queryKey: ['projects', 'to-move'],
  queryFn: () => listProjects({ lifecycle: 'activo', page_size: 100 }),
  enabled: moving,
})
const writable = computed(() => (projects.value?.items ?? []).filter((p) => p.can.write_work_items))

const busy = ref(false)
async function run(fn: () => Promise<WorkItem>) {
  busy.value = true
  try {
    const w = await fn()
    for (const k of [['dashboard'], ['claimable'], ['team-tasks'], ['my-team']]) void qc.invalidateQueries({ queryKey: k })
    emit('changed', w)
  } catch (e) {
    emit('error', e)
  } finally {
    busy.value = false
  }
}
const claim = () => run(() => claimTask(props.item.id))
const release = () => run(() => releaseTask(props.item.id))
const move = () => target.value && run(() => moveToProject(props.item.id, target.value, props.item.version))
</script>

<template>
  <section class="flex flex-col gap-3 rounded-lg border border-dashed bg-muted/30 p-3" data-testid="loose-bar">
    <div class="flex flex-wrap items-center gap-2">
      <span class="rounded-full border bg-background px-2.5 py-0.5 text-xs font-medium" data-testid="loose-label">{{ label }}</span>
      <template v-if="loose.mode === 'each'">
        <span class="text-sm text-muted-foreground">{{ t('loose.progressLabel') }}</span>
        <span class="text-sm font-semibold" data-testid="loose-progress">{{ t('loose.progress', { done: loose.done, total: loose.total }) }}</span>
        <span class="h-1.5 w-24 overflow-hidden rounded-full bg-muted" aria-hidden="true"><span class="block h-full rounded-full bg-primary" :style="{ width: `${pct}%` }" /></span>
      </template>
      <div class="ml-auto flex flex-wrap items-center gap-2">
        <Button v-if="loose.can.claim" size="sm" :disabled="busy" data-testid="loose-claim" @click="claim">{{ t('loose.claim') }}</Button>
        <Button v-if="loose.can.release" size="sm" variant="outline" :disabled="busy" data-testid="loose-release" @click="release">{{ t('loose.release') }}</Button>
        <Button v-if="loose.can.move && !moving" size="sm" variant="outline" data-testid="loose-move" @click="moving = true">{{ t('loose.move') }}</Button>
      </div>
    </div>
    <form v-if="moving" class="flex flex-wrap items-center gap-2" @submit.prevent="move">
      <NativeSelect v-model="target" class="h-9 min-w-56 flex-1" :aria-label="t('loose.moveChoose')" data-testid="loose-move-project">
        <option value="">{{ t('loose.moveChoose') }}</option>
        <option v-for="p in writable" :key="p.id" :value="p.id">{{ p.identifier }} · {{ p.name }}</option>
      </NativeSelect>
      <Button type="submit" size="sm" :disabled="!target || busy" data-testid="loose-move-do">{{ t('loose.moveDo') }}</Button>
      <Button type="button" size="sm" variant="ghost" @click="moving = false">{{ t('common.cancel') }}</Button>
    </form>
  </section>
</template>

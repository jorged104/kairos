<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, CheckCircle2, Circle, CircleDot, Copy, Loader2, Sparkles, Undo2 } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import {
  activeReportTemplates,
  createReport,
  generateReport,
  getReport,
  previewReport,
  updateReport,
  type GenerateBody,
  type ReportCan,
  type ReportEvent,
  type ReportOrigin,
} from '@/api/reports'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import RichTextView from '@/features/richtext/RichTextView.vue'
import type { Project, RichDoc } from '@/types/work'

// Writing a report: a new one generated from a template, or a saved one
// changed by asking the AI or by hand, saved over the last version
// (spec 020, RF-35 to RF-44, RF-54 to RF-67, RF-71 to RF-77).
const props = defineProps<{ project: Project; reportId?: string; can: ReportCan }>()
const { t, d } = useI18n()
const route = useRoute()
const router = useRouter()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

type Step = 'setup' | 'generating' | 'editing'
const step = ref<Step>(props.reportId ? 'editing' : 'setup')
const error = ref<unknown>(null)
const conflict = ref(false)

// The draft and what was saved, to know whether there is something to lose.
const title = ref('')
const content = ref('')
const version = ref(0)
const templateId = ref<string | null>(null)
const saved = ref({ title: '', content: '' })
const dirty = computed(() => title.value !== saved.value.title || content.value !== saved.value.content)

// Where the content came from, for the audit (RF-70).
const aiOutput = ref<string | null>(null)
const aiUsed = ref(false)
const origin = computed<ReportOrigin>(() => (!aiUsed.value ? 'manual' : content.value === aiOutput.value ? 'ai' : 'ai_edited'))

const { data: existing } = useQuery({
  queryKey: computed(() => ['report', props.reportId]),
  queryFn: () => getReport(String(props.reportId)),
  enabled: !!props.reportId,
})
function load() {
  const r = existing.value
  if (!r) return
  title.value = r.title
  content.value = r.content
  version.value = r.version
  templateId.value = r.template_id
  saved.value = { title: r.title, content: r.content }
}
watch(existing, (r, before) => {
  if (r && !before) load()
}, { immediate: true })

// A new report: a template and optional instructions (RF-35, RF-36).
const { data: templates } = useQuery({ queryKey: ['report-templates', 'active'], queryFn: activeReportTemplates, enabled: !props.reportId })
const form = ref({ templateId: '', instructions: '' })
watch(templates, (x) => {
  if (x && !form.value.templateId) form.value.templateId = x.items[0]?.id ?? ''
}, { immediate: true })

// What happens while the AI works (RF-37 to RF-40).
type Todo = Extract<ReportEvent, { type: 'todos' }>['todos'][number]
const todos = ref<Todo[]>([])
const tool = ref<{ tool: string; project?: string } | null>(null)
const live = ref('')
let controller: AbortController | null = null

async function stream(body: GenerateBody): Promise<{ text: string; failure: unknown }> {
  todos.value = []
  tool.value = null
  live.value = ''
  error.value = null
  step.value = 'generating'
  controller = new AbortController()
  let finished = false
  let failure: unknown = null
  try {
    await generateReport(
      props.project.id,
      body,
      (e) => {
        if (e.type === 'todos') todos.value = e.todos
        else if (e.type === 'tool') tool.value = e
        else if (e.type === 'text') live.value += e.delta
        else if (e.type === 'reset') live.value = ''
        else if (e.type === 'done') finished = true
        else if (e.type === 'error') failure = e.code
      },
      controller.signal,
    )
  } catch (e) {
    failure = e
  }
  controller = null
  if (!finished && !failure) failure = 'network_error'
  return { text: live.value, failure }
}

async function generate() {
  const tpl = templates.value?.items.find((x) => x.id === form.value.templateId)
  if (!tpl) return
  const { text, failure } = await stream({ template_id: tpl.id, instructions: form.value.instructions.trim() })
  error.value = failure
  if (!text.trim()) {
    step.value = 'setup' // nothing to keep (RF-76)
    return
  }
  // What arrived is kept as a draft even if it failed (RF-74).
  content.value = text
  aiOutput.value = text
  aiUsed.value = true
  templateId.value = tpl.id
  if (!title.value) title.value = `${tpl.name} · ${d(new Date(), 'long').split(',')[0]}` // RF-41
  step.value = 'editing'
}

function cancel() {
  controller?.abort()
}

// A change asked to the AI over the current draft, which can be undone
// while not saved (RF-56 to RF-61); a failed one keeps the draft (RF-75).
const change = ref('')
const undo = ref<string[]>([])
async function askChange() {
  const before = content.value
  const { text, failure } = await stream({ draft: before, change: change.value.trim(), template_id: templateId.value })
  step.value = 'editing'
  if (failure || !text.trim()) {
    error.value = failure ?? 'ai_empty'
    content.value = before
    return
  }
  undo.value.push(before)
  content.value = text
  aiOutput.value = text
  aiUsed.value = true
  change.value = ''
}
function undoChange() {
  const prev = undo.value.pop()
  if (prev !== undefined) content.value = prev
}

// Editing by hand, with a preview as it will be shown (RF-31, RF-54).
const mode = ref<'write' | 'preview'>('write')
const preview = ref<RichDoc>(null)
watch(mode, async (m) => {
  if (m !== 'preview') return
  try {
    preview.value = (await previewReport(content.value)).doc
  } catch (e) {
    error.value = e
  }
})

// Saving (RF-62 to RF-67).
const saving = ref(false)
const leaving = ref(false)
const valid = computed(() => title.value.trim() && title.value.length <= 200 && content.value.trim() && content.value.length <= 200000)
async function save() {
  if (!valid.value) return
  saving.value = true
  error.value = null
  conflict.value = false
  try {
    const body = { title: title.value.trim(), content: content.value, origin: origin.value }
    const r = props.reportId
      ? await updateReport(props.reportId, { ...body, version: version.value })
      : await createReport(props.project.id, { ...body, template_id: templateId.value })
    saved.value = { title: r.title, content: r.content }
    title.value = r.title
    version.value = r.version
    undo.value = []
    qc.setQueryData(['report', r.id], r)
    await qc.invalidateQueries({ queryKey: ['reports', props.project.id] })
    leaving.value = true
    await router.replace({ query: { ...route.query, report: r.id, edit: undefined } })
  } catch (e) {
    error.value = e
    conflict.value = e instanceof ApiError && e.code === 'version_conflict'
  } finally {
    saving.value = false
  }
}

// Someone else saved first: keep a copy of the own text, then reload (RF-64, RF-65).
const copied = ref(false)
async function copyText() {
  await navigator.clipboard?.writeText(content.value)
  copied.value = true
}
async function reload() {
  await qc.invalidateQueries({ queryKey: ['report', props.reportId] })
  const r = await qc.fetchQuery({ queryKey: ['report', props.reportId], queryFn: () => getReport(String(props.reportId)) })
  title.value = r.title
  content.value = r.content
  version.value = r.version
  saved.value = { title: r.title, content: r.content }
  aiUsed.value = false
  aiOutput.value = null
  undo.value = []
  conflict.value = false
  error.value = null
}

// Leaving with unsaved work asks first (RF-43); a running generation stops.
const mayLeave = () => leaving.value || (!dirty.value && step.value !== 'generating') || window.confirm(t('reports.leave'))
onBeforeRouteLeave(() => {
  if (!mayLeave()) return false
  cancel()
})
onBeforeRouteUpdate((to) => {
  if (to.query.report === route.query.report && to.query.edit === route.query.edit) return
  if (!mayLeave()) return false
  cancel()
})
const onBeforeUnload = (e: BeforeUnloadEvent) => {
  if (dirty.value || step.value === 'generating') e.preventDefault()
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', onBeforeUnload)
  cancel()
})

const back = computed(() => ({ query: { ...route.query, report: props.reportId ?? undefined, edit: undefined } }))
const toolLabel = computed(() => {
  if (!tool.value) return t('reports.stream.thinking')
  const key = `reports.tools.${tool.value.tool}`
  return t(key, { project: tool.value.project || t('reports.stream.aProject') })
})
const todoIcon = { completed: CheckCircle2, in_progress: CircleDot, pending: Circle }
</script>

<template>
  <section class="flex flex-col gap-4" data-testid="report-editor">
    <div class="flex flex-wrap items-center gap-2">
      <Button variant="ghost" size="sm" :disabled="step === 'generating'" data-testid="report-back" @click="router.push(back)">
        <ArrowLeft aria-hidden="true" />{{ t('reports.back') }}
      </Button>
      <h2 class="font-heading text-lg font-semibold">{{ reportId ? t('reports.editTitle') : t('reports.newTitle') }}</h2>
    </div>

    <Alert v-if="error" variant="destructive" role="alert" data-testid="report-error">
      <AlertDescription class="flex flex-wrap items-center gap-2">
        {{ errorMessage(error) }}
        <template v-if="conflict">
          <Button size="sm" variant="outline" data-testid="report-copy" @click="copyText">
            <Copy aria-hidden="true" />{{ copied ? t('reports.copied') : t('reports.copy') }}
          </Button>
          <Button size="sm" variant="outline" data-testid="report-reload" @click="reload">{{ t('work.reload') }}</Button>
        </template>
      </AlertDescription>
    </Alert>

    <!-- 1. Template and instructions (RF-35, RF-36). -->
    <form v-if="step === 'setup'" class="flex max-w-2xl flex-col gap-4 rounded-lg border p-5" data-testid="report-setup" @submit.prevent="generate">
      <label class="flex flex-col gap-1 text-sm">
        {{ t('reports.template') }}
        <NativeSelect v-model="form.templateId" required data-testid="report-template">
          <option v-for="x in templates?.items ?? []" :key="x.id" :value="x.id">{{ x.name }}</option>
        </NativeSelect>
        <span class="text-xs text-muted-foreground">{{ templates?.items.find((x) => x.id === form.templateId)?.description }}</span>
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('reports.instructions') }}
        <textarea
          v-model="form.instructions"
          rows="3"
          maxlength="2000"
          :placeholder="t('reports.instructionsHint')"
          class="rounded-md border bg-background px-3 py-2 text-sm"
          data-testid="report-instructions"
        />
      </label>
      <p class="text-xs text-muted-foreground">{{ t('reports.scope', { project: project.name }) }}</p>
      <div>
        <Button type="submit" :disabled="!form.templateId" data-testid="report-generate"><Sparkles aria-hidden="true" />{{ t('reports.generate') }}</Button>
      </div>
    </form>

    <!-- 2. The AI at work: its plan, the query it is making and the text (RF-37 to RF-40). -->
    <div v-else-if="step === 'generating'" class="grid gap-4 md:grid-cols-[16rem_1fr]" data-testid="report-generating" aria-live="polite">
      <aside class="flex flex-col gap-3 rounded-lg border p-4 text-sm">
        <p class="flex items-center gap-2 font-medium">
          <Loader2 class="size-4 animate-spin" aria-hidden="true" />
          <span data-testid="report-tool">{{ toolLabel }}</span>
        </p>
        <ol v-if="todos.length" class="flex flex-col gap-1.5" data-testid="report-todos">
          <li v-for="(x, i) in todos" :key="i" class="flex items-start gap-2" :class="x.status === 'completed' && 'text-muted-foreground line-through'">
            <component :is="todoIcon[x.status] ?? Circle" class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
            <span>{{ x.content }}</span>
          </li>
        </ol>
        <Button variant="outline" size="sm" data-testid="report-cancel" @click="cancel">{{ t('common.cancel') }}</Button>
      </aside>
      <pre class="min-h-48 whitespace-pre-wrap rounded-lg border bg-muted/30 p-4 font-sans text-sm" data-testid="report-live">{{ live || t('reports.stream.waiting') }}</pre>
    </div>

    <!-- 3. The draft: by hand, by asking the AI, and saved (RF-54 to RF-67). -->
    <template v-else>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('reports.title') }}
        <Input v-model="title" maxlength="200" required data-testid="report-title-input" />
      </label>
      <div class="flex gap-1 border-b text-sm" role="tablist">
        <button
          v-for="m in ['write', 'preview'] as const"
          :key="m"
          type="button"
          role="tab"
          :aria-selected="mode === m"
          class="-mb-px border-b-2 px-3 py-1.5"
          :class="mode === m ? 'border-primary font-medium' : 'border-transparent text-muted-foreground hover:text-foreground'"
          :data-testid="`report-mode-${m}`"
          @click="mode = m"
        >
          {{ t(`reports.modes.${m}`) }}
        </button>
      </div>
      <textarea
        v-if="mode === 'write'"
        v-model="content"
        rows="22"
        maxlength="200000"
        :aria-label="t('reports.content')"
        class="rounded-md border bg-background px-3 py-2 font-mono text-sm"
        data-testid="report-content-input"
      />
      <div v-else class="min-h-48 rounded-lg border p-5" data-testid="report-preview"><RichTextView :doc="preview" /></div>

      <form v-if="can.generate" class="flex flex-col gap-2 rounded-lg border p-4" data-testid="report-change" @submit.prevent="askChange">
        <label class="flex flex-col gap-1 text-sm">
          {{ t('reports.change') }}
          <textarea
            v-model="change"
            rows="2"
            maxlength="2000"
            :placeholder="t('reports.changeHint')"
            class="rounded-md border bg-background px-3 py-2 text-sm"
            data-testid="report-change-input"
          />
        </label>
        <div class="flex flex-wrap gap-2">
          <Button type="submit" size="sm" variant="outline" :disabled="!change.trim() || !content.trim()" data-testid="report-change-send">
            <Sparkles aria-hidden="true" />{{ t('reports.changeSend') }}
          </Button>
          <Button v-if="undo.length" type="button" size="sm" variant="ghost" data-testid="report-undo" @click="undoChange">
            <Undo2 aria-hidden="true" />{{ t('reports.undo') }}
          </Button>
        </div>
      </form>

      <div class="flex flex-wrap items-center gap-2">
        <Button :disabled="saving || !valid || (!dirty && !!reportId)" data-testid="report-save" @click="save">{{ t('common.save') }}</Button>
        <span v-if="dirty" class="text-xs text-muted-foreground">{{ t('reports.unsaved') }}</span>
      </div>
    </template>
  </section>
</template>

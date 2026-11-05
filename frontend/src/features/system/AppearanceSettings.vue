<script setup lang="ts">
import { useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAppearance, type SystemSettings } from '@/api/system'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { DEFAULT_THEME, PALETTES, PRIMARIES, PRIMARY_TONES, RADII, themeVariables, type Theme } from '@/lib/theme'

// Palette, primary colour and corners, with a preview in light and dark
// before saving (spec 015, RF-39 to RF-47).
const props = defineProps<{ settings: SystemSettings }>()
const emit = defineEmits<{ saved: [SystemSettings] }>()
const { t } = useI18n()
const client = useQueryClient()
const errorMessage = useErrorMessage()

const draft = ref<Theme>({ ...props.settings.appearance })
watch(() => props.settings.appearance, (a) => (draft.value = { ...a }))
const error = ref<unknown>(null)
const saved = ref(false)

// The preview sets the variables of the draft on its own box.
const vars = computed(() => themeVariables(draft.value))
const style = (v: Record<string, string>) => Object.fromEntries(Object.entries(v).map(([k, x]) => [`--${k}`, x]))
const swatch = (p: (typeof PRIMARIES)[number]) =>
  p === 'default' ? themeVariables({ ...draft.value, primary: 'default' }).light.primary : `oklch(${PRIMARY_TONES[p].light[0]})`

async function save(theme: Theme) {
  error.value = null
  saved.value = false
  try {
    emit('saved', await saveAppearance(theme))
    saved.value = true
    // Everybody's screens, this one included, read it from the public config (RF-45).
    await client.invalidateQueries({ queryKey: ['auth', 'config'] })
  } catch (e) {
    error.value = e
  }
}
function reset() {
  draft.value = { ...DEFAULT_THEME }
  void save(draft.value)
}
</script>

<template>
  <div class="flex flex-col gap-5" data-testid="appearance-settings">
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <Alert v-if="saved" role="status">
      <AlertDescription>{{ t('system.appearance.saved') }}</AlertDescription>
    </Alert>
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 text-sm font-medium">{{ t('system.appearance.palette') }}</legend>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="p in PALETTES"
          :key="p"
          type="button"
          class="rounded-lg border px-3 py-1.5 text-sm"
          :class="draft.palette === p ? 'border-primary ring-2 ring-primary/30' : 'hover:bg-muted'"
          :aria-pressed="draft.palette === p"
          :data-testid="`palette-${p}`"
          @click="draft.palette = p"
        >
          {{ t(`system.appearance.palettes.${p}`) }}
        </button>
      </div>
    </fieldset>
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 text-sm font-medium">{{ t('system.appearance.primary') }}</legend>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="p in PRIMARIES"
          :key="p"
          type="button"
          class="flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm"
          :class="draft.primary === p ? 'border-primary ring-2 ring-primary/30' : 'hover:bg-muted'"
          :aria-pressed="draft.primary === p"
          :data-testid="`primary-${p}`"
          @click="draft.primary = p"
        >
          <span class="size-3.5 rounded-full border" :style="{ background: swatch(p) }" aria-hidden="true" />
          {{ t(`system.appearance.primaries.${p}`) }}
        </button>
      </div>
    </fieldset>
    <fieldset class="flex flex-col gap-2">
      <legend class="mb-1 text-sm font-medium">{{ t('system.appearance.radius') }}</legend>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="r in RADII"
          :key="r"
          type="button"
          class="rounded-lg border px-3 py-1.5 text-sm"
          :class="draft.radius === r ? 'border-primary ring-2 ring-primary/30' : 'hover:bg-muted'"
          :aria-pressed="draft.radius === r"
          :data-testid="`radius-${r}`"
          @click="draft.radius = r"
        >
          {{ t(`system.appearance.radii.${r}`) }}
        </button>
      </div>
    </fieldset>

    <div class="grid gap-3 md:grid-cols-2" data-testid="appearance-preview">
      <div
        v-for="variant in ['light', 'dark'] as const"
        :key="variant"
        :class="variant === 'dark' && 'dark'"
        :style="style({ ...vars.light, ...(variant === 'dark' ? vars.dark : {}) })"
        :data-testid="`preview-${variant}`"
      >
        <div class="flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-background p-4 text-foreground">
          <p class="text-xs font-medium text-muted-foreground">{{ t(`system.appearance.${variant}`) }}</p>
          <div class="rounded-[var(--radius)] border border-border bg-card p-3 text-card-foreground">
            <p class="text-sm font-semibold">{{ t('system.appearance.sampleTitle') }}</p>
            <p class="text-xs text-muted-foreground">{{ t('system.appearance.sampleText') }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <span class="rounded-[var(--radius)] bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground">{{ t('system.appearance.samplePrimary') }}</span>
            <span class="rounded-[var(--radius)] bg-secondary px-3 py-1.5 text-sm text-secondary-foreground">{{ t('system.appearance.sampleSecondary') }}</span>
            <span class="rounded-[var(--radius)] border border-input px-3 py-1.5 text-sm">{{ t('system.appearance.sampleOutline') }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="flex gap-2">
      <Button type="button" data-testid="appearance-save" @click="save(draft)">{{ t('common.save') }}</Button>
      <Button type="button" variant="outline" data-testid="appearance-reset" @click="reset">{{ t('system.appearance.reset') }}</Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MentionRefs } from '@/types/work'

// A mention as it reads now (RF-27 to RF-29). The refs come from the API, so
// a renamed element shows its new name and a hidden one never shows its title.
const props = defineProps(nodeViewProps)
const { t } = useI18n()

const attrs = computed(() => props.node.attrs as { id: string; kind: string; label?: string })
const refs = computed(() => (props.extension.options.refs as () => MentionRefs)())
const ref = computed(() => refs.value[attrs.value.id])
const missing = computed(() => ref.value?.state === 'no_access' || ref.value?.state === 'deleted')
const label = computed(() => {
  if (ref.value?.state === 'no_access') return t('page.mention.noAccess')
  if (ref.value?.state === 'deleted') return t('page.mention.deleted')
  return (attrs.value.kind === 'user' ? '@' : '#') + (ref.value?.label ?? attrs.value.label ?? '')
})
const href = computed(() => {
  if (missing.value) return null
  const text = ref.value?.label ?? attrs.value.label
  switch (attrs.value.kind) {
    case 'work_item':
      return text ? `/work-items/${text.split(' ')[0]}` : null
    case 'milestone':
      return `/milestones/${attrs.value.id}`
    case 'project':
      return `/projects/${attrs.value.id}`
  }
  return null
})
</script>

<template>
  <NodeViewWrapper as="span" class="mention" :class="{ 'mention-missing': missing }" :data-state="ref?.state ?? 'ok'">
    <a v-if="href" :href="href">{{ label }}</a>
    <template v-else>{{ label }}</template>
  </NodeViewWrapper>
</template>

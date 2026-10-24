<script setup lang="ts">
import { EditorContent, useEditor } from '@tiptap/vue-3'
import { watch } from 'vue'
import type { RichDoc } from '@/types/work'
import { richTextExtensions } from './extensions'

// Renders a stored document read-only through TipTap: never as raw HTML.
const props = defineProps<{ doc: RichDoc }>()

const editor = useEditor({ extensions: richTextExtensions(), editable: false, content: props.doc ?? '' })
watch(
  () => props.doc,
  (doc) => editor.value?.commands.setContent(doc ?? ''),
)
</script>

<template>
  <EditorContent v-if="doc" :editor="editor" class="rich-text text-sm" />
</template>

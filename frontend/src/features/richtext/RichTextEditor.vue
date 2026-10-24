<script setup lang="ts">
import { EditorContent, useEditor } from '@tiptap/vue-3'
import { Bold, Code, Heading2, Italic, Link, List, ListOrdered, Table } from '@lucide/vue'
import { onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RichDoc } from '@/types/work'
import { SAFE_LINK, richTextExtensions } from './extensions'

const { t } = useI18n()
const model = defineModel<RichDoc>({ default: null })

const editor = useEditor({
  extensions: richTextExtensions(),
  content: model.value ?? '',
  onUpdate: ({ editor }) => {
    model.value = editor.isEmpty ? null : (editor.getJSON() as RichDoc)
  },
})
watch(model, (doc) => {
  const current = editor.value?.isEmpty ? null : editor.value?.getJSON()
  if (JSON.stringify(current ?? null) !== JSON.stringify(doc ?? null)) editor.value?.commands.setContent(doc ?? '')
})
onBeforeUnmount(() => editor.value?.destroy())

function setLink() {
  const url = window.prompt(t('richtext.linkPrompt'))
  if (url === null) return
  const chain = editor.value?.chain().focus().extendMarkRange('link')
  if (!url) chain?.unsetLink().run()
  else if (SAFE_LINK.test(url)) chain?.setLink({ href: url }).run()
}

const tools = [
  { icon: Bold, label: 'richtext.bold', run: () => editor.value?.chain().focus().toggleBold().run() },
  { icon: Italic, label: 'richtext.italic', run: () => editor.value?.chain().focus().toggleItalic().run() },
  { icon: Heading2, label: 'richtext.heading', run: () => editor.value?.chain().focus().toggleHeading({ level: 2 }).run() },
  { icon: List, label: 'richtext.bulletList', run: () => editor.value?.chain().focus().toggleBulletList().run() },
  { icon: ListOrdered, label: 'richtext.orderedList', run: () => editor.value?.chain().focus().toggleOrderedList().run() },
  { icon: Code, label: 'richtext.codeBlock', run: () => editor.value?.chain().focus().toggleCodeBlock().run() },
  { icon: Link, label: 'richtext.link', run: setLink },
  {
    icon: Table,
    label: 'richtext.table',
    run: () => editor.value?.chain().focus().insertTable({ rows: 2, cols: 2, withHeaderRow: true }).run(),
  },
]
</script>

<template>
  <div class="rounded-lg border bg-background">
    <div class="flex flex-wrap gap-1 border-b p-1" role="toolbar">
      <button
        v-for="tool in tools"
        :key="tool.label"
        type="button"
        class="rounded p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground"
        :title="t(tool.label)"
        :aria-label="t(tool.label)"
        @click="tool.run"
      >
        <component :is="tool.icon" class="size-4" />
      </button>
    </div>
    <EditorContent :editor="editor" class="rich-text min-h-24 p-2 text-sm" />
  </div>
</template>

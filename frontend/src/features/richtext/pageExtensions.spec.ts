import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { Editor, type JSONContent } from '@tiptap/vue-3'
import { afterEach, describe, expect, it } from 'vitest'
import { defineComponent } from 'vue'
import { CLIENT_ONLY_NODES, PAGE_NODES, pageExtensions, pageForSave, type PageContext } from './pageExtensions'

const ctx: PageContext = {
  t: (key) => key,
  refs: () => ({}),
  searchPeople: async () => [],
  searchElements: async () => [],
  pickFiles: () => {},
  upload: () => {},
  uploadView: defineComponent({ render: () => null }),
}

function nodesOf(node: JSONContent, out = new Set<string>()): Set<string> {
  if (node.type) out.add(node.type)
  for (const c of node.content ?? []) nodesOf(c, out)
  return out
}

/** Types text as a user would, so that the input rules (Markdown shortcuts) run. */
function type(editor: Editor, text: string) {
  for (const ch of text) {
    const { from, to } = editor.state.selection
    const handled = editor.view.someProp('handleTextInput', (f) => f(editor.view, from, to, ch, () => editor.state.tr.insertText(ch, from, to)))
    if (!handled) editor.view.dispatch(editor.state.tr.insertText(ch, from, to))
  }
}

// The nodes the API admits in a page, read from its allowlist.
function apiPageNodes(): string[] {
  const src = readFileSync(resolve(__dirname, '../../../../backend/internal/richtext/richtext.go'), 'utf8')
  const block = (name: string) => {
    const start = src.indexOf(name)
    return src.slice(start, src.indexOf('\n}', start))
  }
  const keys = (code: string) => [...code.matchAll(/^\s*"(\w+)":\s*\{/gm)].map((m) => m[1])
  return [...keys(block('var nodeAttrs = map')), ...keys(block('var pageNodeAttrs = func'))]
}

let editor: Editor | null = null
afterEach(() => editor?.destroy())

describe('editor de páginas', () => {
  it('conoce los mismos nodos que la API (RF-1 a RF-6)', () => {
    expect([...apiPageNodes()].sort()).toEqual([...PAGE_NODES].sort())
  })

  it('solo produce nodos que la API admite, y nada de otras webs (RF-45)', () => {
    editor = new Editor({
      extensions: pageExtensions(ctx),
      content: `<h2>Título</h2>
        <ul data-type="taskList"><li data-type="taskItem" data-checked="true"><p>hecho</p></li></ul>
        <details><summary>Notas</summary><div data-type="detailsContent"><p>dentro</p></div></details>
        <div data-callout><p>Ojo</p></div>
        <img src="https://evil.example/x.png"><img data-file-id="11111111-1111-4111-8111-111111111111">
        <a data-file-attachment="22222222-2222-4222-8222-222222222222" data-name="a.pdf" data-size="10">a.pdf</a>
        <p><span data-type="mention" data-id="33333333-3333-4333-8333-333333333333" data-kind="user">@Ana</span></p>
        <iframe src="https://x"></iframe><script>alert(1)</script>`,
    })
    const json = editor.getJSON()
    const nodes = nodesOf(json)
    expect([...nodes].filter((n) => !PAGE_NODES.includes(n))).toEqual([])
    for (const n of ['taskList', 'details', 'callout', 'image', 'fileAttachment', 'mention']) expect(nodes).toContain(n)
    const images = JSON.stringify(json).match(/"type":"image"/g) ?? []
    expect(images).toHaveLength(1) // the outside image is dropped
    expect(JSON.stringify(json)).not.toMatch(/evil|script|iframe/)
  })

  it('nunca guarda el plegado de los bloques (RF-4)', () => {
    editor = new Editor({ extensions: pageExtensions(ctx), content: '<details open><summary>s</summary><div data-type="detailsContent"><p>x</p></div></details>' })
    expect(JSON.stringify(editor.getJSON())).not.toContain('"open"')
  })

  it('quita al guardar las subidas en curso', () => {
    const doc = {
      type: 'doc',
      content: [{ type: 'paragraph' }, { type: 'fileUpload', attrs: { uploadId: 'x' } }, { type: 'callout', content: [{ type: 'fileUpload' }, { type: 'paragraph' }] }],
    }
    expect(JSON.stringify(pageForSave(doc))).not.toMatch(new RegExp(CLIENT_ONLY_NODES.join('|')))
  })

  it.each([
    ['# ', 'heading'],
    ['## ', 'heading'],
    ['- ', 'bulletList'],
    ['1. ', 'orderedList'],
    ['[] ', 'taskList'],
    ['> ', 'blockquote'],
    ['``` ', 'codeBlock'],
    ['---', 'horizontalRule'],
  ])('convierte el atajo %s en %s (RF-56)', (shortcut, node) => {
    editor = new Editor({ extensions: pageExtensions(ctx), content: '<p></p>' })
    editor.commands.focus('start')
    type(editor, shortcut)
    expect(nodesOf(editor.getJSON())).toContain(node)
  })
})

describe('tamaño de los adjuntos', () => {
  it('se escribe en el idioma de la página', async () => {
    const { formatSize } = await import('./pageExtensions')
    expect(formatSize(1200000)).toMatch(/^1,1\s?MB$/)
    expect(formatSize(2048)).toMatch(/^2\s?kB$/i)
  })
})

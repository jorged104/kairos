import { Editor, type JSONContent } from '@tiptap/vue-3'
import { afterEach, describe, expect, it } from 'vitest'
import { ALLOWED_MARKS, ALLOWED_NODES, richTextExtensions } from './extensions'

function walk(node: JSONContent, nodes: Set<string>, marks: Set<string>, hrefs: string[]) {
  if (node.type) nodes.add(node.type)
  for (const m of node.marks ?? []) {
    marks.add(m.type)
    if (m.type === 'link') hrefs.push(String(m.attrs?.href))
  }
  for (const c of node.content ?? []) walk(c, nodes, marks, hrefs)
}

let editor: Editor | null = null
afterEach(() => editor?.destroy())

describe('editor de descripciones', () => {
  it('solo produce nodos y marcas que la API admite (RF-70, RF-71)', () => {
    editor = new Editor({
      extensions: richTextExtensions(),
      content: `<h2>Título</h2><h5>Grande</h5>
        <p><strong>b</strong> <em>i</em> <s>s</s> <u>u</u> <code>c</code>
        <a href="https://example.com">bien</a> <a href="javascript:alert(1)">mal</a>
        <span style="color:red" onclick="x()">estilo</span><img src="x" onerror="alert(1)"></p>
        <ul><li><p>a</p></li></ul><ol start="3"><li><p>b</p></li></ol>
        <pre><code class="language-go">x := 1</code></pre><blockquote><p>q</p></blockquote><hr>
        <table><tr><th>h</th></tr><tr><td>c</td></tr></table>
        <script>alert(1)</script><iframe src="https://x"></iframe>`,
    })
    const nodes = new Set<string>()
    const marks = new Set<string>()
    const hrefs: string[] = []
    walk(editor.getJSON(), nodes, marks, hrefs)

    expect([...nodes].filter((n) => !ALLOWED_NODES.includes(n))).toEqual([])
    expect([...marks].filter((m) => !ALLOWED_MARKS.includes(m))).toEqual([])
    expect(hrefs).toEqual(['https://example.com'])
    expect(nodes).toContain('table')
    expect(JSON.stringify(editor.getJSON())).not.toMatch(/script|iframe|onerror|onclick|style/)
  })

  it('limita los títulos a tres niveles', () => {
    editor = new Editor({ extensions: richTextExtensions(), content: '<h5>x</h5>' })
    const levels = (editor.getJSON().content ?? []).map((n) => n.attrs?.level).filter(Boolean)
    expect(levels.every((l) => l <= 3)).toBe(true)
  })
})

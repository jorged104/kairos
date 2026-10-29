import { computePosition, flip, offset, shift } from '@floating-ui/dom'
import type { PluginKey } from '@tiptap/pm/state'
import { exitSuggestion, type SuggestionOptions, type SuggestionProps } from '@tiptap/suggestion'
import { VueRenderer } from '@tiptap/vue-3'
import SuggestionList from './SuggestionList.vue'

/** One option of the / menu or of the @ and # searches. */
export interface SuggestionItem {
  id: string
  label: string
  hint?: string
  /** Heading of its group in the list (# search: work items, milestones, projects). */
  group?: string
  /** For mentions: user, work_item, milestone or project. */
  kind?: string
}

/**
 * Shows the options of a suggestion under the cursor. Arrows and Enter pick
 * one; Escape closes the list and keeps what was typed (RF-8, RF-9).
 */
export function suggestionRenderer(key: PluginKey, title: string, empty: () => string): SuggestionOptions<SuggestionItem>['render'] {
  return () => {
    let renderer: VueRenderer | null = null
    const place = (props: SuggestionProps<SuggestionItem>) => {
      const rect = props.clientRect?.()
      const el = renderer?.element as HTMLElement | undefined
      if (!rect || !el) return
      void computePosition({ getBoundingClientRect: () => rect }, el, {
        placement: 'bottom-start',
        middleware: [offset(6), flip(), shift({ padding: 8 })],
      }).then(({ x, y }) => Object.assign(el.style, { left: `${x}px`, top: `${y}px` }))
    }
    const close = () => {
      renderer?.element?.remove()
      renderer?.destroy()
      renderer = null
    }
    return {
      onStart(props) {
        renderer = new VueRenderer(SuggestionList, { props: { ...props, title, empty: empty() }, editor: props.editor })
        const el = renderer.element as HTMLElement
        Object.assign(el.style, { position: 'absolute', zIndex: '50', left: '0', top: '0' })
        document.body.appendChild(el)
        place(props)
      },
      onUpdate(props) {
        renderer?.updateProps({ ...props, title, empty: empty() })
        place(props)
      },
      onKeyDown({ event, view }) {
        if (event.key === 'Escape') {
          exitSuggestion(view, key)
          close()
          return true
        }
        return (renderer?.ref as { onKeyDown?: (e: KeyboardEvent) => boolean } | null)?.onKeyDown?.(event) ?? false
      },
      onExit: close,
    }
  }
}

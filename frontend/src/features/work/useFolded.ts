import { effectScope, ref, watch, type Ref } from 'vue'

const folds = new Map<string, Ref<boolean>>()

/**
 * Whether a section of the work item sheet is open, the same for every sheet
 * and remembered in this browser (spec 007, RF-7). Closed by default.
 */
export function useFolded(name: string): Ref<boolean> {
  const key = `kairos.sheet.${name}`
  const known = folds.get(key)
  if (known) return known
  let initial = false
  try {
    initial = localStorage.getItem(key) === '1'
  } catch {
    // Private window or blocked storage: closed.
  }
  const open = ref(initial)
  // Shared by every sheet, so it outlives the component that made it.
  effectScope(true).run(() =>
    watch(open, (v) => {
      try {
        localStorage.setItem(key, v ? '1' : '0')
      } catch {
        // Not remembered this time; the sheet still works.
      }
    }),
  )
  folds.set(key, open)
  return open
}

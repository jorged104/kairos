import { ref, watch } from 'vue'

const KEY = 'kairos.page.fullWidth'

function read(): boolean {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false // private window or blocked storage: the normal view
  }
}

// One choice for the whole app, remembered in this browser (RF-40, RF-41).
const on = ref(read())
watch(on, (v) => {
  try {
    localStorage.setItem(KEY, v ? '1' : '0')
  } catch {
    // Not remembered this time; the page still works.
  }
})

/** Whether pages use the whole window, without the app's navigation. */
export function useFullWidth() {
  return { on, toggle: () => (on.value = !on.value) }
}

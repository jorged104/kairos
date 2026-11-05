import { onUnmounted, ref, watch, type Ref } from 'vue'

// Columns that do not fit: the mouse wheel moves them sideways until an end,
// where the page scrolls again, and two buttons move them a column at a time
// (spec 013, RF-14b, RF-14c). The element may come and go (an empty board).
export function useHorizontalScroll(el: Ref<HTMLElement | null>, step: () => number) {
  const canLeft = ref(false)
  const canRight = ref(false)

  function update() {
    const e = el.value
    if (!e) {
      canLeft.value = canRight.value = false
      return
    }
    canLeft.value = e.scrollLeft > 1
    canRight.value = e.scrollLeft + e.clientWidth < e.scrollWidth - 1
  }

  function onWheel(ev: WheelEvent) {
    const e = el.value
    if (!e || ev.ctrlKey || Math.abs(ev.deltaX) > Math.abs(ev.deltaY)) return
    if ((ev.deltaY < 0 && !canLeft.value) || (ev.deltaY > 0 && !canRight.value)) return
    ev.preventDefault()
    e.scrollLeft += ev.deltaY
    update()
  }

  function by(direction: 1 | -1) {
    el.value?.scrollBy({ left: direction * step(), behavior: 'smooth' })
  }

  const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(update) : null
  function detach(e: HTMLElement) {
    e.removeEventListener('wheel', onWheel)
    e.removeEventListener('scroll', update)
    observer?.unobserve(e)
  }
  watch(
    el,
    (e, old) => {
      if (old) detach(old)
      if (e) {
        e.addEventListener('wheel', onWheel, { passive: false })
        e.addEventListener('scroll', update, { passive: true })
        observer?.observe(e)
      }
      update()
    },
    { immediate: true, flush: 'post' },
  )
  onUnmounted(() => {
    if (el.value) detach(el.value)
    observer?.disconnect()
  })

  return { canLeft, canRight, by, update }
}

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { defineComponent, h, nextTick, ref } from 'vue'
import { useHorizontalScroll } from './useHorizontalScroll'

// jsdom has no layout: the sizes of the board are set by hand.
async function board(scrollWidth: number, clientWidth: number) {
  let api!: ReturnType<typeof useHorizontalScroll>
  const el = ref<HTMLElement | null>(null)
  const w = mount(defineComponent({
    setup() {
      api = useHorizontalScroll(el, () => 268)
      return () => h('div', { ref: el })
    },
  }))
  await nextTick() // the listeners go on after the render
  Object.defineProperty(el.value!, 'scrollWidth', { value: scrollWidth })
  Object.defineProperty(el.value!, 'clientWidth', { value: clientWidth })
  api.update()
  return { w, el: el.value!, api }
}
const wheel = (el: HTMLElement, deltaY: number) => {
  const ev = new WheelEvent('wheel', { deltaY, cancelable: true })
  el.dispatchEvent(ev)
  return ev
}

describe('columnas que no caben (spec 013, RF-14b, RF-14c)', () => {
  it('la rueda las desplaza en horizontal hasta un extremo, y allí deja desplazar la página', async () => {
    const { el, api } = await board(1000, 400)
    expect([api.canLeft.value, api.canRight.value]).toEqual([false, true])
    expect(wheel(el, -100).defaultPrevented).toBe(false)
    expect(wheel(el, 120).defaultPrevented).toBe(true)
    expect(el.scrollLeft).toBe(120)
    await nextTick()
    expect(api.canLeft.value).toBe(true)
    el.scrollLeft = 600
    api.update()
    expect(api.canRight.value).toBe(false)
    expect(wheel(el, 120).defaultPrevented).toBe(false)
  })

  it('si caben, ni botones ni rueda', async () => {
    const { el, api } = await board(400, 400)
    expect([api.canLeft.value, api.canRight.value]).toEqual([false, false])
    expect(wheel(el, 120).defaultPrevented).toBe(false)
  })
})

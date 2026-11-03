import { QueryClient, VueQueryPlugin } from '@tanstack/vue-query'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ScheduleSettings } from '@/api/schedule'
import { createAppI18n } from '@/lib/i18n'
import ScheduleSettingsAdmin from './ScheduleSettingsAdmin.vue'

const json = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
const settings: ScheduleSettings = {
  upper: 5, risk: -5, late: -15, version: 3,
  levels: { ahead: { name: null, color: 'blue' }, on_track: { name: null, color: 'green' }, at_risk: { name: null, color: 'amber' }, late: { name: null, color: 'red' } },
}

describe('semáforo (spec 009)', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('cambia umbrales, nombres y colores y no deja guardar umbrales desordenados (RF-36 a RF-40)', async () => {
    const fetch = vi.fn((_url: string, init?: RequestInit) => json(init?.method === 'PUT' ? { ...JSON.parse(String(init.body)), version: 4 } : settings))
    vi.stubGlobal('fetch', fetch)
    const wrapper = mount(ScheduleSettingsAdmin, { global: { plugins: [createAppI18n(), [VueQueryPlugin, { queryClient: new QueryClient() }]] } })
    await flushPromises()
    expect((wrapper.find('[data-testid=threshold-upper]').element as HTMLInputElement).value).toBe('5')

    await wrapper.find('[data-testid=threshold-risk]').setValue('6')
    expect(wrapper.find('[data-testid=threshold-order]').exists()).toBe(true)
    expect(wrapper.find('[data-testid=schedule-save]').attributes('disabled')).toBeDefined()

    await wrapper.find('[data-testid=threshold-risk]').setValue('-3')
    await wrapper.find('[data-testid=level-name-ahead]').setValue('Por delante')
    await wrapper.find('[data-testid=level-color-late]').setValue('orange')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const put = fetch.mock.calls.find(([, i]) => i?.method === 'PUT')!
    expect(JSON.parse(String(put[1]!.body))).toMatchObject({
      upper: 5, risk: -3, late: -15, version: 3,
      levels: { ahead: { name: 'Por delante', color: 'blue' }, late: { name: null, color: 'orange' } },
    })
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
})

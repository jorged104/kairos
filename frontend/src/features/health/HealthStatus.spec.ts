import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { ReadinessState } from '@/api/health'
import { createAppI18n } from '@/lib/i18n'
import HealthStatus from './HealthStatus.vue'

function render(state?: ReadinessState) {
  return mount(HealthStatus, { props: { state }, global: { plugins: [createAppI18n()] } })
}

describe('HealthStatus', () => {
  it('muestra API y BD disponibles sin alerta', () => {
    const wrapper = render({ api: 'available', database: 'available' })

    expect(wrapper.get('[data-testid=status-api]').text()).toContain('Disponible')
    expect(wrapper.get('[data-testid=status-database]').text()).toContain('Disponible')
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
  })

  it('muestra la BD no disponible con alerta', () => {
    const wrapper = render({ api: 'available', database: 'unavailable' })

    expect(wrapper.get('[data-testid=status-database]').attributes('data-status')).toBe('unavailable')
    expect(wrapper.get('[role=alert]').text()).toContain('la base de datos no responde')
  })

  it('muestra la API no disponible y la BD desconocida con alerta', () => {
    const wrapper = render({ api: 'unavailable', database: 'unknown' })

    expect(wrapper.get('[data-testid=status-api]').text()).toContain('No disponible')
    expect(wrapper.get('[data-testid=status-database]').text()).toContain('Desconocido')
    expect(wrapper.get('[role=alert]').text()).toContain('No se puede contactar con la API')
  })

  it('muestra "Comprobando" antes de la primera respuesta', () => {
    const wrapper = render()

    expect(wrapper.get('[data-testid=status-api]').text()).toContain('Comprobando')
    expect(wrapper.find('[role=alert]').exists()).toBe(false)
  })
})

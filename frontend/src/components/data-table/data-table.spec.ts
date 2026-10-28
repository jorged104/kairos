import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { h } from 'vue'
import { ApiError } from '@/api/client'
import { createAppI18n } from '@/lib/i18n'
import EditableCell from './EditableCell.vue'
import { neighbor } from './gridNav'
import QuickAddRow from './QuickAddRow.vue'

const global = { plugins: [createAppI18n()] }

function cell(props: Record<string, unknown>) {
  return mount(
    { render: () => h('table', h('tbody', h('tr', h(EditableCell, { label: 'Título', ...props })))) },
    { global, attachTo: document.body },
  )
}

describe('celda editable (RF-139)', () => {
  it('edita con clic, guarda con Enter y avisa del guardado', async () => {
    const save = vi.fn(() => Promise.resolve())
    const w = cell({ value: 'Login', save })
    await w.find('td').trigger('click')
    await flushPromises()
    const input = w.find('input')
    await input.setValue('Login v2')
    await input.trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(save).toHaveBeenCalledWith('Login v2')
    expect(w.find('input').exists()).toBe(false)
    w.unmount()
  })

  it('Esc descarta el cambio sin guardar', async () => {
    const save = vi.fn(() => Promise.resolve())
    const w = cell({ value: 'Login', save })
    await w.find('td').trigger('click')
    await flushPromises()
    await w.find('input').setValue('Otro')
    await w.find('input').trigger('keydown', { key: 'Escape' })
    expect(save).not.toHaveBeenCalled()
    expect(w.text()).toContain('Login')
    w.unmount()
  })

  it('no guarda un obligatorio vacío y muestra el error de la API por código', async () => {
    const save = vi.fn(() => Promise.reject(new ApiError('version_conflict', 409)))
    const w = cell({ value: 'Login', save, required: true })
    await w.find('td').trigger('click')
    await flushPromises()
    await w.find('input').setValue('  ')
    await w.find('input').trigger('keydown', { key: 'Enter' })
    expect(save).not.toHaveBeenCalled()
    expect(w.find('[role=alert]').text()).toContain('obligatorio')

    await w.find('input').setValue('Nuevo')
    await w.find('input').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(w.find('[role=alert]').text()).toContain('Otra persona cambió')
    expect(w.find('[role=alert] button').text()).toBe('Recargar')
    w.unmount()
  })

  it('carga las opciones de un selector al empezar a editar', async () => {
    const options = vi.fn(() => Promise.resolve([{ value: 'a', label: 'Abierto' }, { value: 'b', label: 'Cerrado' }]))
    const save = vi.fn(() => Promise.resolve())
    const w = cell({ value: 'a', kind: 'select', options, save, required: true })
    await w.find('td').trigger('click')
    await flushPromises()
    expect(options).toHaveBeenCalledOnce()
    await w.find('select').setValue('b')
    await flushPromises()
    expect(save).toHaveBeenCalledWith('b')
    w.unmount()
  })

  it('sin permiso no se puede editar ni enfocar', async () => {
    const w = cell({ value: 'Login', save: vi.fn(), editable: false })
    await w.find('td').trigger('click')
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('td').attributes('tabindex')).toBeUndefined()
    w.unmount()
  })
})

describe('fila rápida (RF-140)', () => {
  const row = (submit: (title: string) => Promise<unknown>) =>
    mount(
      { render: () => h('table', h('tbody', h(QuickAddRow, { colspan: 3, label: 'Nuevo hito', placeholder: 'Nombre', submit }))) },
      { global, attachTo: document.body },
    )

  it('crea con Enter y queda abierta y vacía para el siguiente', async () => {
    const submit = vi.fn(() => Promise.resolve())
    const w = row(submit)
    await w.find('button').trigger('click')
    await w.find('input').setValue('Beta')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(submit).toHaveBeenCalledWith('Beta')
    expect((w.find('input').element as HTMLInputElement).value).toBe('')
    w.unmount()
  })

  it('muestra el error y conserva el texto', async () => {
    const w = row(() => Promise.reject(new ApiError('forbidden', 403)))
    await w.find('button').trigger('click')
    await w.find('input').setValue('Beta')
    await w.find('form').trigger('submit')
    await flushPromises()
    expect(w.find('[role=alert]').exists()).toBe(true)
    expect((w.find('input').element as HTMLInputElement).value).toBe('Beta')
    w.unmount()
  })
})

describe('navegación con flechas', () => {
  it('se mueve entre celdas editables y salta las que no lo son', () => {
    document.body.innerHTML = `<table><tbody>
      <tr><td data-grid-cell id="a1"></td><td data-grid-cell id="b1"></td></tr>
      <tr><td data-grid-cell id="a2"></td><td id="b2"></td></tr>
      <tr><td data-grid-cell id="a3"></td><td data-grid-cell id="b3"></td></tr>
    </tbody></table>`
    const el = (id: string) => document.getElementById(id) as HTMLElement
    expect(neighbor(el('a1'), 'ArrowRight')?.id).toBe('b1')
    expect(neighbor(el('b1'), 'ArrowRight')).toBeNull()
    expect(neighbor(el('b1'), 'ArrowDown')?.id).toBe('b3')
    expect(neighbor(el('a3'), 'ArrowUp')?.id).toBe('a2')
    expect(neighbor(el('a1'), 'ArrowUp')).toBeNull()
  })
})

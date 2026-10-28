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

describe('fila rápida (RF-145 a RF-147)', () => {
  // The slot holds a required phase select, like the extra fields of the screens.
  const row = (submit: (title: string) => Promise<unknown>) =>
    mount(
      {
        render: () =>
          h(
            'table',
            h(
              'tbody',
              h(QuickAddRow, { colspan: 3, label: 'Nuevo hito', placeholder: 'Nombre', submit }, () =>
                h('select', { 'aria-label': 'Fase', required: true }, [h('option', { value: '' }, '—'), h('option', { value: 'f1', selected: true }, 'Inicio')]),
              ),
            ),
          ),
      },
      { global, attachTo: document.body },
    )
  const open = async (w: ReturnType<typeof row>) => {
    await w.find('button').trigger('click')
    await flushPromises()
  }
  const submitButton = (w: ReturnType<typeof row>) => w.find('button[type=submit]')

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

  it('crea con Enter desde un selector de la fila (RF-146)', async () => {
    const submit = vi.fn(() => Promise.resolve())
    const w = row(submit)
    await open(w)
    await w.find('input').setValue('Beta')
    await w.find('select').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(submit).toHaveBeenCalledWith('Beta')
    w.unmount()
  })

  it('crea con el botón «Crear» visible y no crea dos veces mientras guarda (RF-146)', async () => {
    let done = () => {}
    const submit = vi.fn(() => new Promise<void>((resolve) => (done = resolve)))
    const w = row(submit)
    await open(w)
    expect(submitButton(w).text()).toBe('Crear')
    expect(submitButton(w).classes()).not.toContain('sr-only')
    await w.find('input').setValue('Beta')
    await submitButton(w).trigger('click')
    await w.find('select').trigger('keydown', { key: 'Enter' })
    await w.find('input').trigger('keydown', { key: 'Enter' })
    expect(submit).toHaveBeenCalledOnce()
    expect(submitButton(w).attributes('disabled')).toBeDefined()
    done()
    await flushPromises()
    expect(submitButton(w).attributes('disabled')).toBeUndefined()
    w.unmount()
  })

  it('sin nombre u otro obligatorio no crea y enfoca el campo que falta (RF-147)', async () => {
    const submit = vi.fn(() => Promise.resolve())
    const w = row(submit)
    await open(w)
    await w.find('input').setValue('   ')
    await submitButton(w).trigger('click')
    expect(submit).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(w.find('input').element)
    expect(w.find('[role=alert]').text()).toBe('Este campo es obligatorio.')

    await w.find('input').setValue('Beta')
    expect(w.find('[role=alert]').exists()).toBe(false)
    await w.find('select').setValue('')
    await w.find('input').trigger('keydown', { key: 'Enter' })
    expect(submit).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(w.find('select').element)
    expect(w.find('[role=alert]').exists()).toBe(true)
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

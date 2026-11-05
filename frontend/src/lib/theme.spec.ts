import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { contrast, DEFAULT_THEME, isDark, PALETTES, PRIMARIES, themeCss, themeVariables } from '@/lib/theme'

const css = readFileSync('src/style.css', 'utf8')

// The variables of a block of style.css, as "name: value".
function block(selector: string): Record<string, string> {
  const start = css.indexOf(`${selector} {`)
  const body = css.slice(start, css.indexOf('}', start))
  const out: Record<string, string> = {}
  for (const m of body.matchAll(/--([\w-]+):\s*([^;]+);/g)) out[m[1]!] = m[2]!.trim()
  return out
}
const lch = (v: string) => v.replace(/^oklch\(|\)$/g, '')

describe('apariencia (spec 015)', () => {
  it('la apariencia inicial reproduce la de siempre (RF-46)', () => {
    const { light, dark } = themeVariables(DEFAULT_THEME)
    const root = block(':root')
    const darkCss = block('.dark')
    for (const [k, v] of Object.entries(light)) expect(`${k}: ${v}`).toBe(`${k}: ${root[k]}`)
    for (const [k, v] of Object.entries(dark)) expect(`${k}: ${v}`).toBe(`${k}: ${darkCss[k]}`)
    expect(themeCss(DEFAULT_THEME)).toContain('--radius: 0.625rem;')
  })

  it('cada combinación tiene un contraste de 4,5:1 entre el primario y su texto, en claro y en oscuro (RF-41, RF-42)', () => {
    for (const palette of PALETTES)
      for (const primary of PRIMARIES) {
        const vars = themeVariables({ palette, primary, radius: 'soft' })
        for (const [variant, v] of Object.entries(vars)) {
          const ratio = contrast(lch(v.primary!), lch(v['primary-foreground']!))
          expect(ratio, `${palette}/${primary}/${variant}: ${ratio.toFixed(2)}`).toBeGreaterThanOrEqual(4.5)
        }
      }
  })

  it('el radio y el color primario cambian las variables (RF-40, RF-43)', () => {
    const v = themeVariables({ palette: 'zinc', primary: 'violet', radius: 'none' })
    expect(v.light.radius).toBe('0rem')
    expect(v.light.primary).not.toBe(themeVariables({ palette: 'zinc', primary: 'default', radius: 'none' }).light.primary)
    expect(v.light.ring).toBe(v.light.primary)
  })

  it('el modo oscuro sigue la elección o el sistema (RF-48, RF-51)', () => {
    expect(isDark('dark', false)).toBe(true)
    expect(isDark('light', true)).toBe(false)
    expect(isDark('system', true)).toBe(true)
    expect(isDark('system', false)).toBe(false)
  })
})

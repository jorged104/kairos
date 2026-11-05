// Appearance of the system (spec 015, RF-39 to RF-51): a base palette of
// greys, an accent colour and a corner radius, each with a light and a dark
// variant. The values are shadcn's, in oklch ("L C H").

export type Palette = 'neutral' | 'zinc' | 'slate' | 'stone' | 'gray'
export type Primary = 'default' | 'blue' | 'green' | 'orange' | 'red' | 'rose' | 'violet' | 'yellow'
export type Radius = 'none' | 'soft' | 'round'
export type ThemeMode = 'light' | 'dark' | 'system'

export interface Theme {
  palette: Palette
  primary: Primary
  radius: Radius
}

export const DEFAULT_THEME: Theme = { palette: 'neutral', primary: 'default', radius: 'soft' } // RF-46
export const PALETTES: Palette[] = ['neutral', 'zinc', 'slate', 'stone', 'gray']
export const PRIMARIES: Primary[] = ['default', 'blue', 'green', 'orange', 'red', 'rose', 'violet', 'yellow']
export const RADII: Radius[] = ['none', 'soft', 'round']

const RADIUS: Record<Radius, string> = { none: '0rem', soft: '0.625rem', round: '1rem' }

interface LightTones {
  fg: string
  primary: string
  primaryFg: string
  secondary: string
  mutedFg: string
  border: string
  ring: string
  sidebar: string
}
interface DarkTones {
  bg: string
  fg: string
  card: string
  primary: string
  primaryFg: string
  secondary: string
  mutedFg: string
  ring: string
}

const PALETTE_TONES: Record<Palette, { light: LightTones; dark: DarkTones }> = {
  neutral: {
    light: { fg: '0.145 0 0', primary: '0.205 0 0', primaryFg: '0.985 0 0', secondary: '0.97 0 0', mutedFg: '0.556 0 0', border: '0.922 0 0', ring: '0.708 0 0', sidebar: '0.985 0 0' },
    dark: { bg: '0.145 0 0', fg: '0.985 0 0', card: '0.205 0 0', primary: '0.922 0 0', primaryFg: '0.205 0 0', secondary: '0.269 0 0', mutedFg: '0.708 0 0', ring: '0.556 0 0' },
  },
  zinc: {
    light: { fg: '0.141 0.005 285.823', primary: '0.21 0.006 285.885', primaryFg: '0.985 0 0', secondary: '0.967 0.001 286.375', mutedFg: '0.552 0.016 285.938', border: '0.92 0.004 286.32', ring: '0.705 0.015 286.067', sidebar: '0.985 0 0' },
    dark: { bg: '0.141 0.005 285.823', fg: '0.985 0 0', card: '0.21 0.006 285.885', primary: '0.92 0.004 286.32', primaryFg: '0.21 0.006 285.885', secondary: '0.274 0.006 286.033', mutedFg: '0.705 0.015 286.067', ring: '0.552 0.016 285.938' },
  },
  slate: {
    light: { fg: '0.129 0.042 264.695', primary: '0.208 0.042 265.755', primaryFg: '0.984 0.003 247.858', secondary: '0.968 0.007 247.896', mutedFg: '0.554 0.046 257.417', border: '0.929 0.013 255.508', ring: '0.704 0.04 256.788', sidebar: '0.984 0.003 247.858' },
    dark: { bg: '0.129 0.042 264.695', fg: '0.984 0.003 247.858', card: '0.208 0.042 265.755', primary: '0.929 0.013 255.508', primaryFg: '0.208 0.042 265.755', secondary: '0.279 0.041 260.031', mutedFg: '0.704 0.04 256.788', ring: '0.551 0.027 264.364' },
  },
  stone: {
    light: { fg: '0.147 0.004 49.25', primary: '0.216 0.006 56.043', primaryFg: '0.985 0.001 106.423', secondary: '0.97 0.001 106.424', mutedFg: '0.553 0.013 58.071', border: '0.923 0.003 48.717', ring: '0.709 0.01 56.259', sidebar: '0.985 0.001 106.423' },
    dark: { bg: '0.147 0.004 49.25', fg: '0.985 0.001 106.423', card: '0.216 0.006 56.043', primary: '0.923 0.003 48.717', primaryFg: '0.216 0.006 56.043', secondary: '0.268 0.007 34.298', mutedFg: '0.709 0.01 56.259', ring: '0.553 0.013 58.071' },
  },
  gray: {
    light: { fg: '0.13 0.028 261.692', primary: '0.21 0.034 264.665', primaryFg: '0.985 0.002 247.839', secondary: '0.967 0.003 264.542', mutedFg: '0.551 0.027 264.364', border: '0.928 0.006 264.531', ring: '0.707 0.022 261.325', sidebar: '0.985 0.002 247.839' },
    dark: { bg: '0.13 0.028 261.692', fg: '0.985 0.002 247.839', card: '0.21 0.034 264.665', primary: '0.928 0.006 264.531', primaryFg: '0.21 0.034 264.665', secondary: '0.278 0.033 256.848', mutedFg: '0.707 0.022 261.325', ring: '0.551 0.027 264.364' },
  },
}

// The accent of each colour and the text on it, chosen so that pair keeps a
// contrast of at least 4.5:1 in both variants (RF-42).
const WHITE = '0.985 0 0'
const INK = '0.205 0 0'
export const PRIMARY_TONES: Record<Exclude<Primary, 'default'>, { light: [string, string]; dark: [string, string] }> = {
  blue: { light: ['0.488 0.217 264.4', WHITE], dark: ['0.707 0.165 254.6', INK] },
  green: { light: ['0.52 0.154 150.1', WHITE], dark: ['0.792 0.209 151.7', INK] },
  orange: { light: ['0.553 0.195 38.4', WHITE], dark: ['0.75 0.183 55.9', INK] },
  red: { light: ['0.505 0.213 27.5', WHITE], dark: ['0.704 0.191 22.2', INK] },
  rose: { light: ['0.514 0.222 16.9', WHITE], dark: ['0.712 0.194 13.4', INK] },
  violet: { light: ['0.491 0.27 292.6', WHITE], dark: ['0.702 0.183 293.5', INK] },
  yellow: { light: ['0.795 0.184 86', INK], dark: ['0.852 0.199 91.9', INK] },
}

/** The CSS variables of a theme, for the light (:root) and dark (.dark) variants. */
export function themeVariables(theme: Theme): { light: Record<string, string>; dark: Record<string, string> } {
  const { light: l, dark: d } = PALETTE_TONES[theme.palette] ?? PALETTE_TONES.neutral
  const ok = (v: string) => `oklch(${v})`
  const light: Record<string, string> = {
    radius: RADIUS[theme.radius] ?? RADIUS.soft,
    background: ok('1 0 0'), foreground: ok(l.fg), card: ok('1 0 0'), 'card-foreground': ok(l.fg),
    popover: ok('1 0 0'), 'popover-foreground': ok(l.fg), primary: ok(l.primary), 'primary-foreground': ok(l.primaryFg),
    secondary: ok(l.secondary), 'secondary-foreground': ok(l.primary), muted: ok(l.secondary), 'muted-foreground': ok(l.mutedFg),
    accent: ok(l.secondary), 'accent-foreground': ok(l.primary), border: ok(l.border), input: ok(l.border), ring: ok(l.ring),
    sidebar: ok(l.sidebar), 'sidebar-foreground': ok(l.fg), 'sidebar-primary': ok(l.primary), 'sidebar-primary-foreground': ok(l.primaryFg),
    'sidebar-accent': ok(l.secondary), 'sidebar-accent-foreground': ok(l.primary), 'sidebar-border': ok(l.border), 'sidebar-ring': ok(l.ring),
  }
  const dark: Record<string, string> = {
    background: ok(d.bg), foreground: ok(d.fg), card: ok(d.card), 'card-foreground': ok(d.fg),
    popover: ok(d.card), 'popover-foreground': ok(d.fg), primary: ok(d.primary), 'primary-foreground': ok(d.primaryFg),
    secondary: ok(d.secondary), 'secondary-foreground': ok(d.fg), muted: ok(d.secondary), 'muted-foreground': ok(d.mutedFg),
    accent: ok(d.secondary), 'accent-foreground': ok(d.fg), border: ok('1 0 0 / 10%'), input: ok('1 0 0 / 15%'), ring: ok(d.ring),
    sidebar: ok(d.card), 'sidebar-foreground': ok(d.fg), 'sidebar-primary': ok('0.488 0.243 264.376'), 'sidebar-primary-foreground': ok(d.fg),
    'sidebar-accent': ok(d.secondary), 'sidebar-accent-foreground': ok(d.fg), 'sidebar-border': ok('1 0 0 / 10%'), 'sidebar-ring': ok(d.ring),
  }
  if (theme.primary !== 'default' && PRIMARY_TONES[theme.primary]) {
    const p = PRIMARY_TONES[theme.primary]
    for (const [vars, [bg, fg]] of [[light, p.light], [dark, p.dark]] as const) {
      vars.primary = vars['sidebar-primary'] = vars.ring = vars['sidebar-ring'] = ok(bg)
      vars['primary-foreground'] = vars['sidebar-primary-foreground'] = ok(fg)
    }
  }
  return { light, dark }
}

export function themeCss(theme: Theme): string {
  const { light, dark } = themeVariables(theme)
  const block = (vars: Record<string, string>) =>
    Object.entries(vars)
      .map(([k, v]) => `  --${k}: ${v};`)
      .join('\n')
  return `:root {\n${block(light)}\n}\n.dark {\n${block(dark)}\n}\n`
}

const STYLE_ID = 'kairos-theme'

/** Writes the theme's variables after the base stylesheet. */
export function applyTheme(theme: Theme, doc: Document = document) {
  let el = doc.getElementById(STYLE_ID) as HTMLStyleElement | null
  if (!el) {
    el = doc.createElement('style')
    el.id = STYLE_ID
    doc.head.appendChild(el)
  }
  el.textContent = themeCss(theme)
}

/** Whether the dark variant applies for a mode (RF-48, RF-51). */
export function isDark(mode: ThemeMode, prefersDark: boolean): boolean {
  return mode === 'dark' || (mode === 'system' && prefersDark)
}

// Contrast (WCAG) between two oklch colours "L C H", through linear sRGB.
function luminance(v: string): number {
  const [L, C, H] = v.split(/\s+/).map(Number) as [number, number, number]
  const a = C * Math.cos((H * Math.PI) / 180)
  const b = C * Math.sin((H * Math.PI) / 180)
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3
  const clamp = (x: number) => Math.min(1, Math.max(0, x))
  const r = clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s)
  const g = clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s)
  const bl = clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s)
  return 0.2126 * r + 0.7152 * g + 0.0722 * bl
}

export function contrast(a: string, b: string): number {
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p) as [number, number]
  return (x + 0.05) / (y + 0.05)
}

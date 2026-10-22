import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'
import es from './es.json'

const srcDir = join(import.meta.dirname, '..')

// Matches t('key'), $t('key') and t("key") with a literal key.
const keyPattern = /\$?\bt\(\s*['"]([\w.-]+)['"]/g

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) return name === 'ui' ? [] : sourceFiles(path)
    return /\.(vue|ts)$/.test(name) && !name.endsWith('.spec.ts') ? [path] : []
  })
}

function usedKeys(files: string[]): Map<string, string> {
  const keys = new Map<string, string>()
  for (const file of files) {
    for (const match of readFileSync(file, 'utf8').matchAll(keyPattern)) {
      keys.set(match[1], relative(srcDir, file))
    }
  }
  return keys
}

function hasKey(catalog: unknown, key: string): boolean {
  let node = catalog
  for (const part of key.split('.')) {
    if (typeof node !== 'object' || node === null || !(part in node)) return false
    node = (node as Record<string, unknown>)[part]
  }
  return typeof node === 'string'
}

describe('catálogo es.json', () => {
  it('contiene todas las claves usadas en la interfaz', () => {
    const missing = [...usedKeys(sourceFiles(srcDir))]
      .filter(([key]) => !hasKey(es, key))
      .map(([key, file]) => `${key} (${file})`)

    expect(missing, `Claves ausentes en es.json:\n${missing.join('\n')}`).toEqual([])
  })

  it('detecta una clave inexistente', () => {
    expect(hasKey(es, 'health.title')).toBe(true)
    expect(hasKey(es, 'health.no.existe')).toBe(false)
    expect(hasKey(es, 'health')).toBe(false)
  })
})

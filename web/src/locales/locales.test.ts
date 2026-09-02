import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative } from 'node:path'
import { cwd } from 'node:process'
import test from 'node:test'
import enUS from './en-US.js'
import zhCN from './zh-CN.js'

const collectLeafPaths = (value: unknown, prefix = ''): string[] => {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return prefix ? [prefix] : []
  }

  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    return collectLeafPaths(child, path)
  })
}

const staticLocalePaths = (locale: unknown): string[] => {
  return collectLeafPaths(locale)
    .filter((path) => !path.startsWith('nodeName.'))
    .sort()
}

const hasMessage = (locale: unknown, path: string): boolean => {
  let current = locale
  for (const segment of path.split('.')) {
    if (typeof current !== 'object' || current === null || !(segment in current)) {
      return false
    }
    current = (current as Record<string, unknown>)[segment]
  }
  return typeof current === 'string' && current.trim() !== ''
}

const sourceFiles = (directory: string): string[] => {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) {
      return sourceFiles(path)
    }
    return ['.ts', '.vue'].includes(extname(entry.name)) && !entry.name.endsWith('.test.ts')
      ? [path]
      : []
  })
}

test('Chinese and English locales expose the same static message keys', () => {
  assert.deepEqual(staticLocalePaths(enUS), staticLocalePaths(zhCN))
})

test('locale leaf values are non-empty strings', () => {
  for (const [name, locale] of [
    ['en-US', enUS],
    ['zh-CN', zhCN]
  ] as const) {
    const visit = (value: unknown, prefix = ''): void => {
      if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
        for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
          visit(child, prefix ? `${prefix}.${key}` : key)
        }
        return
      }

      assert.equal(typeof value, 'string', `${name}:${prefix} must be a string`)
      assert.notEqual((value as string).trim(), '', `${name}:${prefix} must not be empty`)
    }

    visit(locale)
  }
})

test('static translation calls resolve in every locale', () => {
  const sourceRoot = join(cwd(), 'src')
  const missing: string[] = []
  for (const filename of sourceFiles(sourceRoot)) {
    const source = readFileSync(filename, 'utf8')
    const translationCall = /(?:\bt|\$t)\(\s*['"]([^'"]+)['"]/g
    for (const match of source.matchAll(translationCall)) {
      const key = match[1]
      if (!key || (hasMessage(enUS, key) && hasMessage(zhCN, key))) {
        continue
      }
      missing.push(`${relative(sourceRoot, filename)}: ${key}`)
    }
  }

  assert.deepEqual(missing, [])
})

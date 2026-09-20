import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative } from 'node:path'
import { cwd } from 'node:process'
import test from 'node:test'
import { createI18n } from 'vue-i18n'
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

function flattenMessages(messages: object, prefix = ''): Map<string, string> {
  const result = new Map<string, string>()
  for (const [key, value] of Object.entries(messages)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') result.set(path, value)
    else for (const [nestedKey, message] of flattenMessages(value, path)) result.set(nestedKey, message)
  }
  return result
}

const parameters = (message: string): string[] =>
  [...new Set([...message.matchAll(/\{\s*([\w]+)\s*\}/g)].map((match) => match[1]!))].sort()

test('interface translations use matching interpolation parameters', () => {
  // Node names fall back to their configured text; only translated names need entries.
  const interfaceMessages = (messages: object) =>
    new Map([...flattenMessages(messages)].filter(([key]) => !key.startsWith('nodeName.')))
  const chinese = interfaceMessages(zhCN)
  const english = interfaceMessages(enUS)
  for (const [key, message] of chinese) {
    assert.deepEqual(parameters(english.get(key)!), parameters(message), `parameters differ: ${key}`)
  }
})

test('all catalog messages compile and interpolate through vue-i18n', () => {
  for (const [locale, messages] of Object.entries({ 'zh-CN': zhCN, 'en-US': enUS })) {
    const i18n = createI18n({ legacy: false, locale, fallbackLocale: false, messages: { [locale]: messages } })
    for (const [key, message] of flattenMessages(messages)) {
      const values = Object.fromEntries(parameters(message).map((parameter) => [parameter, `test_${parameter}`]))
      const rendered = i18n.global.t(key, values)
      assert.notEqual(rendered, key, `message failed to render: ${locale}.${key}`)
      for (const value of Object.values(values)) {
        assert.ok(rendered.includes(value), `interpolation missing: ${locale}.${key}: ${value}`)
      }
    }
    i18n.dispose()
  }
})

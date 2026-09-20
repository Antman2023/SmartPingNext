import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vueRouter from 'vue-router'

const source = readFileSync(new URL('../../src/router/index.ts', import.meta.url), 'utf8')

test('router navigation and fallback retain the configured deployment prefix', async () => {
  for (const base of ['/', '/smartping/', '/nested/app/']) {
    const compiled = ts.transpileModule(source.replace('import.meta.env.BASE_URL', JSON.stringify(base)), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
    }).outputText
    const exports = {} as { default: vueRouter.Router }
    const document = { title: '' }
    runInNewContext(compiled, {
      exports, document,
      require: (name: string) => {
        // Run the actual route matcher and navigation guards without a browser DOM.
        if (name === 'vue-router') return { ...vueRouter, createWebHistory: vueRouter.createMemoryHistory }
        if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
        if (name.startsWith('@/views/') && name.endsWith('.vue')) return { default: {} }
        throw new Error(`Unexpected dependency: ${name}`)
      }
    })
    const router = exports.default
    for (const path of ['/config', '/tools', '/alerts', '/mapping', '/topology', '/reverse', '/']) {
      await router.push(path)
      assert.equal(router.currentRoute.value.path, path)
      assert.equal(router.resolve(path).href, base.replace(/\/$/, '') + path)
      assert.equal(document.title, `${router.currentRoute.value.meta.titleKey} - SmartPingNext`)
    }
    await router.push('/unknown/page')
    assert.equal(router.currentRoute.value.path, '/')
    assert.equal(router.resolve(router.currentRoute.value).href, base)
  }
})

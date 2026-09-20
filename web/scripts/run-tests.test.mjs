import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative } from 'node:path'
import test from 'node:test'
import { collectTestFiles } from './run-tests.mjs'

test('test discovery includes nested compiled and script tests without importing helpers', (t) => {
  const root = mkdtempSync(join(tmpdir(), 'smartping-tests-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  for (const path of ['.test-dist/api', 'scripts/nested', 'src', 'node_modules']) {
    mkdirSync(join(root, path), { recursive: true })
  }
  for (const path of [
    '.test-dist/api/new.test.js',
    'scripts/nested/new.test.mjs',
    '.test-dist/helper.js',
    'scripts/run-tests.mjs',
    'src/uncompiled.test.ts',
    'node_modules/foreign.test.js'
  ]) {
    writeFileSync(join(root, path), 'throw new Error("must not be imported during discovery")')
  }
  assert.deepEqual(
    collectTestFiles(root).map((path) => relative(root, path).replaceAll('\\', '/')),
    ['.test-dist/api/new.test.js', 'scripts/nested/new.test.mjs']
  )
})

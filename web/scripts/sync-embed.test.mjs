import assert from 'node:assert/strict'
import fs from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { syncEmbeddedFiles } from './sync-embed-files.mjs'

function fixture(t, previous = true) {
  const root = fs.mkdtempSync(join(tmpdir(), 'smartping-embed-test-'))
  t.after(() => fs.rmSync(root, { recursive: true, force: true }))
  const source = join(root, 'dist')
  const staticRoot = join(root, 'static')
  const destination = join(staticRoot, 'html')
  fs.mkdirSync(source)
  fs.mkdirSync(staticRoot)
  fs.writeFileSync(join(source, 'index.html'), 'new index')
  fs.writeFileSync(join(source, 'new.js'), 'new asset')
  if (previous) {
    fs.mkdirSync(destination)
    fs.writeFileSync(join(destination, 'index.html'), 'old index')
    fs.writeFileSync(join(destination, 'old.js'), 'old asset')
  }
  return { source, staticRoot, destination }
}

for (const previous of [false, true]) {
  test(`sync installs a complete build (previous=${previous})`, (t) => {
    const { source, staticRoot, destination } = fixture(t, previous)
    syncEmbeddedFiles(source, staticRoot)
    assert.deepEqual(fs.readdirSync(staticRoot), ['html'])
    assert.deepEqual(fs.readdirSync(destination), ['index.html', 'new.js'])
    assert.equal(fs.readFileSync(join(destination, 'index.html'), 'utf8'), 'new index')
  })
}

test('copy failure preserves the old build and cleans partial staging files', (t) => {
  const { source, staticRoot, destination } = fixture(t)
  t.mock.method(fs, 'cpSync', (_source, staged) => {
    fs.mkdirSync(staged)
    fs.writeFileSync(join(staged, 'partial.js'), 'partial')
    throw new Error('copy failed')
  })
  assert.throws(() => syncEmbeddedFiles(source, staticRoot), /copy failed/)
  assert.equal(fs.readFileSync(join(destination, 'index.html'), 'utf8'), 'old index')
  assert.deepEqual(fs.readdirSync(staticRoot), ['html'])
})

test('install failure restores the previous build', (t) => {
  const { source, staticRoot, destination } = fixture(t)
  const rename = fs.renameSync
  t.mock.method(fs, 'renameSync', (from, to) => {
    if (from.endsWith('next')) throw new Error('install failed')
    rename(from, to)
  })
  assert.throws(() => syncEmbeddedFiles(source, staticRoot), /install failed/)
  assert.equal(fs.readFileSync(join(destination, 'old.js'), 'utf8'), 'old asset')
  assert.deepEqual(fs.readdirSync(staticRoot), ['html'])
})

test('failed rollback retains the backup and reports its location', (t) => {
  const { source, staticRoot, destination } = fixture(t)
  const rename = fs.renameSync
  t.mock.method(fs, 'renameSync', (from, to) => {
    if (to === destination) throw new Error('destination locked')
    rename(from, to)
  })
  assert.throws(
    () => syncEmbeddedFiles(source, staticRoot),
    (error) => {
      assert.ok(error instanceof AggregateError)
      const temporary = fs.readdirSync(staticRoot).find((name) => name.startsWith('.html-sync-'))
      const backup = join(staticRoot, temporary, 'previous')
      assert.ok(error.message.includes(backup))
      assert.equal(fs.readFileSync(join(backup, 'index.html'), 'utf8'), 'old index')
      return true
    }
  )
})

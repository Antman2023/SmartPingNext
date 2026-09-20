import { readdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import process from 'node:process'

export function collectTestFiles(root) {
  const files = []
  const visit = (directory) => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name)
      if (entry.isDirectory()) visit(path)
      else if (entry.isFile() && /\.test\.(?:js|mjs)$/.test(entry.name)) files.push(path)
    }
  }
  visit(join(root, '.test-dist'))
  visit(join(root, 'scripts'))
  return files.sort()
}

const script = fileURLToPath(import.meta.url)
if (process.argv[1] && resolve(process.argv[1]) === script) {
  const webRoot = resolve(dirname(script), '..')
  const files = collectTestFiles(webRoot)
  if (files.length === 0) throw new Error('No frontend tests found.')
  const result = spawnSync(process.execPath, ['--test', ...files], {
    cwd: webRoot,
    stdio: 'inherit'
  })
  if (result.error) throw result.error
  process.exitCode = result.status ?? 1
}

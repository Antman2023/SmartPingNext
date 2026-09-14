import { spawnSync } from 'node:child_process'
import { readdirSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')

function findTests(directory, suffix) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return findTests(path, suffix)
    return entry.isFile() && entry.name.endsWith(suffix) ? [path] : []
  }).sort()
}

function runNode(args) {
  const result = spawnSync(process.execPath, args, { cwd: webRoot, stdio: 'inherit' })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}

const sourceRoot = join(webRoot, 'src')
const sourceTests = findTests(sourceRoot, '.test.ts')
const scriptTests = findTests(join(webRoot, 'scripts'), '.test.mjs')
if (sourceTests.length + scriptTests.length === 0) throw new Error('No tests found.')

runNode([join(webRoot, 'node_modules/typescript/bin/tsc'), '-p', 'tsconfig.test.json'])
// Select outputs from current sources, so removed tests cannot run from stale output.
const compiledTests = sourceTests.map((path) =>
  join(webRoot, '.test-dist', relative(sourceRoot, path).replace(/\.ts$/, '.js'))
)
runNode(['--test', ...compiledTests, ...scriptTests])

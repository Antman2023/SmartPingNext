import { realpathSync, rmSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const output = join(webRoot, '.test-dist')
// Only remove the generated test directory inside this checkout.
if (realpathSync(webRoot) !== webRoot || dirname(output) !== webRoot) {
  throw new Error('Test output must remain inside the frontend directory.')
}
rmSync(output, { recursive: true, force: true })

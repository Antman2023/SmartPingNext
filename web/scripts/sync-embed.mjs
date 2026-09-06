import { cpSync, realpathSync, rmSync, statSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import process from 'node:process'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const source = join(webRoot, 'dist')
const staticRoot = resolve(webRoot, '../src/static')
const destination = join(staticRoot, 'html')

// Check the build and its destination before replacing generated files.
if (!statSync(join(source, 'index.html')).isFile()) {
  throw new Error('Build the frontend before synchronizing embedded files.')
}
if (realpathSync(staticRoot) !== staticRoot) {
  throw new Error('The static directory must not redirect outside the project.')
}

rmSync(destination, { recursive: true, force: true })
cpSync(source, destination, { recursive: true })
process.stdout.write('Frontend synchronized to src/static/html.\n')

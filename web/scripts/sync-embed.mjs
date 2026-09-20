import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import process from 'node:process'
import { syncEmbeddedFiles } from './sync-embed-files.mjs'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const source = join(webRoot, 'dist')
const staticRoot = resolve(webRoot, '../src/static')
syncEmbeddedFiles(source, staticRoot)
process.stdout.write('Frontend synchronized to src/static/html.\n')

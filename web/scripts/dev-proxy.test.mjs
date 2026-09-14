import assert from 'node:assert/strict'
import { createServer as createHTTPServer } from 'node:http'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'
import { createServer } from 'vite'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')

test('development proxy preserves browser origin and host for configuration POSTs', async () => {
  const backend = createHTTPServer((request, response) => {
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify({ host: request.headers.host, origin: request.headers.origin }))
  })
  let vite
  try {
    await new Promise((resolve, reject) => {
      backend.once('error', reject)
      backend.listen(0, '127.0.0.1', resolve)
    })
    vite = await createServer({
      root: webRoot,
      configFile: resolve(webRoot, 'vite.config.ts'),
      logLevel: 'silent',
      server: {
        host: '127.0.0.1',
        port: 0,
        hmr: false,
        watch: null,
        proxy: { '/api': { target: `http://127.0.0.1:${backend.address().port}` } }
      },
      optimizeDeps: { noDiscovery: true, include: [] }
    })
    await vite.listen()
    const address = `127.0.0.1:${vite.httpServer.address().port}`
    const origin = `http://${address}`
    for (const path of ['/api/verify-password.json', '/api/saveconfig.json']) {
      for (const requestOrigin of [origin, 'https://external.example']) {
        const response = await fetch(`${origin}${path}`, {
          method: 'POST',
          headers: { Origin: requestOrigin, 'Content-Type': 'application/x-www-form-urlencoded' },
          body: 'password=example',
          signal: AbortSignal.timeout(5000)
        })
        assert.equal(response.status, 200)
        assert.deepEqual(await response.json(), { host: address, origin: requestOrigin })
      }
    }
  } finally {
    await vite?.close()
    backend.closeAllConnections()
    await new Promise((resolve) => backend.close(resolve))
  }
})

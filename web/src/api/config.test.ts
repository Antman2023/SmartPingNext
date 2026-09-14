import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import axios from 'axios'
import ts from 'typescript'

const source = readFileSync(new URL('../../src/api/config.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

async function verificationClient(
  t: test.TestContext,
  handler: (request: IncomingMessage, response: ServerResponse) => void,
  timeout = 1000
) {
  const server = createServer(handler)
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
  t.after(async () => {
    server.closeAllConnections()
    await new Promise<void>((resolve) => server.close(() => resolve()))
  })
  const address = server.address()
  assert.ok(address && typeof address !== 'string')
  const exports = {} as { verifyPassword: (password: string, signal: AbortSignal) => Promise<string> }
  runInNewContext(compiled, {
    exports,
    URLSearchParams,
    require: (name: string) => {
      if (name === 'axios') return { default: axios }
      assert.equal(name, './index')
      return { default: { defaults: { baseURL: `http://127.0.0.1:${address.port}/custom-api`, timeout } } }
    }
  })
  return exports.verifyPassword
}

test('password verification uses configured API path and preserves form encoding', async (t) => {
  let body = ''
  let path = ''
  let contentType = ''
  const verify = await verificationClient(t, (request, response) => {
    path = request.url ?? ''
    contentType = request.headers['content-type'] ?? ''
    request.setEncoding('utf8')
    request.on('data', (chunk: string) => { body += chunk })
    request.on('end', () => { response.end('{"status":"true"}') })
  })
  assert.equal(await verify('p&=中', new AbortController().signal), 'valid')
  assert.equal(path, '/custom-api/verify-password.json')
  assert.match(contentType, /^application\/x-www-form-urlencoded/)
  assert.equal(new URLSearchParams(body).get('password'), 'p&=中')
})

test('password verification distinguishes invalid credentials, rate limits and server failures', async (t) => {
  let status = 200
  let body = '{"status":"false"}'
  const verify = await verificationClient(t, (request, response) => {
    request.resume()
    response.statusCode = status
    response.end(body)
  })
  const signal = new AbortController().signal
  assert.equal(await verify('password', signal), 'invalid')
  body = 'null'
  assert.equal(await verify('password', signal), 'invalid')
  status = 429
  assert.equal(await verify('password', signal), 'rate-limited')
  status = 503
  await assert.rejects(verify('password', signal), (error: unknown) =>
    axios.isAxiosError(error) && error.response?.status === 503)
})

test('password verification times out when the server does not respond', async (t) => {
  const verify = await verificationClient(t, (request) => { request.resume() }, 100)
  await assert.rejects(verify('password', new AbortController().signal), (error: unknown) =>
    axios.isAxiosError(error) && error.code === 'ECONNABORTED')
})

test('password verification cancels an active request', async (t) => {
  const controller = new AbortController()
  const verify = await verificationClient(t, (request) => {
    request.resume()
    controller.abort()
  })
  await assert.rejects(verify('password', controller.signal), (error: unknown) => axios.isCancel(error))
})

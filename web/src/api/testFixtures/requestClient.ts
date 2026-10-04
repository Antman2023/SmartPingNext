import { readFileSync } from 'node:fs'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import type { TestContext } from 'node:test'
import { runInNewContext } from 'node:vm'
import axios, { type AxiosInstance } from 'axios'
import ts from 'typescript'
import { normalizeConfigResponse } from '../../utils/configResponse.js'
import { isRequestTimeout } from '../../utils/requestErrors.js'
import { isRequestCanceled, normalizeRejectedRequest } from '../../utils/requestCancellation.js'
import { normalizeRequestTimeout, resolveProxyClientTimeout } from '../../utils/requestTimeouts.js'
import type { Config, ToolsResult } from '../../types/index.js'

const compile = (path: string) => ts.transpileModule(
  readFileSync(new URL(path, import.meta.url), 'utf8').replace(/import\.meta\.env/g, 'testEnvironment'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }
).outputText
const errorModule = compile('../../../src/utils/error.ts')
const clientModule = compile('../../../src/api/index.ts')
const toolsModule = compile('../../../src/api/tools.ts')
const configModule = compile('../../../src/api/config.ts')

export async function createRequestClientFixture(
  t: TestContext,
  handler: (request: IncomingMessage, response: ServerResponse) => void
) {
  const server = createServer(handler)
  await new Promise<void>((resolve) => { server.listen(0, '127.0.0.1', resolve) })
  t.after(async () => {
    server.closeAllConnections()
    await new Promise<void>((resolve) => { server.close(() => resolve()) })
  })
  const address = server.address() as AddressInfo
  const testEnvironment = { VITE_API_BASE_URL: `http://127.0.0.1:${address.port}/api`, VITE_API_TIMEOUT: '15000' }
  const dependencies: Record<string, unknown> = {
    axios: { default: axios },
    'element-plus': { ElMessage: {} },
    '@/locales': { default: { global: { t: (key: string) => key } } },
    '@/utils/requestErrors': { isRequestTimeout },
    '@/utils/requestCancellation': { isRequestCanceled, normalizeRejectedRequest },
    '@/utils/requestTimeouts': { normalizeRequestTimeout, resolveProxyClientTimeout },
    '@/utils/configResponse': { normalizeConfigResponse }
  }
  const evaluate = <T>(source: string): T => {
    const exports = {}
    runInNewContext(source, {
      exports, testEnvironment, Error, URL, URLSearchParams,
      require: (name: string) => {
        if (!(name in dependencies)) throw new Error(`Unexpected dependency: ${name}`)
        return dependencies[name]
      }
    })
    return exports as T
  }
  const errors = evaluate<{ handleNetworkError: (error: unknown) => Error }>(errorModule)
  dependencies['@/utils/error'] = errors
  const client = evaluate<{ default: AxiosInstance }>(clientModule)
  dependencies['./index'] = client
  const tools = evaluate<{
    runTools: (baseUrl: string, target: string, signal?: AbortSignal) => Promise<ToolsResult>
  }>(toolsModule)
  const config = evaluate<{
    verifyConfigPassword: (password: string, signal?: AbortSignal) => Promise<string>
    saveConfig: (config: Config, password: string, signal?: AbortSignal) => Promise<unknown>
  }>(configModule)
  return { request: client.default, ...tools, ...config }
}

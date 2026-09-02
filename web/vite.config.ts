import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import { cwd } from 'process'
import { fileURLToPath } from 'url'
import { dirname } from 'path'

const __dirname = dirname(fileURLToPath(import.meta.url))

const elementPlusCssLayer = () => ({
  name: 'element-plus-css-layer',
  enforce: 'pre' as const,
  transform(code: string, id: string) {
    const cleanId = id.split('?', 1)[0].replace(/\\/g, '/')
    if (!cleanId.includes('/element-plus/theme-chalk/') || !cleanId.endsWith('.css')) {
      return null
    }

    return {
      code: `@layer element-plus {\n${code}\n}`,
      map: null
    }
  }
})

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, cwd(), '')
  const proxyTarget = env.VITE_PROXY_TARGET || 'http://localhost:8899'

  return {
    plugins: [elementPlusCssLayer(), vue()],
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src')
      }
    },
    server: {
      port: 3000,
      proxy: {
        '/api': {
          target: proxyTarget,
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: 'dist',
      assetsDir: 'assets',
      chunkSizeWarningLimit: 1500,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (
              id.includes('/node_modules/vue/') ||
              id.includes('/node_modules/vue-router/') ||
              id.includes('/node_modules/pinia/')
            ) {
              return 'vue-vendor'
            }
          }
        }
      }
    }
  }
})

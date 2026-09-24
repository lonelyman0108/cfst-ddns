import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'
import { mkdirSync, writeFileSync } from 'node:fs'

const backend = 'http://127.0.0.1:8080'

// 构建后重新写入 dist/.gitkeep，保证 Go 在未构建前端时也能通过 go:embed 编译
function keepDist(): Plugin {
  return {
    name: 'keep-dist-gitkeep',
    apply: 'build',
    closeBundle() {
      const dir = fileURLToPath(new URL('./dist', import.meta.url))
      mkdirSync(dir, { recursive: true })
      writeFileSync(dir + '/.gitkeep', '')
    },
  }
}

export default defineConfig({
  base: '/',
  plugins: [
    vue(),
    tailwindcss(),
    keepDist(),
  ],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: backend,
        changeOrigin: true,
        timeout: 0,
        proxyTimeout: 0,
        configure: (proxy) => {
          // SSE 不缓冲
          proxy.on('proxyRes', (proxyRes) => {
            if (String(proxyRes.headers['content-type'] || '').includes('text/event-stream')) {
              proxyRes.headers['cache-control'] = 'no-cache'
              proxyRes.headers['x-accel-buffering'] = 'no'
            }
          })
        },
      },
      '/healthz': { target: backend, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (/[\\/](echarts|zrender|vue-echarts)[\\/]/.test(id)) return 'echarts'
          if (/[\\/](reka-ui|@floating-ui|@vueuse|@internationalized|@lucide|vue-sonner)[\\/]/.test(id)) return 'ui'
          return 'vendor'
        },
      },
    },
  },
})

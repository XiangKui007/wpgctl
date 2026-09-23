import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

const src = resolve(__dirname, 'src')

export default defineConfig({
  plugins: [vue()],
  base: '/',
  resolve: {
    alias: {
      '@': src,
    },
  },
  build: {
    outDir: resolve(__dirname, '../internal/ui/dist'),
    emptyOutDir: true,
    // 静态目录带产品名：现场经业务 Nginx 8877 反代时，根级 /assets 太通用易与其他前端撞路径
    assetsDir: 'wpg-deploy-assets',
    rollupOptions: {
      output: {
        chunkFileNames: 'wpg-deploy-assets/[name]-[hash].js',
        entryFileNames: 'wpg-deploy-assets/[name]-[hash].js',
        assetFileNames: 'wpg-deploy-assets/[name]-[hash][extname]',
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('codemirror') || id.includes('@codemirror')) return 'codemirror'
          if (
            id.includes('element-plus') ||
            id.includes('@element-plus') ||
            id.includes('vue-router') ||
            /[/\\]vue[/\\]/.test(id) ||
            id.includes('@vue/')
          ) {
            return 'vendor'
          }
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:9527',
      '/wpg-deploy-api': 'http://127.0.0.1:9527',
    },
  },
})

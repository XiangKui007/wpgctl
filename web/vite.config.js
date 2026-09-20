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
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        chunkFileNames: 'assets/[name]-[hash].js',
        entryFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash][extname]',
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
    },
  },
})

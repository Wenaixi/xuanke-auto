import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  base: '/',
  build: {
    outDir: '../backend/web/dist', // 构建产物直接输出到后端 embed 目录
    emptyOutDir: true,
  },
  server: {
    proxy: { '/api': 'http://localhost:8080' }, // 开发时代理到后端
  },
})

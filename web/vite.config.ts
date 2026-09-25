import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

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
    proxy: { '/api': 'http://localhost:3091' }, // 开发时代理到后端 3091 端口
  },
  // C3 测试地基：纯函数 + hook 逻辑测试，无需 jsdom
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})

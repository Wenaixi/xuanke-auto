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
    // woff2 一律不内联：默认 4096 字节阈值会把中文字体里若干小分片转成 base64 写进 CSS，
    // 使 unicode-range 的按需加载语义失效——base64 内联进 CSS 就等于无条件下载。
    // 返回 false 明确不内联，返回 undefined 表示交回默认逻辑。
    assetsInlineLimit: (filePath) =>
      filePath.endsWith('.woff2') ? false : undefined,
  },
  server: {
    proxy: { '/api': 'http://localhost:3091' }, // 开发时代理到后端 3091 端口
  },
  // 测试地基：纯函数 + hook 逻辑测试，无需 jsdom
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})

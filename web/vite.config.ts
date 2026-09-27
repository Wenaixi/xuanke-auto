import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'
import type { Plugin } from 'vite'

// misans-vf 自带的 98 条 @font-face 全都漏写 font-weight 描述符 → 浏览器按默认
// normal(400) 匹配，500/600 会被合成「假粗体」（笔画糊、不是真字重）。
// 这里只在构建期给它补上可变字重区间，让 font-medium / font-semibold 真正驱动 wght 轴
// （该字体 wght 轴实测 150–700，100–900 描述符由浏览器线性映射）。
// 只匹配 misans-vf 的文件，不碰任何其它 CSS。
const misansWeightRange: Plugin = {
  name: 'misans-vf-weight-range',
  enforce: 'pre',
  transform(code, id) {
    if (!id.includes('misans-vf') || !id.endsWith('.css')) return null
    if (!code.includes('@font-face')) return null
    return code.replace(/@font-face\s*\{/g, '@font-face{font-weight:100 900;')
  },
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    misansWeightRange,
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

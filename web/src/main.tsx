import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// 界面字体：Geist（SIL OFL 1.1，Vercel 出品）可变无衬线，与 xAI 规范的
// universalSans（私有、不可获取）气质最接近，也和 Geist Mono 同源。
// Geist 不含汉字，中文由系统中文字体兜底（见 global.css 的 --font-sans 栈）。
// 不再自托管霞鹜文楷：楷体与冷峻单色体系不搭，且其 194 个中文分片占约 8.9MiB。
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import './styles/global.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

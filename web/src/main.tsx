import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
// 中文：MiSans VF（小米，免费商用）。98 条 @font-face 分片；包内 CSS 漏写 font-weight，
// 由 vite.config.ts 的 misans-vf-weight-range 插件在构建期补上可变区间（否则 500/600 是假粗体）。
import 'misans-vf/lib/MiSans.min.css'
import './styles/global.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

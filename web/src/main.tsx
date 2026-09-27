import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/geist-mono'
// 中英数统一字体：MiSans（小米，免费商用）——同一套 98 片分片里既有汉字也有拉丁与数字。
// 包内 CSS 漏写 font-weight，由 vite.config.ts 的 misans-vf-weight-range 插件在构建期补区间
// （否则 500/600 会被合成假粗体）。等宽数字（倒计时/ID）仍走上面的 Geist Mono。
import 'misans-vf/lib/MiSans.min.css'
import './styles/global.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource-variable/geist-mono'
// 界面字体：霞鹜文楷（SIL OFL 1.1，免费商用）。只引 400/700 两个字重——300 用不到，
// 省下约 5MB；只给这里两个入口是为了让未使用的字重分片不进构建产物。
import 'lxgw-wenkai-webfont/lxgwwenkai-regular.css'
import 'lxgw-wenkai-webfont/lxgwwenkai-bold.css'
import './styles/global.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)

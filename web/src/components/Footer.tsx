// 仓库地址是页脚里唯一的事实来源：换仓库只改这一行。
const REPO_URL = "https://github.com/Wenaixi/xuanke-auto"

// GitHub 标记内联 SVG：本项目的 lucide-react 版本已移除品牌图标（无 Github 导出），
// 为一个 16×16 的图案引依赖不值得，内联 path 最省事（fill 走 currentColor，随文字色变）。
function GitHubMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={className} aria-hidden="true" focusable="false" fill="currentColor">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  )
}

/**
 * 站点页脚（全站共用，在 App 层渲染一次）。
 * 形态刻意极简：发丝上边框 + 冷灰小字，不套卡片——页脚不该再叠一层面板底色。
 * 桌面左右分栏、窄屏上下堆叠顺序不变（窄屏优先露出仓库入口）。
 */
export function Footer() {
  return (
    <footer className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8 mt-10 pb-8">
      <div className="border-t border-neutral-900 pt-5 flex flex-col sm:flex-row items-center justify-between gap-2 text-2xs text-neutral-400">
        <div className="flex items-center gap-3">
          <a
            href={REPO_URL}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1.5 hover:text-white active:text-white transition-colors"
            title="在 GitHub 上查看源码"
          >
            <GitHubMark className="h-3.5 w-3.5" />
            <span>github.com/Wenaixi/xuanke-auto</span>
          </a>
          <span className="text-neutral-600">·</span>
          <span>MIT License</span>
        </div>
        <div className="flex items-center gap-3">
          <span>开发者：炼天</span>
          <span className="text-neutral-600">·</span>
          <span>© 2026 至道选课</span>
        </div>
      </div>
    </footer>
  )
}

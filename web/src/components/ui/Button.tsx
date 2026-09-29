import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cn } from "../../lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean
  variant?: "default" | "primary" | "success" | "warning" | "destructive" | "outline" | "secondary" | "ghost" | "invert" | "dark"
  size?: "default" | "sm" | "lg" | "icon"
}

// 每个变体都必须有 active 态，且**按下比悬停更强烈**：触屏没有 hover，
// 全局 CSS 又主动关掉了 -webkit-tap-highlight-color，若只留 hover，手指按下去
// 等于没有任何反馈（用户原话「点下去不确定点上没」）。active 是移动端唯一的即时反馈。
// 另一条同源规则：**凡是 hover 会变的属性，active 必须变同一个属性**。此前只在 hover 里
// 写 text/border 变白，桌面鼠标移上去会白、启动端（Android WebView）点下去毫无变化
// ——用户原话「鼠标放上去可以变白，但是启动端点击不行」。故 hover:xxx 一律配 active:xxx。
const variantStyles: Record<NonNullable<ButtonProps["variant"]>, string> = {
  default: "bg-[var(--surface-soft)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--surface-muted)] hover:border-[var(--border-hover)] active:bg-black/50 active:border-[var(--border-hover)]",
  primary: "bg-white text-black border border-white font-semibold hover:bg-neutral-200 active:bg-neutral-300 shadow-sm",
  success: "bg-neutral-100 text-black border border-white font-semibold hover:bg-white active:bg-neutral-300 shadow-sm",
  warning: "bg-neutral-800 text-white border border-neutral-700 font-medium hover:bg-neutral-700 active:bg-neutral-600 active:border-neutral-500",
  destructive: "bg-rose-950/40 text-rose-300 border border-rose-900/60 font-medium hover:bg-rose-900/50 active:bg-rose-900/70 active:border-rose-700",
  // outline：文字取纯白（原 --fg-muted 灰）——选课页的「返回 / 按剩余排序 / 显示全部」
  // 都走这个变体，主人要求这些字是纯白而不是灰字。
  outline: "bg-transparent text-[var(--fg)] border border-[var(--border)] hover:text-white hover:border-[var(--border-hover)] hover:bg-[var(--surface-soft)] active:text-white active:border-white active:bg-white/15",
  secondary: "bg-[var(--surface)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--surface-soft)] hover:border-[var(--border-hover)] active:bg-white/15 active:border-white",
  ghost: "bg-transparent text-[var(--fg-muted)] border-transparent hover:bg-[var(--surface-soft)] hover:text-white active:bg-white/15 active:text-white",
  invert: "bg-white text-black border border-white font-semibold hover:bg-neutral-200 active:bg-neutral-300 shadow-sm",
  // dark：曾是 bg-black 实色底（唯一不随 --surface 走的按钮变体，全站透明化后它就是唯一的黑块）。
  // 改透明玻璃 + 更亮的描边：靠边框与全大写白字保持存在感，而不是靠一块实色面板。
  dark: "glass text-white border border-white/35 font-medium hover:border-white/70 hover:bg-white/10 active:border-white active:bg-white/15",
}

// 触控目标按「移动端优先、桌面回收」写：手指精度远低于鼠标，32px 的小按钮在手机上
// 必然点空或点到隔壁（公认舒适线 44px、可接受下限 40px）；而桌面鼠标精确、密度优先，
// 故 sm 断点起回到原紧凑档。这是移动端与桌面唯一分叉的尺寸规则，全站按钮一次收敛。
// 按钮一律胶囊（xAI 规范的 rounded.pill = 9999px，规范原文「Every button is a pill」）。
// 与面板直角并存的形态两轨制：面板/卡片/输入框用 --radius-*（已归零），
// 按钮用 rounded-full。二者的分界是「是否可点」，不是尺寸。
const sizeStyles: Record<NonNullable<ButtonProps["size"]>, string> = {
  default: "h-11 sm:h-10 px-4 py-2 text-sm rounded-full",
  sm: "h-10 sm:h-8 px-3.5 sm:px-3 text-xs rounded-full",
  lg: "h-12 px-6 text-base rounded-full",
  icon: "h-11 w-11 sm:h-10 sm:w-10 p-0 flex items-center justify-center rounded-full",
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"
    return (
      <Comp
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center gap-2 select-none font-medium transition-all duration-150 active:scale-[0.97] disabled:opacity-40 disabled:pointer-events-none disabled:active:scale-100",
          // 键盘焦点可见性：global.css「基础交互重置」对 button 统一 outline:none 抹掉
          // 原生 focus，此处补 focus-visible ring 作补偿——Tab 键盘导航的焦点环可见，
          // 鼠标点击不显示（focus-visible 语义），全站 Button 一次收敛（无障碍基线）。
          // ring-offset 必须为 0：偏移量会在胶囊外侧留一圈方角描边，破坏胶囊轮廓。
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-0",
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
        {...props}
      />
    )
  }
)
Button.displayName = "Button"

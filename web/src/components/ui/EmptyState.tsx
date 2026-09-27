import * as React from "react"
import { cn } from "../../lib/utils"

/** 空态/失败态统一样式（透明磨砂 + 虚线发丝边框），与 Dashboard「预选目标课程」空态同源。
 *  icon 传图标元素、action 传出口按钮（重试 / 前往挑选…），无出口时省略。
 *  加载态不用本组件——短暂过程态保持轻量纯文本，避免每次查询抖动都闪一张卡片。 */
export function EmptyState({
  icon,
  action,
  className,
  children,
}: {
  icon?: React.ReactNode
  action?: React.ReactNode
  className?: string
  children: React.ReactNode
}) {
  return (
    <div
      className={cn(
        "rounded-[var(--radius-lg)] border border-neutral-800 border-dashed glass p-8 text-center flex flex-col items-center justify-center gap-3",
        className
      )}
    >
      {icon}
      <p className="text-xs text-neutral-400">{children}</p>
      {action}
    </div>
  )
}

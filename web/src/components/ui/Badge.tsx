import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "success" | "warning" | "destructive" | "primary" | "outline" | "secondary" | "active"
}

const badgeVariants: Record<NonNullable<BadgeProps["variant"]>, string> = {
  default: "bg-white/10 text-neutral-100 border-neutral-700",
  // primary/active：关键状态徽章。此前是白底黑字，是页面上唯一的「黑字」元素——
  // 主人反馈过「已确认选课怎么这么黑」并要求「关键的地方要是白字，不要都是灰字」。
  // 现改为提亮玻璃 + 白字：仍是最高强调档（底色更亮、边框更亮、字重加粗），但不再有黑字。
  primary: "bg-white/20 text-white border-white/50 font-semibold",
  success: "bg-white/10 text-neutral-100 border-neutral-600",
  warning: "bg-white/10 text-neutral-200 border-neutral-700",
  destructive: "bg-white/10 text-neutral-300 border-neutral-700",
  outline: "bg-transparent text-neutral-300 border-neutral-700",
  secondary: "bg-white/10 text-neutral-200 border-neutral-700",
  active: "bg-white/20 text-white border-white/50 font-semibold",
}

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-1.5 px-2.5 py-0.5 text-xs font-medium border rounded-[var(--radius-sm)] transition-colors select-none",
        badgeVariants[variant],
        className
      )}
      {...props}
    />
  )
}

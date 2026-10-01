import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "success" | "primary" | "outline"
}

// 徽章文字一律纯白：夹在 6% 半透明面板上的小字原本是 100/200/300 三档灰，
// 主人要求「关键的地方要是白字，不要都是灰字」——灰阶在这里没有信息价值，
// 强调差异交给底色与边框深浅（primary 最亮、outline 最淡）。
const badgeVariants: Record<NonNullable<BadgeProps["variant"]>, string> = {
  default: "bg-white/10 text-white border-neutral-700",
  primary: "bg-white/20 text-white border-white/50 font-semibold",
  success: "bg-white/10 text-white border-neutral-600",
  outline: "bg-transparent text-white border-neutral-700",
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

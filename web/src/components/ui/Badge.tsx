import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "outline" | "secondary" | "active"
}

const badgeVariants: Record<NonNullable<BadgeProps["variant"]>, string> = {
  default: "bg-[var(--fg)] text-[var(--bg)] border border-[var(--fg)]",
  outline: "bg-transparent text-[var(--fg-muted)] border border-[var(--border)]",
  secondary: "bg-[var(--surface-soft)] text-[var(--fg-dim)] border border-[var(--border)]",
  active: "bg-[var(--surface)] text-[var(--fg)] border border-[var(--border-invert)]",
}

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center px-2 py-0.5 text-[10px] font-mono uppercase tracking-widest transition-colors select-none",
        badgeVariants[variant],
        className
      )}
      {...props}
    />
  )
}

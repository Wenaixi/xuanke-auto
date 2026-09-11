import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "success" | "warning" | "destructive" | "primary" | "outline" | "secondary" | "active"
}

const badgeVariants: Record<NonNullable<BadgeProps["variant"]>, string> = {
  default: "bg-[var(--surface-soft)] text-[var(--fg)] border-[var(--border)]",
  primary: "bg-[var(--cyan-bg)] text-[var(--cyan)] border-[var(--cyan-border)]",
  success: "bg-[var(--emerald-bg)] text-[var(--emerald)] border-[var(--emerald-border)]",
  warning: "bg-[var(--amber-bg)] text-[var(--amber)] border-[var(--amber-border)]",
  destructive: "bg-[var(--rose-bg)] text-[var(--rose)] border-[var(--rose-border)]",
  outline: "bg-transparent text-[var(--fg-muted)] border-[var(--border)]",
  secondary: "bg-[var(--surface-muted)] text-[var(--fg-muted)] border-transparent",
  active: "bg-[var(--emerald-bg)] text-[var(--emerald)] border-[var(--emerald-border)] shadow-[0_0_10px_rgba(16,185,129,0.2)]",
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

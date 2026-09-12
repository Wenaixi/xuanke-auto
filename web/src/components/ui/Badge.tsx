import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "success" | "warning" | "destructive" | "primary" | "outline" | "secondary" | "active"
}

const badgeVariants: Record<NonNullable<BadgeProps["variant"]>, string> = {
  default: "bg-white/10 text-neutral-100 border-neutral-700",
  primary: "bg-white text-black border-white font-semibold",
  success: "bg-white/10 text-neutral-100 border-neutral-600",
  warning: "bg-white/10 text-neutral-200 border-neutral-700",
  destructive: "bg-white/10 text-neutral-300 border-neutral-700",
  outline: "bg-transparent text-neutral-300 border-neutral-700",
  secondary: "bg-white/10 text-neutral-200 border-neutral-700",
  active: "bg-white text-black border-white font-semibold",
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

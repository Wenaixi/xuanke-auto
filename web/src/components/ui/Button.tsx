import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cn } from "../../lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean
  variant?: "default" | "primary" | "success" | "warning" | "destructive" | "outline" | "secondary" | "ghost" | "invert" | "dark"
  size?: "default" | "sm" | "lg" | "icon"
}

const variantStyles: Record<NonNullable<ButtonProps["variant"]>, string> = {
  default: "bg-[var(--surface-soft)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--surface-muted)] hover:border-[var(--border-hover)]",
  primary: "bg-white text-black border border-white font-semibold hover:bg-neutral-200 shadow-sm",
  success: "bg-neutral-100 text-black border border-white font-semibold hover:bg-white shadow-sm",
  warning: "bg-neutral-800 text-white border border-neutral-700 font-medium hover:bg-neutral-700",
  destructive: "bg-rose-950/40 text-rose-300 border border-rose-900/60 font-medium hover:bg-rose-900/50",
  outline: "bg-transparent text-[var(--fg-muted)] border border-[var(--border)] hover:text-[var(--fg)] hover:border-[var(--border-hover)] hover:bg-[var(--surface-soft)]",
  secondary: "bg-[var(--surface)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--surface-soft)] hover:border-[var(--border-hover)]",
  ghost: "bg-transparent text-[var(--fg-muted)] border-transparent hover:bg-[var(--surface-soft)] hover:text-[var(--fg)]",
  invert: "bg-white text-black border border-white font-semibold hover:bg-neutral-200 shadow-sm",
  dark: "bg-black text-white border border-white/25 font-medium hover:bg-neutral-900 hover:border-white/50",
}

const sizeStyles: Record<NonNullable<ButtonProps["size"]>, string> = {
  default: "h-10 px-4 py-2 text-sm rounded-[var(--radius-md)]",
  sm: "h-8 px-3 text-xs rounded-[var(--radius-sm)]",
  lg: "h-11 md:h-12 px-6 text-base rounded-[var(--radius-md)]",
  icon: "h-10 w-10 p-0 flex items-center justify-center rounded-[var(--radius-md)]",
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"
    return (
      <Comp
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center gap-2 select-none font-medium transition-all duration-150 active:scale-[0.98] disabled:opacity-40 disabled:pointer-events-none disabled:active:scale-100",
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

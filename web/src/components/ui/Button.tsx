import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cn } from "../../lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean
  variant?: "default" | "primary" | "success" | "warning" | "destructive" | "outline" | "secondary" | "ghost" | "invert"
  size?: "default" | "sm" | "lg" | "icon"
}

const variantStyles: Record<NonNullable<ButtonProps["variant"]>, string> = {
  default: "bg-[var(--surface-soft)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--surface-muted)] hover:border-[var(--border-hover)] shadow-sm",
  primary: "bg-[var(--cyan)] text-white border border-[var(--cyan)] font-medium hover:brightness-110 shadow-sm",
  success: "bg-[var(--emerald)] text-white border border-[var(--emerald)] font-medium hover:brightness-110 shadow-sm",
  warning: "bg-[var(--amber)] text-black border border-[var(--amber)] font-medium hover:brightness-110 shadow-sm",
  destructive: "bg-[var(--rose)] text-white border border-[var(--rose)] font-medium hover:brightness-110 shadow-sm",
  outline: "bg-transparent text-[var(--fg-muted)] border border-[var(--border)] hover:text-[var(--fg)] hover:border-[var(--border-hover)] hover:bg-[var(--surface)]",
  secondary: "bg-[var(--surface)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--surface-soft)] hover:border-[var(--border-hover)]",
  ghost: "bg-transparent text-[var(--fg-muted)] border-transparent hover:bg-[var(--surface-soft)] hover:text-[var(--fg)]",
  invert: "bg-[var(--fg)] text-[var(--bg)] border border-[var(--fg)] font-medium hover:bg-white/90 shadow-sm",
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

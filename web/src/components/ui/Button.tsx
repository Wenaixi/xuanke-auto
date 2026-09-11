import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cn } from "../../lib/utils"

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean
  variant?: "default" | "outline" | "invert" | "secondary" | "ghost" | "destructive"
  size?: "default" | "sm" | "lg" | "icon"
}

const variantStyles: Record<NonNullable<ButtonProps["variant"]>, string> = {
  default: "bg-[var(--surface)] text-[var(--fg)] border border-[var(--border)] hover:bg-[var(--fg)] hover:text-[var(--bg)] hover:border-[var(--fg)]",
  outline: "bg-transparent text-[var(--fg-muted)] border border-[var(--border)] hover:text-[var(--fg)] hover:border-[var(--border-strong)]",
  invert: "bg-[var(--fg)] text-[var(--bg)] border border-[var(--fg)] font-medium hover:bg-transparent hover:text-[var(--fg)]",
  secondary: "bg-[var(--surface-soft)] text-[var(--fg)] border border-[var(--border)] hover:border-[var(--border-strong)]",
  ghost: "bg-transparent text-[var(--fg-muted)] border-transparent hover:bg-[var(--surface-soft)] hover:text-[var(--fg)]",
  destructive: "bg-[var(--surface)] text-[var(--fg)] border border-[var(--border-strong)] hover:bg-white hover:text-black",
}

const sizeStyles: Record<NonNullable<ButtonProps["size"]>, string> = {
  default: "h-9 px-4 py-2 text-sm",
  sm: "h-7 px-3 text-xs tracking-wider",
  lg: "h-11 px-6 text-base tracking-widest",
  icon: "h-9 w-9 p-0 flex items-center justify-center",
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button"
    return (
      <Comp
        ref={ref}
        className={cn(
          "inline-flex items-center justify-center uppercase tracking-wider select-none font-normal transition-colors duration-150 disabled:opacity-30 disabled:pointer-events-none",
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

import * as React from "react"
import { cn } from "../../lib/utils"

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        type={type}
        ref={ref}
        className={cn(
          "flex h-9 w-full bg-[var(--surface)] px-3 py-1 text-sm text-[var(--fg)] border border-[var(--border)] transition-colors duration-150 placeholder:text-[var(--fg-dim)] focus:border-[var(--border-invert)] focus:outline-none disabled:cursor-not-allowed disabled:opacity-40",
          className
        )}
        {...props}
      />
    )
  }
)
Input.displayName = "Input"

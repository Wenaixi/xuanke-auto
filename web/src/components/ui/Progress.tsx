import * as React from "react"
import { cn } from "../../lib/utils"

export interface ProgressProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: number
  max?: number
  indicatorColor?: "emerald" | "amber" | "rose" | "cyan" | "default"
}

export const Progress = React.forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, value = 0, max = 100, indicatorColor = "default", ...props }, ref) => {
    const percentage = Math.min(Math.max((value / max) * 100, 0), 100)

    const colorMap = {
      default: "bg-[var(--cyan)]",
      cyan: "bg-[var(--cyan)]",
      emerald: "bg-[var(--emerald)]",
      amber: "bg-[var(--amber)]",
      rose: "bg-[var(--rose)]",
    }

    return (
      <div
        ref={ref}
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        className={cn("relative h-2.5 w-full overflow-hidden rounded-full bg-[var(--surface-muted)]", className)}
        {...props}
      >
        <div
          className={cn("h-full transition-all duration-300 ease-out rounded-full", colorMap[indicatorColor])}
          style={{ width: `${percentage}%` }}
        />
      </div>
    )
  }
)
Progress.displayName = "Progress"

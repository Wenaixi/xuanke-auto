import * as React from 'react'

const cn = (...cls: (string | undefined | false)[]) => cls.filter(Boolean).join(' ')

export const Input = React.forwardRef<HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => {
    return <input ref={ref} className={cn('px-3 py-2 border bg-transparent', className)} {...props} />
  },
)
Input.displayName = 'Input'

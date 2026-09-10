import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'

const cn = (...cls: (string | undefined | false)[]) => cls.filter(Boolean).join(' ')

export const Button = React.forwardRef<HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement> & { asChild?: boolean }>(
  ({ className, asChild, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button'
    return <Comp ref={ref} className={cn('px-4 py-2 border transition-colors', className)} {...props} />
  },
)
Button.displayName = 'Button'

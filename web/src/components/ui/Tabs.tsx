import * as React from "react"
import * as TabsPrimitive from "@radix-ui/react-tabs"
import { cn } from "../../lib/utils"

export const Tabs = TabsPrimitive.Root

export const TabsList = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.List
    ref={ref}
    className={cn(
      // 窄屏（管理页 5 个标签 ≈450px）超出容器时横向滚动，不再被 body 的
      // overflow-x:hidden 裁掉导致末尾标签点不到；末尾标签半露本身就是可滑动线索。
      // 隐藏滚动条：6px 条会挤占 h-10 栏高并制造视觉噪音。
      "inline-flex h-10 max-w-full items-center justify-start rounded-[var(--radius-md)] bg-[var(--surface)] p-1 text-[var(--fg-muted)] border border-[var(--border)] select-none overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
      className
    )}
    {...props}
  />
))
TabsList.displayName = TabsPrimitive.List.displayName

export const TabsTrigger = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Trigger
    ref={ref}
    className={cn(
      // shrink-0：标签栏可横滑后 flex 项默认会被压缩，去掉后标签保持自然宽度并溢出滚动
      "inline-flex shrink-0 items-center justify-center whitespace-nowrap rounded-[var(--radius-sm)] px-3 sm:px-4 py-1.5 text-xs sm:text-sm font-medium transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--cyan)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--bg)] disabled:pointer-events-none disabled:opacity-40",
      "data-[state=active]:bg-[var(--surface-soft)] data-[state=active]:text-[var(--fg)] data-[state=active]:shadow-sm data-[state=active]:border data-[state=active]:border-[var(--border-hover)]",
      className
    )}
    {...props}
  />
))
TabsTrigger.displayName = TabsPrimitive.Trigger.displayName

export const TabsContent = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Content
    ref={ref}
    className={cn("mt-4 focus-visible:outline-none", className)}
    {...props}
  />
))
TabsContent.displayName = TabsPrimitive.Content.displayName

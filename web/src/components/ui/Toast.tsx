import * as React from "react"
import * as ToastPrimitive from "@radix-ui/react-toast"
import { X, CheckCircle2, AlertCircle, Info } from "lucide-react"
import { cn } from "../../lib/utils"

export type ToastVariant = "default" | "success" | "warning" | "destructive"

export interface ToastMessage {
  id: string
  title?: React.ReactNode
  description?: React.ReactNode
  variant?: ToastVariant
}

interface ToastContextValue {
  toast: (msg: Omit<ToastMessage, "id">) => void
}

const ToastContext = React.createContext<ToastContextValue>({
  toast: () => {},
})

export function useToast() {
  return React.useContext(ToastContext)
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = React.useState<ToastMessage[]>([])
  // F5-02：toast 自增 id 计数器（跨渲染保持，无碰撞）。
  const idRef = React.useRef(0)

  const toast = React.useCallback((msg: Omit<ToastMessage, "id">) => {
    // F5-02：唯一 id 用自增计数器而非 Math.random 7 位短串——
    // 同一页 20 分钟内高频提示（满员退避/轮询失败）Math.random 碰撞会让 React key 重复、
    // 状态异常（一条 toast 被误删），自增 id 从根上消除碰撞概率。
    idRef.current += 1
    setToasts((prev) => [...prev, { ...msg, id: `t-${idRef.current}` }])
  }, [])

  const removeToast = React.useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  return (
    <ToastContext.Provider value={{ toast }}>
      <ToastPrimitive.Provider swipeDirection="right">
        {children}
        {/* F48-M1：定位类必须在 viewport 上而非外层空壳——Radix Toast.Root 全部
            createPortal 进 viewport（wrapper 渲染完是空壳 div），且 viewport 无任何
            inset 类时 toast 落点交给浏览器「无 inset fixed 元素」的 static-position
            行为（当前视觉正常属碰巧稳定）。布局并入 viewport className，删空壳 wrapper。 */}
        {toasts.map(({ id, title, description, variant = "default" }) => (
          <ToastPrimitive.Root
            key={id}
            onOpenChange={(open) => {
              if (!open) removeToast(id)
            }}
            duration={3500}
            className={cn(
              "pointer-events-auto relative flex w-full items-start gap-3 overflow-hidden rounded-[var(--radius-lg)] border p-4 shadow-xl transition-all duration-200",
              "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-80 data-[state=closed]:slide-out-to-right-full data-[state=open]:slide-in-from-bottom-full",
              variant === "default" && "bg-[var(--surface)] text-[var(--fg)] border-[var(--border-hover)]",
              variant === "success" && "bg-[var(--surface)] text-[var(--fg)] border-[var(--emerald-border)] shadow-[0_4px_20px_rgba(16,185,129,0.15)]",
              variant === "warning" && "bg-[var(--surface)] text-[var(--fg)] border-[var(--amber-border)] shadow-[0_4px_20px_rgba(245,158,11,0.15)]",
              variant === "destructive" && "bg-[var(--surface)] text-[var(--fg)] border-[var(--rose-border)] shadow-[0_4px_20px_rgba(244,63,94,0.15)]"
            )}
          >
            <div className="mt-0.5 shrink-0">
              {variant === "success" && <CheckCircle2 className="h-5 w-5 text-[var(--emerald)]" />}
              {variant === "warning" && <AlertCircle className="h-5 w-5 text-[var(--amber)]" />}
              {variant === "destructive" && <AlertCircle className="h-5 w-5 text-[var(--rose)]" />}
              {variant === "default" && <Info className="h-5 w-5 text-[var(--cyan)]" />}
            </div>
            <div className="grid gap-1 flex-1">
              {title && <ToastPrimitive.Title className="text-sm font-semibold leading-tight">{title}</ToastPrimitive.Title>}
              {description && (
                <ToastPrimitive.Description className="text-xs text-[var(--fg-muted)] leading-relaxed">
                  {description}
                </ToastPrimitive.Description>
              )}
            </div>
            <ToastPrimitive.Close className="shrink-0 rounded-[var(--radius-sm)] p-1 text-[var(--fg-muted)] opacity-70 transition-opacity hover:opacity-100 hover:text-[var(--fg)] focus:outline-none">
              <X className="h-4 w-4" />
            </ToastPrimitive.Close>
          </ToastPrimitive.Root>
        ))}
        <ToastPrimitive.Viewport className="fixed bottom-4 right-4 z-50 flex w-full max-w-[380px] flex-col gap-2 p-4 pointer-events-none outline-none" />
      </ToastPrimitive.Provider>
    </ToastContext.Provider>
  )
}

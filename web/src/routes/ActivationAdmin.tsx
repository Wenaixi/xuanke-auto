import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type { ActivationCode } from "../types"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "../components/ui/Dialog"
import { Badge } from "../components/ui/Badge"
import {
  KeyRound,
  Plus,
  Trash2,
  X,
  Copy,
  Check,
  Loader2,
  ShieldCheck,
  RefreshCw,
} from "lucide-react"
import { useToast } from "../components/ui/Toast"

interface Props {
  open: boolean
  onClose: () => void
}

const ADMIN_TOKEN_KEY = "xk_admin_token"

function loadAdminToken(): string {
  try {
    return localStorage.getItem(ADMIN_TOKEN_KEY) || ""
  } catch {
    return ""
  }
}

export default function ActivationAdmin({ open, onClose }: Props) {
  const { toast } = useToast()
  const [adminToken, setAdminToken] = useState(loadAdminToken)
  const [tokenError, setTokenError] = useState("")

  // 生成参数
  const [count, setCount] = useState(5)
  const [uses, setUses] = useState(1)
  const [generating, setGenerating] = useState(false)
  const [generated, setGenerated] = useState<string[]>([])
  const [copied, setCopied] = useState("")

  const codesQuery = useQuery({
    queryKey: ["admin-codes", adminToken],
    queryFn: () => api<ActivationCode[]>("/admin/codes", { headers: { "X-Admin-Token": adminToken } }),
    enabled: open && !!adminToken,
    refetchInterval: 5000,
  })

  const adminError = codesQuery.error
    ? (codesQuery.error as { code?: number; message: string }).message
    : ""
  const unauthorized = adminError.includes("管理口令错误") || (codesQuery.error as any)?.code === 403

  const saveToken = () => {
    if (!adminToken.trim()) {
      setTokenError("请输入管理口令")
      return
    }
    localStorage.setItem(ADMIN_TOKEN_KEY, adminToken.trim())
    setTokenError("")
    codesQuery.refetch()
  }

  const clearToken = () => {
    localStorage.removeItem(ADMIN_TOKEN_KEY)
    setAdminToken("")
    setTokenError("")
  }

  const generate = async () => {
    setGenerating(true)
    try {
      const codes = await api<string[]>("/admin/codes", {
        method: "POST",
        headers: { "X-Admin-Token": adminToken },
        body: JSON.stringify({ count, uses }),
      })
      setGenerated(codes)
      codesQuery.refetch()
      toast({ title: "激活码已生成", description: `本次生成 ${codes.length} 个，每个可用 ${uses} 次`, variant: "default" })
    } catch (e: any) {
      toast({ title: "生成失败", description: e.message || "通信异常", variant: "destructive" })
    } finally {
      setGenerating(false)
    }
  }

  const remove = async (code: string) => {
    try {
      await api("/admin/codes", {
        method: "DELETE",
        headers: { "X-Admin-Token": adminToken },
        body: JSON.stringify({ code }),
      })
      codesQuery.refetch()
    } catch (e: any) {
      toast({ title: "删除失败", description: e.message || "通信异常", variant: "destructive" })
    }
  }

  const copy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(code)
      setTimeout(() => setCopied(""), 1500)
    } catch {
      toast({ title: "复制失败", description: "浏览器未授权剪贴板", variant: "destructive" })
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <div className="flex items-center gap-2">
            <KeyRound className="h-4 w-4 text-[var(--fg-muted)]" />
            <span className="text-xs uppercase tracking-wider font-mono text-[var(--fg-muted)]">
              ACTIVATION CODES
            </span>
          </div>
          <DialogTitle className="pt-1">激活码管理</DialogTitle>
          <DialogDescription>
            生成可分配多次使用的激活码，账号激活一次后永久免激活
          </DialogDescription>
        </DialogHeader>

        {/* 管理口令区 */}
        {!adminToken ? (
          <div className="space-y-3 py-2">
            <div className="flex flex-col gap-1.5">
              <label className="text-xs text-[var(--fg-muted)] flex items-center gap-1.5">
                <ShieldCheck className="h-3.5 w-3.5" />
                <span>管理口令（服务启动时 XUANKE_ADMIN_TOKEN 设置）</span>
              </label>
              <div className="flex gap-2">
                <Input
                  type="password"
                  value={adminToken}
                  onChange={(e) => setAdminToken(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && saveToken()}
                  placeholder="输入管理口令"
                  autoFocus
                  className="h-10 text-sm bg-[var(--surface-soft)] border-[var(--border)] text-[var(--fg)] placeholder:text-[var(--fg-dim)] focus:border-white"
                />
                <Button variant="primary" size="sm" onClick={saveToken} className="h-10 px-4 text-xs">
                  解锁
                </Button>
              </div>
            </div>
            {tokenError && <p className="text-xs text-[var(--fg)]">{tokenError}</p>}
          </div>
        ) : (
          <div className="space-y-4 py-1">
            {/* 管理口令已解锁行 */}
            <div className="flex items-center justify-between text-xs text-[var(--fg-muted)] font-mono border-b border-[var(--border)] pb-3">
              <span className="flex items-center gap-2">
                <ShieldCheck className="h-3.5 w-3.5 text-[var(--fg)]" />
                <span>管理口令已解锁</span>
              </span>
              <button onClick={clearToken} className="text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors">
                更换口令
              </button>
            </div>

            {unauthorized && (
              <div className="p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--surface-soft)] text-xs text-[var(--fg)]">
                管理口令错误或已失效，请重新输入
              </div>
            )}

            {/* 生成区 */}
            <div className="flex flex-col sm:flex-row items-end gap-3 p-3 rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface-soft)]">
              <div className="flex-1 flex flex-col gap-1.5">
                <label className="text-xs text-[var(--fg-muted)]">生成数量（1-100）</label>
                <Input
                  type="number"
                  min={1}
                  max={100}
                  value={count}
                  onChange={(e) => setCount(Math.max(1, Math.min(100, Number(e.target.value) || 1)))}
                  className="h-10 text-sm bg-[var(--surface)] border-[var(--border)] text-[var(--fg)]"
                />
              </div>
              <div className="flex-1 flex flex-col gap-1.5">
                <label className="text-xs text-[var(--fg-muted)]">每个可用次数</label>
                <Input
                  type="number"
                  min={1}
                  value={uses}
                  onChange={(e) => setUses(Math.max(1, Number(e.target.value) || 1))}
                  className="h-10 text-sm bg-[var(--surface)] border-[var(--border)] text-[var(--fg)]"
                />
              </div>
              <Button
                variant="primary"
                size="sm"
                onClick={generate}
                disabled={generating || unauthorized}
                className="h-10 px-5 text-xs flex items-center gap-1.5"
              >
                {generating ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin text-black" />
                ) : (
                  <Plus className="h-3.5 w-3.5 text-black" />
                )}
                <span>生成</span>
              </Button>
            </div>

            {/* 本次生成的激活码（便于复制分发） */}
            {generated.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs text-[var(--fg-muted)] font-mono">
                  <span>本次生成 {generated.length} 个</span>
                  <button onClick={() => setGenerated([])} className="hover:text-[var(--fg)] transition-colors">
                    收起
                  </button>
                </div>
                <div className="max-h-36 overflow-y-auto space-y-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] p-3">
                  {generated.map((c) => (
                    <div key={c} className="flex items-center justify-between text-xs font-mono">
                      <span className="text-[var(--fg)] tracking-widest">{c}</span>
                      <button
                        onClick={() => copy(c)}
                        className="flex items-center gap-1 text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors"
                      >
                        {copied === c ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                        <span>{copied === c ? "已复制" : "复制"}</span>
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* 全部激活码列表 */}
            <div className="text-xs font-mono text-[var(--fg-muted)] flex items-center justify-between">
              <span>全部激活码</span>
              <button onClick={() => codesQuery.refetch()} className="flex items-center gap-1 hover:text-[var(--fg)] transition-colors">
                <RefreshCw className="h-3 w-3" />
                刷新
              </button>
            </div>

            {codesQuery.isLoading ? (
              <div className="p-8 text-center text-xs text-[var(--fg-dim)]">加载中...</div>
            ) : codesQuery.data && codesQuery.data.length > 0 ? (
              <div className="max-h-64 overflow-y-auto rounded-[var(--radius-lg)] border border-[var(--border)] divide-y divide-[var(--border)]">
                {codesQuery.data.map((c) => {
                  const exhausted = c.used_uses >= c.total_uses
                  return (
                    <div key={c.code} className="flex items-center justify-between gap-2 px-3 py-2.5 text-xs">
                      <div className="flex items-center gap-3 min-w-0">
                        <span className="font-mono tracking-widest text-[var(--fg)] truncate">{c.code}</span>
                        <Badge variant={exhausted ? "outline" : "primary"} className="text-[10px] shrink-0">
                          {c.used_uses}/{c.total_uses}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        <span className="text-[10px] text-[var(--fg-dim)] font-mono hidden sm:inline">
                          {c.created_at}
                        </span>
                        <button
                          onClick={() => copy(c.code)}
                          className="p-1 text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors"
                          title="复制"
                        >
                          {copied === c.code ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                        </button>
                        <button
                          onClick={() => remove(c.code)}
                          className="p-1 text-[var(--fg-muted)] hover:text-[var(--fg)] transition-colors"
                          title="删除"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="p-8 text-center text-xs text-[var(--fg-dim)] rounded-[var(--radius-lg)] border border-dashed border-[var(--border)]">
                暂无激活码，生成后即可分发
              </div>
            )}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={onClose} className="text-xs flex items-center gap-1.5">
            <X className="h-3.5 w-3.5" />
            <span>关闭</span>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
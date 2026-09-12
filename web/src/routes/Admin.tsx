import { useEffect, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { api } from "../api/client"
import type {
  Account,
  ActivationCode,
  AdminAccount,
  AdminConfig,
  AdminLog,
  AdminStats,
} from "../types"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "../components/ui/Tabs"
import { useToast } from "../components/ui/Toast"
import {
  Activity,
  Copy,
  Check,
  LogOut,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Loader2,
  KeyRound,
  Settings,
  Users,
  FileText,
  ArrowLeft,
} from "lucide-react"

interface Props {
  account: Account
  sessionToken: string
  onLogout: () => void
  onBackToStudent: () => void
}

export default function Admin({ sessionToken, onLogout, onBackToStudent }: Props) {
  const [copied, setCopied] = useState("")
  const { toast } = useToast()

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
    <div className="min-h-screen text-white p-4 sm:p-6 lg:p-8 select-none">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-neutral-900 pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <h1 className="text-lg sm:text-xl font-medium tracking-tight text-white">
                系统管理
              </h1>
              <Badge variant="primary" className="text-[10px] uppercase font-mono tracking-wider">
                ADMIN
              </Badge>
            </div>
            <p className="text-xs text-neutral-400">
              集中管理运行配置、激活码、账号与调度日志，修改立即生效
            </p>
          </div>
          <div className="flex items-center gap-2.5">
            <Button
              variant="outline"
              size="sm"
              onClick={onBackToStudent}
              className="flex items-center gap-1.5 text-xs text-neutral-400 hover:text-white"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>学生端</span>
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={onLogout}
              className="flex items-center gap-1.5 text-xs text-neutral-400 hover:text-white"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>退出</span>
            </Button>
          </div>
        </header>

        <Tabs defaultValue="codes">
          <TabsList>
            <TabsTrigger value="codes" className="flex items-center gap-1.5">
              <KeyRound className="h-3.5 w-3.5" />
              <span>激活码</span>
            </TabsTrigger>
            <TabsTrigger value="config" className="flex items-center gap-1.5">
              <Settings className="h-3.5 w-3.5" />
              <span>系统配置</span>
            </TabsTrigger>
            <TabsTrigger value="stats" className="flex items-center gap-1.5">
              <Activity className="h-3.5 w-3.5" />
              <span>运行状态</span>
            </TabsTrigger>
            <TabsTrigger value="accounts" className="flex items-center gap-1.5">
              <Users className="h-3.5 w-3.5" />
              <span>账号管理</span>
            </TabsTrigger>
            <TabsTrigger value="logs" className="flex items-center gap-1.5">
              <FileText className="h-3.5 w-3.5" />
              <span>日志总览</span>
            </TabsTrigger>
          </TabsList>

          <TabsContent value="codes">
            <CodesTab sessionToken={sessionToken} onCopy={copy} copied={copied} />
          </TabsContent>
          <TabsContent value="config">
            <ConfigTab sessionToken={sessionToken} />
          </TabsContent>
          <TabsContent value="stats">
            <StatsTab sessionToken={sessionToken} />
          </TabsContent>
          <TabsContent value="accounts">
            <AccountsTab sessionToken={sessionToken} />
          </TabsContent>
          <TabsContent value="logs">
            <LogsTab sessionToken={sessionToken} />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  )
}

// ---- 激活码管理 ----

function CodesTab({
  sessionToken,
  onCopy,
  copied,
}: {
  sessionToken: string
  onCopy: (c: string) => void
  copied: string
}) {
  const { toast } = useToast()
  const [count, setCount] = useState(5)
  const [uses, setUses] = useState(1)
  const [generating, setGenerating] = useState(false)
  const [generated, setGenerated] = useState<string[]>([])

  const codesQuery = useQuery({
    queryKey: ["admin-codes", sessionToken],
    queryFn: () => api<ActivationCode[]>("/admin/codes", { session: sessionToken }),
    refetchInterval: 5000,
  })

  const generate = async () => {
    setGenerating(true)
    try {
      const codes = await api<string[]>("/admin/codes", {
        method: "POST",
        session: sessionToken,
        body: JSON.stringify({ count, uses }),
      })
      setGenerated(codes)
      codesQuery.refetch()
      toast({ title: "激活码已生成", description: `本次生成 ${codes.length} 个，每个可用 ${uses} 次` })
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
        session: sessionToken,
        body: JSON.stringify({ code }),
      })
      codesQuery.refetch()
    } catch (e: any) {
      toast({ title: "删除失败", description: e.message || "通信异常", variant: "destructive" })
    }
  }

  return (
    <div className="space-y-4">
      {/* 生成区 */}
      <div className="flex flex-col sm:flex-row items-end gap-3 p-3 rounded-[var(--radius-lg)] border border-neutral-900 glass">
        <div className="flex-1 flex flex-col gap-1.5">
          <label className="text-xs text-neutral-400">生成数量（1-100）</label>
          <Input
            type="number"
            min={1}
            max={100}
            value={count}
            onChange={(e) => setCount(Math.max(1, Math.min(100, Number(e.target.value) || 1)))}
            className="h-10 text-sm glass-input border-neutral-800 text-white"
          />
        </div>
        <div className="flex-1 flex flex-col gap-1.5">
          <label className="text-xs text-neutral-400">每个可用次数</label>
          <Input
            type="number"
            min={1}
            value={uses}
            onChange={(e) => setUses(Math.max(1, Number(e.target.value) || 1))}
            className="h-10 text-sm glass-input border-neutral-800 text-white"
          />
        </div>
        <Button
          variant="primary"
          size="sm"
          onClick={generate}
          disabled={generating}
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

      {/* 本次生成 */}
      {generated.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-neutral-500 font-mono">
            <span>本次生成 {generated.length} 个</span>
            <button onClick={() => setGenerated([])} className="hover:text-white transition-colors">
              收起
            </button>
          </div>
          <div className="max-h-36 overflow-y-auto space-y-1.5 rounded-[var(--radius-sm)] border border-neutral-900 p-3">
            {generated.map((c) => (
              <div key={c} className="flex items-center justify-between text-xs font-mono">
                <span className="text-white tracking-widest">{c}</span>
                <button
                  onClick={() => onCopy(c)}
                  className="flex items-center gap-1 text-neutral-500 hover:text-white transition-colors"
                >
                  {copied === c ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                  <span>{copied === c ? "已复制" : "复制"}</span>
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 全部激活码 */}
      <div className="text-xs font-mono text-neutral-500 flex items-center justify-between">
        <span>全部激活码</span>
        <button onClick={() => codesQuery.refetch()} className="flex items-center gap-1 hover:text-white transition-colors">
          <RefreshCw className="h-3 w-3" />
          刷新
        </button>
      </div>

      {codesQuery.isLoading ? (
        <div className="p-8 text-center text-xs text-neutral-600">加载中...</div>
      ) : codesQuery.data && codesQuery.data.length > 0 ? (
        <div className="max-h-64 overflow-y-auto rounded-[var(--radius-lg)] border border-neutral-900 divide-y divide-neutral-900">
          {codesQuery.data.map((c) => {
            const exhausted = c.used_uses >= c.total_uses
            return (
              <div key={c.code} className="flex items-center justify-between gap-2 px-3 py-2.5 text-xs">
                <div className="flex items-center gap-3 min-w-0">
                  <span className="font-mono tracking-widest text-white truncate">{c.code}</span>
                  <Badge variant={exhausted ? "outline" : "primary"} className="text-[10px] shrink-0">
                    {c.used_uses}/{c.total_uses}
                  </Badge>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <span className="text-[10px] text-neutral-600 font-mono hidden sm:inline">{c.created_at}</span>
                  <button onClick={() => onCopy(c.code)} className="p-1 text-neutral-500 hover:text-white transition-colors" title="复制">
                    {copied === c.code ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                  </button>
                  <button onClick={() => remove(c.code)} className="p-1 text-neutral-500 hover:text-white transition-colors" title="删除">
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        <div className="p-8 text-center text-xs text-neutral-600 rounded-[var(--radius-lg)] border border-dashed border-neutral-900">
          暂无激活码，生成后即可分发
        </div>
      )}
    </div>
  )
}

// ---- 系统配置 ----

function ConfigTab({ sessionToken }: { sessionToken: string }) {
  const { toast } = useToast()
  const [baseUrl, setBaseUrl] = useState("")
  const [apiKey, setApiKey] = useState("")
  const [model, setModel] = useState("")
  const [engine, setEngine] = useState("vision")
  const [concurrency, setConcurrency] = useState(1)
  const [openTime, setOpenTime] = useState("")
  const [activationOn, setActivationOn] = useState(true)
  const [saving, setSaving] = useState(false)
  const initializedRef = useRef(false)

  const configQuery = useQuery({
    queryKey: ["admin-config", sessionToken],
    queryFn: () => api<AdminConfig>("/admin/config", { session: sessionToken }),
  })

  // 后端配置加载后仅回填一次表单（key 为脱敏掩码，保存时留空=不改）
  const loaded = configQuery.data
  useEffect(() => {
    if (loaded && !initializedRef.current) {
      initializedRef.current = true
      setBaseUrl(loaded.vision_base_url)
      setModel(loaded.vision_model)
      setEngine(loaded.captcha_engine || "vision")
      setConcurrency(loaded.captcha_concurrency || 1)
      setOpenTime(loaded.open_time)
      setActivationOn(loaded.activation_enabled)
    }
  }, [loaded])

  const save = async () => {
    setSaving(true)
    try {
      const body: Record<string, unknown> = {
        activation_enabled: activationOn,
        vision_base_url: baseUrl.trim(),
        vision_model: model.trim(),
        captcha_engine: engine,
        captcha_concurrency: Math.max(1, concurrency || 1),
        open_time: openTime.trim(),
      }
      // 留空 = 不改动 key（脱敏回显无法完整回填）
      if (apiKey.trim()) body.vision_api_key = apiKey.trim()
      await api("/admin/config", { method: "PUT", session: sessionToken, body: JSON.stringify(body) })
      setApiKey("")
      configQuery.refetch()
      toast({ title: "配置已保存", description: "已生效，无需重启服务" })
    } catch (e: any) {
      toast({ title: "保存失败", description: e.message || "通信异常", variant: "destructive" })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4">
      <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass shadow-none">
        <CardHeader className="pb-2 border-b border-neutral-900">
          <div className="flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-neutral-400" />
            <CardTitle className="text-sm font-medium tracking-wide text-white">激活码机制</CardTitle>
          </div>
          <CardDescription className="text-xs text-neutral-500">
            关闭后登录直接进入系统，激活与激活码管理接口全部停用
          </CardDescription>
        </CardHeader>
        <CardContent className="pt-3 flex items-center justify-between">
          <span className="text-xs text-neutral-400">当前状态</span>
          <button
            onClick={() => setActivationOn(!activationOn)}
            className={`w-11 h-6 rounded-full border transition-colors relative ${
              activationOn ? "bg-white border-white" : "glass-input border-neutral-700"
            }`}
            aria-pressed={activationOn}
          >
            <span
              className={`absolute top-0.5 h-5 w-5 rounded-full transition-all ${
                activationOn ? "left-[22px] bg-black" : "left-0.5 bg-neutral-500"
              }`}
            />
          </button>
        </CardContent>
      </Card>

      <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass shadow-none">
        <CardHeader className="pb-2 border-b border-neutral-900">
          <CardTitle className="text-sm font-medium tracking-wide text-white">验证码识别</CardTitle>
          <CardDescription className="text-xs text-neutral-500">
            OpenAI 兼容多模态接口，用于教务登录验证码自动识别
          </CardDescription>
        </CardHeader>
        <CardContent className="pt-3 grid gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-neutral-400">接口地址</label>
            <Input
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="https://api.siliconflow.cn/v1"
              className="h-10 text-sm font-mono glass-input border-neutral-800 text-white placeholder:text-neutral-600"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-neutral-400">
              密钥{loaded?.vision_api_key_masked ? `（当前 ${loaded.vision_api_key_masked}，留空不改）` : ""}
            </label>
            <Input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="sk-..."
              className="h-10 text-sm glass-input border-neutral-800 text-white placeholder:text-neutral-600"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-neutral-400">模型</label>
            <Input
              value={model}
              onChange={(e) => setModel(e.target.value)}
              placeholder="Qwen/Qwen3-VL-30B-A3B-Instruct"
              className="h-10 text-sm font-mono glass-input border-neutral-800 text-white placeholder:text-neutral-600"
            />
          </div>
        </CardContent>
      </Card>

      <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass shadow-none">
        <CardHeader className="pb-2 border-b border-neutral-900">
          <CardTitle className="text-sm font-medium tracking-wide text-white">识别引擎与并发</CardTitle>
          <CardDescription className="text-xs text-neutral-500">
            ddddocr 走本机 Python 识别（免 API 密钥）；并发上限默认 1（串行识别防平台熔断）
          </CardDescription>
        </CardHeader>
        <CardContent className="pt-3 grid gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-neutral-400">识别引擎</label>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setEngine("vision")}
                className={`flex-1 h-10 rounded-[var(--radius-sm)] border text-xs transition-colors ${
                  engine === "vision"
                    ? "border-white bg-white text-black font-medium"
                    : "border-neutral-800 glass-input text-neutral-400 hover:text-white"
                }`}
              >
                硅基流动 Vision（云）
              </button>
              <button
                onClick={() => setEngine("ddddocr")}
                className={`flex-1 h-10 rounded-[var(--radius-sm)] border text-xs transition-colors ${
                  engine === "ddddocr"
                    ? "border-white bg-white text-black font-medium"
                    : "border-neutral-800 glass-input text-neutral-400 hover:text-white"
                }`}
              >
                本地 ddddocr（离线）
              </button>
            </div>
            <p className="text-[11px] text-neutral-600 mt-0.5">
              {engine === "ddddocr"
                ? "切换后将校验本机 Python + ddddocr 环境，缺失自动回退 Vision"
                : "需在后台填写硅基流动接口地址、密钥与模型"}
            </p>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-neutral-400">识别并发上限（默认 1，串行）</label>
            <Input
              type="number"
              min={1}
              max={16}
              value={concurrency}
              onChange={(e) => setConcurrency(Math.max(1, Math.min(16, Number(e.target.value) || 1)))}
              className="h-10 text-sm font-mono glass-input border-neutral-800 text-white"
            />
          </div>
        </CardContent>
      </Card>

      <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass shadow-none">
        <CardHeader className="pb-2 border-b border-neutral-900">
          <CardTitle className="text-sm font-medium tracking-wide text-white">选课开放时间</CardTitle>
          <CardDescription className="text-xs text-neutral-500">调度器按此时间切换探测节奏并自动抢报</CardDescription>
        </CardHeader>
        <CardContent className="pt-3">
          <Input
            value={openTime}
            onChange={(e) => setOpenTime(e.target.value)}
            placeholder="2026-09-13 09:00:00"
            className="h-10 text-sm font-mono glass-input border-neutral-800 text-white placeholder:text-neutral-600"
          />
        </CardContent>
      </Card>

      <Button variant="primary" size="sm" onClick={save} disabled={saving} className="h-10 px-5 text-xs">
        {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin text-black" /> : null}
        <span>保存并生效</span>
      </Button>
    </div>
  )
}

// ---- 运行状态 ----

function StatsTab({ sessionToken }: { sessionToken: string }) {
  const statsQuery = useQuery({
    queryKey: ["admin-stats", sessionToken],
    queryFn: () => api<AdminStats>("/admin/stats", { session: sessionToken }),
    refetchInterval: 5000,
  })
  const s = statsQuery.data

  const rows: { label: string; value: string }[] = s
    ? [
        { label: "开放时间", value: s.open_time },
        { label: "窗口状态", value: s.window_opened ? "已开放" : "待命中" },
        { label: "激活码机制", value: s.activation_on ? "开启" : "关闭" },
        { label: "账号数", value: String(s.account_count) },
        { label: "预选目标", value: String(s.targets_count) },
        { label: "已选成功", value: String(s.success_count) },
        { label: "日志条数", value: String(s.log_count) },
        { label: "识别模型", value: s.vision_model },
      ]
    : []

  return (
    <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass shadow-none">
      <CardHeader className="pb-2 border-b border-neutral-900">
        <div className="flex items-center gap-2">
          <Activity className="h-4 w-4 text-neutral-400" />
          <CardTitle className="text-sm font-medium tracking-wide text-white">系统运行状态</CardTitle>
        </div>
        <CardDescription className="text-xs text-neutral-500">每 5 秒自动刷新</CardDescription>
      </CardHeader>
      <CardContent className="pt-3">
        {!s ? (
          <div className="p-8 text-center text-xs text-neutral-600">加载中...</div>
        ) : (
          <div className="divide-y divide-neutral-900">
            {rows.map((r) => (
              <div key={r.label} className="py-2.5 flex items-center justify-between text-xs">
                <span className="text-neutral-400">{r.label}</span>
                <span className="text-white font-mono tabular-nums">{r.value}</span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ---- 账号管理 ----

function AccountsTab({ sessionToken }: { sessionToken: string }) {
  const { toast } = useToast()
  const accountsQuery = useQuery({
    queryKey: ["admin-accounts", sessionToken],
    queryFn: () => api<AdminAccount[]>("/admin/accounts", { session: sessionToken }),
    refetchInterval: 10000,
  })

  const remove = async (acct: string) => {
    if (!window.confirm(`删除账号 ${acct}？将清空其凭据、目标与成功记录（日志保留）`)) return
    try {
      await api("/admin/accounts", {
        method: "DELETE",
        session: sessionToken,
        body: JSON.stringify({ account: acct }),
      })
      accountsQuery.refetch()
      toast({ title: "已删除", description: `账号 ${acct} 已移除` })
    } catch (e: any) {
      toast({ title: "删除失败", description: e.message || "通信异常", variant: "destructive" })
    }
  }

  return (
    <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass shadow-none overflow-hidden">
      <CardHeader className="pb-2 border-b border-neutral-900">
        <CardTitle className="text-sm font-medium tracking-wide text-white">账号管理</CardTitle>
        <CardDescription className="text-xs text-neutral-500">全部账号的目标课程与已选成功记录</CardDescription>
      </CardHeader>
      {accountsQuery.isLoading ? (
        <div className="p-8 text-center text-xs text-neutral-600">加载中...</div>
      ) : accountsQuery.data && accountsQuery.data.length > 0 ? (
        <div className="overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr className="border-b border-neutral-900">
                <th className="h-10 px-4 text-left align-middle font-medium text-[11px] uppercase tracking-wider text-neutral-500 select-none">账号</th>
                <th className="h-10 px-4 text-left align-middle font-medium text-[11px] uppercase tracking-wider text-neutral-500 select-none">目标课程</th>
                <th className="h-10 px-4 text-left align-middle font-medium text-[11px] uppercase tracking-wider text-neutral-500 select-none">已选成功</th>
                <th className="h-10 px-4 text-right align-middle font-medium text-[11px] uppercase tracking-wider text-neutral-500 select-none">操作</th>
              </tr>
            </thead>
            <tbody>
              {accountsQuery.data.map((a) => {
                // 防御：后端 Go 切片未赋值序列化为 JSON null（非空数组），此处统一兜底
                const targets = a.targets ?? []
                const success = a.success ?? []
                return (
                  <tr key={a.account} className="border-b border-neutral-900 hover:bg-white/5 transition-colors">
                    <td className="p-3 sm:p-4 align-middle text-white font-mono">{a.account}</td>
                    <td className="p-3 sm:p-4 align-middle text-neutral-300 tabular-nums">
                      {targets.length > 0 ? (
                        <span className="flex flex-wrap gap-1.5">
                          {targets.map((t) => (
                            <Badge key={t.class_id} variant="outline" className="text-[10px] font-mono">
                              {t.course_name || t.class_id}
                              {(t.priority ?? 0) > 0 ? ` #${t.priority}` : ""}
                            </Badge>
                          ))}
                        </span>
                      ) : (
                        <span className="text-neutral-600 text-xs">未设置</span>
                      )}
                    </td>
                    <td className="p-3 sm:p-4 align-middle text-white font-mono tabular-nums">
                      {success.length > 0 ? success.join(", ") : <span className="text-neutral-600 text-xs">-</span>}
                    </td>
                  <td className="p-3 sm:p-4 align-middle text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => remove(a.account)}
                      className="text-xs text-neutral-400 hover:text-white hover:border-white"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      删除
                    </Button>
                  </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="p-8 text-center text-xs text-neutral-600">暂无账号</div>
      )}
    </Card>
  )
}

// ---- 日志总览 ----

function LogsTab({ sessionToken }: { sessionToken: string }) {
  const logsQuery = useQuery({
    queryKey: ["admin-logs", sessionToken],
    queryFn: () => api<AdminLog[]>("/admin/logs?limit=200", { session: sessionToken }),
    refetchInterval: 5000,
  })
  const logs = logsQuery.data

  return (
    <Card className="rounded-[var(--radius-lg)] border border-neutral-900 glass overflow-hidden shadow-none">
      <CardHeader className="pb-2 border-b border-neutral-900">
        <CardTitle className="text-sm font-medium tracking-wide text-white">日志总览</CardTitle>
        <CardDescription className="text-xs text-neutral-500">
          全账号调度日志流 {logs ? `（最近 ${logs.length} 条）` : ""}
        </CardDescription>
      </CardHeader>
      <div className="p-4 max-h-[28rem] overflow-y-auto text-xs space-y-2">
        {logs && logs.length > 0 ? (
          logs.map((l) => (
            <div key={l.id} className="flex flex-col sm:flex-row sm:items-baseline gap-1.5 sm:gap-4 border-b border-neutral-900/60 pb-2 font-mono">
              <span className="text-neutral-600 text-[11px] shrink-0">{l.created_at}</span>
              <span className="text-white shrink-0 font-medium">{l.account}</span>
              <span className={`shrink-0 font-medium ${l.is_ok ? "text-white" : "text-neutral-400"}`}>
                [{l.action}]
              </span>
              <span className="text-neutral-400 break-all">{l.result}</span>
            </div>
          ))
        ) : (
          <div className="py-8 text-center text-neutral-600 font-mono">NO LOGS</div>
        )}
      </div>
    </Card>
  )
}

import { useEffect, useRef, useState } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
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
  BookMarked,
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
  AlertTriangle,
} from "lucide-react"

interface Props {
  account: Account
  sessionToken: string
  onLogout: () => void
  onBackToStudent: () => void
  onSelectAccount?: (acct: string) => void
  // M28-01：管理员删除账号后通知 App 做本地会话快照式清除——
  // 后端 DeleteAccount 只清服务端 6 表，前端 localStorage 的 xk_sessions 若不同步
  // 移除，被删账号会继续占账号槽位、首次刷新复活，直到下次请求 401 才被吊销链
  // 摘除（"删账号绝不残留"的整套工程理念与后端四段防线对齐，见 App.tsx onDeleted）。
  onDeleted?: (acct: string) => void
}

export default function Admin({ account, sessionToken, onLogout, onBackToStudent, onSelectAccount, onDeleted }: Props) {
  const [copied, setCopied] = useState("")
  // F12-M2：Tabs 受控化——defaultValue 只在首次挂载生效，管理员 Tab 间
  // 切换后状态现场保留；受控 value 只决定激活项，不破坏 Radix Tabs 键盘 roving focus。
  // 注：进出学生大厅（Admin 卸载重挂）后 useMemo 仍会重置为"codes"——如需跨挂载保留
  // 需提升到 App 层或 localStorage（见 review-round13 F13-M2）。
  const [activeTab, setActiveTab] = useState("codes")
  // n8：复制反馈定时器句柄——连续复制不同码时先 clearTimeout 旧定时器，
  // 避免旧定时器提前清空新复制码的"已复制"提示（状态复用错乱）
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const { toast } = useToast()
  // F8-03：删除账号成功后立即失效账号列表查询（否则 10s 轮询前行不消失）
  const queryClient = useQueryClient()
  // N3：删除账号确认态（账号名 + 确认中），用极简黑白 Dialog 二次确认替代 window.confirm
  const [pendingDelete, setPendingDelete] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  const copy = async (code: string) => {
    try {
      // F19-03：navigator.clipboard 非安全上下文会整体不可用——http://内网/
      // 明文部署、iframe 嵌入、权限被拒时抛错落入 catch 提示"未授权剪贴板"，管理员复制
      // 激活码整链失效。降级：手动构造 textarea 走已废弃的 document.execCommand("copy")
      // 兜底（旧兼容路径，同步执行），仍失败才提示（且把完整激活码展示给管理员抄录）。
      // F20-02：textarea 不设 readOnly 时部分浏览器会从可编辑区弹出软键盘或
      // 选择行为异常导致 execCommand 返回 false——补 readOnly 加固复制兜底稳定性。
      if (!navigator.clipboard || !navigator.clipboard.writeText) {
        throw new Error("clipboard API 不可用")
      }
      await navigator.clipboard.writeText(code)
      setCopied(code)
      if (copyTimer.current) clearTimeout(copyTimer.current)
      copyTimer.current = setTimeout(() => setCopied(""), 1500)
    } catch {
      // execCommand 兜底：document 活跃才可能成功；text area 仅内存驻留不入 DOM 树
      let ok = false
      try {
        const ta = document.createElement("textarea")
        ta.value = code
        ta.style.position = "fixed"
        ta.style.opacity = "0"
        // F20-02：readOnly 固化，防可编辑区干扰选中/复制
        ta.readOnly = true
        document.body.appendChild(ta)
        ta.select()
        ok = document.execCommand("copy")
        document.body.removeChild(ta)
      } catch {
        ok = false
      }
      if (ok) {
        setCopied(code)
        if (copyTimer.current) clearTimeout(copyTimer.current)
        copyTimer.current = setTimeout(() => setCopied(""), 1500)
        return
      }
      toast({
        title: "复制失败，请手动抄录",
        description: `浏览器未授权剪贴板权限（激活码：${code}）`,
        variant: "destructive",
      })
    }
  }

  // 组件卸载时清理挂起的复制反馈定时器（切 Tab/退出 Admin 后不再 setState）
  useEffect(
    () => () => {
      if (copyTimer.current) clearTimeout(copyTimer.current)
    },
    []
  )

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

        <Tabs value={activeTab} onValueChange={setActiveTab}>
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
            <CodesTab account={account} sessionToken={sessionToken} onCopy={copy} copied={copied} />
          </TabsContent>
          <TabsContent value="config">
            <ConfigTab account={account} sessionToken={sessionToken} />
          </TabsContent>
          <TabsContent value="stats">
            <StatsTab account={account} sessionToken={sessionToken} />
          </TabsContent>
          <TabsContent value="accounts">
            <AccountsTab
              account={account}
              sessionToken={sessionToken}
              onSelectAccount={onSelectAccount}
              onAskDelete={(acct) => setPendingDelete(acct)}
            />
          </TabsContent>
          <TabsContent value="logs">
            <LogsTab account={account} sessionToken={sessionToken} />
          </TabsContent>
        </Tabs>

        {/* N3：删除账号二次确认 Dialog（替代 window.confirm，符合黑白极简设计） */}
        {/* F7-03：补对话语义——role=dialog/aria-modal/aria-labelledby，读屏可识别 */}
        {/* F20-04：补 Esc 关闭——与 Login 激活弹窗对齐，键盘可达性闭环 */}
        {pendingDelete && (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-in fade-in duration-150"
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-acct-modal-title"
            onKeyDown={(e) => {
              if (e.key === "Escape" && !deleting) setPendingDelete(null)
            }}
          >
            <div className="relative w-full max-w-sm rounded-[var(--radius-lg)] border border-neutral-800 bg-[#09090b] p-5 shadow-2xl space-y-4">
              <div className="flex items-start gap-3">
                <div className="p-2 rounded-full bg-red-500/10 text-red-400 border border-red-500/20 shrink-0">
                  <AlertTriangle className="h-4 w-4" />
                </div>
                <div className="space-y-1">
                  <h3 id="delete-acct-modal-title" className="text-sm font-medium text-white tracking-wide">确认删除该账号？</h3>
                  <p className="text-xs text-neutral-400 leading-relaxed break-all">
                    账号：<span className="text-white font-mono">{pendingDelete}</span>
                    <br />
                    将清空其凭据、目标与成功记录（日志保留），操作不可撤销。
                  </p>
                </div>
              </div>
              <div className="flex items-center justify-end gap-2 pt-2 border-t border-neutral-900">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPendingDelete(null)}
                  disabled={deleting}
                  className="text-xs h-8"
                  autoFocus
                >
                  取消
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  disabled={deleting}
                  onClick={async () => {
                    // F30-01：在飞幂等守卫——双击删除按钮第二发 DELETE 报
                    // "账号不存在"假失败 toast（F19-02 同族）。
                    if (deleting) return
                    setDeleting(true)
                    try {
                      await api("/admin/accounts", {
                        method: "DELETE",
                        session: sessionToken,
                        body: JSON.stringify({ account: pendingDelete }),
                      })
                      // F8-03：删除成功后立即失效账号列表查询——
                      // 否则行要等 10s refetch 轮询才消失，用户刚删的账号仍显示在列表中
                      queryClient.invalidateQueries({ queryKey: ["admin-accounts"] })
                      // M28-01：删除成功必须把本地会话同步摘除（快照式三连，
                      // 与 logout/onUnauthorized 同款）——后端 6 表已清、前端 sessions 若残留
                      // 该账号条目，会继续占账号槽位 + 首次刷新复活。DeleteAccount 已由后端
                      // RevokeAccount 吊销服务端会话，这里绝不调用 logout()（那是用当前管理员
                      // 令牌登出自己，语义完全不对）。
                      if (pendingDelete) onDeleted?.(pendingDelete)
                      setPendingDelete(null)
                      toast({ title: "已删除", description: `账号 ${pendingDelete} 已移除`, variant: "success" })
                    } catch (e: any) {
                      toast({ title: "删除失败", description: e.message || "通信异常", variant: "destructive" })
                    } finally {
                      setDeleting(false)
                    }
                  }}
                  className="text-xs h-8 bg-red-600 hover:bg-red-500 text-white border-none"
                >
                  {deleting ? "删除中..." : "确认删除"}
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ---- 激活码管理 ----

function CodesTab({
  account,
  sessionToken,
  onCopy,
  copied,
}: {
  account: Account
  sessionToken: string
  onCopy: (c: string) => void
  copied: string
}) {
  const { toast } = useToast()
  const [count, setCount] = useState(5)
  const [uses, setUses] = useState(1)
  const [generating, setGenerating] = useState(false)
  const [generated, setGenerated] = useState<string[]>([])
  // 删除中的激活码集合：按码独立跟踪——单值布尔会被并发删除不同码互相覆盖标记
  // （码 A 删除在飞点码 B 覆盖标记、A finally 清 false 把 B 在飞态抹掉，B 再点发第二发
  // 后端报"不存在"假失败），与选课大厅手动操作 actionLoading 同款结构
  const [removing, setRemoving] = useState<ReadonlySet<string>>(new Set())

  // n11：queryKey 必须含 account——管理员会话令牌在切换目标账号后复用
  // 同一浏览器令牌，若缓存只按令牌分键，另一账号的轮询数据会覆盖本账号视图。
  // （会话级服务端数据本就按 sessionAccount 过滤，本地缓存键必须跟随同一维度。）
  const codesQuery = useQuery({
    queryKey: ["admin-codes", account, sessionToken],
    queryFn: () => api<ActivationCode[]>("/admin/codes", { session: sessionToken }),
    refetchInterval: 5000,
  })

  const generate = async () => {
    // F30-01：在飞幂等守卫——与 login submit F19-02 / activate F21-04 同款
    // 短路：按钮 disabled 依赖 React 渲染落地有延迟，连按两次会在 disabled 生效前发出两个
    // POST /admin/codes → 激活码重复生成 count 个（后端落库两次，前端 setGenerated 被第二发
    // 覆盖）。入口先查在飞标记即停。
    if (generating) return
    setGenerating(true)
    try {
      const codes = await api<string[]>("/admin/codes", {
        method: "POST",
        session: sessionToken,
        body: JSON.stringify({ count, uses }),
      })
      setGenerated(codes)
      codesQuery.refetch()
      toast({ title: "激活码已生成", description: `本次生成 ${codes.length} 个，每个可用 ${uses} 次`, variant: "success" })
    } catch (e: any) {
      toast({ title: "生成失败", description: e.message || "通信异常", variant: "destructive" })
    } finally {
      setGenerating(false)
    }
  }

  const remove = async (code: string) => {
    // 在飞幂等守卫：入口先查本码是否已在删除中（Set 按码独立跟踪，互不覆盖标记）；
    // 按钮 disabled 依赖渲染落地有延迟，连按两次会在 disabled 生效前发出两个
    // DELETE /admin/codes → 第二发后端报"不存在"假失败 toast。
    if (removing.has(code)) return
    setRemoving((prev) => new Set(prev).add(code))
    try {
      await api("/admin/codes", {
        method: "DELETE",
        session: sessionToken,
        body: JSON.stringify({ code }),
      })
      codesQuery.refetch()
    } catch (e: any) {
      toast({ title: "删除失败", description: e.message || "通信异常", variant: "destructive" })
    } finally {
      setRemoving((prev) => {
        const next = new Set(prev)
        next.delete(code)
        return next
      })
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
                  <button onClick={() => remove(c.code)} disabled={removing.has(c.code)} className="p-1 text-neutral-500 hover:text-white transition-colors disabled:opacity-40 disabled:pointer-events-none" title="删除">
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

function ConfigTab({ account, sessionToken }: { account: Account; sessionToken: string }) {
  const { toast } = useToast()
  const [baseUrl, setBaseUrl] = useState("")
  const [apiKey, setApiKey] = useState("")
  const [model, setModel] = useState("")
  const [engine, setEngine] = useState("vision")
  const [concurrency, setConcurrency] = useState(1)
  const [activationOn, setActivationOn] = useState(true)
  const [saving, setSaving] = useState(false)

  const configQuery = useQuery({
    queryKey: ["admin-config", account, sessionToken],
    queryFn: () => api<AdminConfig>("/admin/config", { session: sessionToken }),
  })

  // 后端配置加载后回填表单。首次加载回填全部字段；配置保存成功后（configQuery.refetch）
  // 更新 refetchKey → effect 重跑，把后端"实际生效值"回填（F7-02：后端可能静默忽略/
  // 规范化某些字段，若表单只回显一次，管理员看到的会是与生效配置分叉的陈旧值）。
  const [configEpoch, setConfigEpoch] = useState(0) // 配置重新加载代际：每次自增触发回填
  const loaded = configQuery.data
  useEffect(() => {
    if (loaded) {
      setBaseUrl(loaded.vision_base_url)
      setModel(loaded.vision_model)
      setEngine(loaded.captcha_engine || "vision")
      setConcurrency(loaded.captcha_concurrency || 1)
      setActivationOn(loaded.activation_enabled)
    }
  }, [loaded, configEpoch])

  const save = async () => {
    // F18-02：配置加载完成前绝不保存——表单未回填时保存会用初始空值
    // 整体覆盖生效配置（激活码机制误开、Vision 配置清空），按钮已 disabled 锁定，
    // 此处再兜一道（加载失败停留初始值的手快路径）。
    if (!loaded) return
    // 在飞幂等：双击保存时按钮 disabled 依赖渲染落地有延迟，入口先查在飞标记短路
    if (saving) return
    setSaving(true)
    try {
      const body: Record<string, unknown> = {
        activation_enabled: activationOn,
        vision_base_url: baseUrl.trim(),
        vision_model: model.trim(),
        captcha_engine: engine,
        captcha_concurrency: Math.max(1, concurrency || 1),
      }
      // 留空 = 不改动 key（脱敏回显无法完整回填）
      if (apiKey.trim()) body.vision_api_key = apiKey.trim()
      await api("/admin/config", { method: "PUT", session: sessionToken, body: JSON.stringify(body) })
      // R27-01：PUT 成功后必须先 refetch 拉取后端"实际生效值"再自增代际——
      // 旧实现只 setConfigEpoch，effect 重跑时用的 loaded 仍是挂载时陈旧快照（configQuery
      // 从未 refetch、缓存 data 未更新），表单被覆盖回旧值；管理员二次保存即把刚生效的
      // 配置静默回滚。refetch 失败则不自增代际——表单保留用户输入不被陈旧值覆盖，
      // 后端此时已生效（本次保存并未回滚），toast 照常确认成功。
      const refreshed = await configQuery.refetch()
      if (!refreshed.error && refreshed.data) {
        setConfigEpoch((e) => e + 1) // F7-02：用后端生效真值经 effect 回填到表单（与生效配置对齐）
      }
      toast({ title: "配置已保存", description: "已生效，无需重启服务", variant: "success" })
      // "PUT 完成 → 用户此刻点进密钥框打字"的亚秒窄窗；且保存后才清空仍保证
      // "留空 = 不改动 key"的回显语义不被上次保存的旧输入污染。
      setApiKey("")
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
          {/* n9：switch 语义化——button role=switch + aria-checked，键盘可聚焦可开关
              （此前是裸 button 无语义，读屏不识别开关状态） */}
          <button
            type="button"
            role="switch"
            aria-checked={activationOn}
            aria-label="激活码机制"
            onClick={() => setActivationOn(!activationOn)}
            className={`w-11 h-6 rounded-full border transition-colors relative focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60 ${
              activationOn ? "bg-white border-white" : "glass-input border-neutral-700"
            }`}
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

      {!loaded && (
        <p className="text-[11px] text-amber-400/90">
          配置加载中——表单尚未回填，保存按钮已锁定，避免用初始空值覆盖生效配置
        </p>
      )}
      <Button
        variant="primary"
        size="sm"
        onClick={save}
        disabled={saving || !loaded}
        className="h-10 px-5 text-xs"
      >
        {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin text-black" /> : null}
        <span>保存并生效</span>
      </Button>
    </div>
  )
}

// ---- 运行状态 ----

function StatsTab({ account, sessionToken }: { account: Account; sessionToken: string }) {
  const statsQuery = useQuery({
    queryKey: ["admin-stats", account, sessionToken],
    queryFn: () => api<AdminStats>("/admin/stats", { session: sessionToken }),
    refetchInterval: 5000,
  })
  const s = statsQuery.data

  const rows: { label: string; value: string }[] = s
    ? [
        // 识别开放时间：唯一事实源 = 平台 beginTimes 自动识别（不可配置）。
        // open_time_set=false = 未识别/识别过期 → 显示"未识别"（绝不把过期旧值当开放时间）。
        { label: "识别开放时间", value: s.open_time_set === false ? "未识别" : s.open_time },
        // N3：窗口关闭信号三态——window_closed 字段依赖后端 /api/admin/stats 补发
        // （统计口径从学生端 /state 对齐），补发前 undefined 走"待命中"，绝不假报关闭。
        {
          label: "窗口状态",
          value: s.window_closed ? "已关闭" : s.window_opened ? "已开放" : "待命中",
        },
        { label: "激活码机制", value: s.activation_on ? "开启" : "关闭" },
        { label: "账号数", value: String(s.account_count) },
        { label: "预选目标", value: String(s.targets_count) },
        { label: "已选成功", value: String(s.success_count) },
        { label: "日志条数", value: String(s.log_count) },
        { label: "识别引擎", value: s.captcha_engine === "ddddocr" ? "本地 ddddocr" : "硅基流动 Vision" },
        { label: "识别并发", value: String(s.captcha_concurrency ?? 1) },
        { label: "识别模型", value: s.vision_model || "（未配置）" },
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
            {/* N5：教务令牌有效性可视化——管理员一眼看到各账号 token 是否失效/恢复中 */}
            <div className="py-2.5 flex items-center justify-between text-xs">
              <span className="text-neutral-400">教务令牌</span>
              <span className="flex items-center gap-1.5">
                {Object.values(s.token_valid ?? {}).some((v) => v === false) ? (
                  <>
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-white/40 animate-pulse" />
                    <span className="text-white/60 font-mono">部分失效 · 自动恢复中</span>
                  </>
                ) : (
                  <>
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
                    <span className="text-white font-mono">全部有效</span>
                  </>
                )}
              </span>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ---- 账号管理 ----

function AccountsTab({
  account,
  sessionToken,
  onSelectAccount,
  onAskDelete,
}: {
  account: Account
  sessionToken: string
  onSelectAccount?: (acct: string) => void
  onAskDelete: (acct: string) => void
}) {
  const accountsQuery = useQuery({
    queryKey: ["admin-accounts", account, sessionToken],
    queryFn: () => api<AdminAccount[]>("/admin/accounts", { session: sessionToken }),
    refetchInterval: 10000,
  })

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
                    <div className="flex items-center justify-end gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => onSelectAccount?.(a.account)}
                        className="text-xs text-white border-neutral-700 hover:border-white flex items-center gap-1.5"
                      >
                        <BookMarked className="h-3.5 w-3.5" />
                        <span>选课大厅</span>
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => onAskDelete(a.account)}
                        className="text-xs text-neutral-400 hover:text-white hover:border-white"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        删除
                      </Button>
                    </div>
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

function LogsTab({ account, sessionToken }: { account: Account; sessionToken: string }) {
  const logsQuery = useQuery({
    queryKey: ["admin-logs", account, sessionToken],
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
          <div className="py-8 text-center text-neutral-600 font-mono">暂无日志</div>
        )}
      </div>
    </Card>
  )
}

import { useMemo, useState, useEffect, useRef } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { api, selectElective, exitElective } from "../api/client"
import type { Account, ClassItem, ElectivesData, Target, SchedulerState, Publish } from "../types"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { Progress } from "../components/ui/Progress"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "../components/ui/Tabs"
import { useToast } from "../components/ui/Toast"
import { useTickingCountdown } from "../lib/useTickingCountdown"
import { selectedHasStalePublish, cleanStaleSelected } from "../lib/targetGuard"
import {
  ArrowLeft,
  ArrowDownWideNarrow,
  BookMarked,
  Check,
  Clock,
  Filter,
  LogOut,
  MapPin,
  Search,
  User,
  Users,
  AlertTriangle,
} from "lucide-react"

interface Props {
  account: Account
  sessionToken: string
  onDone: () => void
}

function fillRate(c: ClassItem): number {
  if (!c.max_count) return 0
  return Math.min(100, Math.round((c.selected_count / c.max_count) * 100))
}

// 优先级序号转展示名：0=首选，1=备选 1，2=备选 2（首位不再是"备选 1"）
function priorityName(p: number): string {
  return p === 0 ? "首选" : `备选 ${p}`
}

export default function Select({ account, sessionToken, onDone }: Props) {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  // M29-01：在飞操作从单值改 Set<number> 按课程 id 独立跟踪——
  // 单值 actionLoading 被并发不同课程操作互相覆盖（A 在飞时点 B 会覆盖 A 的标记，
  // A 的 finally 清 null 又把 B 的在飞态抹掉，用户再点 B 发第三发请求被后端
  // TryAcquireSubmit 拒绝 → 假失败 toast 在"不同课程"维度复发）。
  const [actionLoading, setActionLoading] = useState<ReadonlySet<number>>(new Set())
  const [exitModalClass, setExitModalClass] = useState<ClassItem | null>(null)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["electives", account, sessionToken],
    queryFn: () => api<ElectivesData>("/electives?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: (query) => {
      // 轮询判定来源：in_date_range 与 window_opened 双信号合并（MAJOR-G）。
      // 窗口即将开启的瞬间平台会短暂返回空 publishes，此时仅凭 in_date_range 会把
      // 10s 慢轮询带到黄金期——必须并入调度器侧 window_opened 信号，一开窗立即升频 2s。
      // F5-05：窗口已关闭（window_closed）并入降频——关闭后课程列表已被平台
      // 清空，继续 10s 高频打 findElectivesData 纯浪费；与 /state 同信号降 30s，全站统一。
      // F9-05：澄清：window_closed 读组件闭包 stateData（/state 查询数据）——
      // electives 自身响应（ElectivesData）无 window_closed 字段，且 /state 每 2s 刷新
      // 触发组件重渲染，react-query 用最新闭包重调度轮询间隔，闭包永不陈旧。
      // 注意：此处绝不直接读组件顶部的 stateData（声明在下方）——refetchInterval 回调
      // 在 useQuery 创建实例时即被同步调用，此刻 stateData 的 const 声明尚未执行，
      // 直读会命中 JS 暂存死区（TDZ）抛 ReferenceError，整个组件渲染中断黑屏。
      // 改从 react-query 缓存按查询 key 读取 /state 最新值，与 stateData 同源且零时序依赖。
      const st = queryClient.getQueryData<SchedulerState>(["state", account, sessionToken])
      const pubs = query.state.data?.publishes ?? []
      const inRange = pubs.some((p) => p.in_date_range)
      if (st?.window_closed) return 30000
      return inRange || st?.window_opened ? 2000 : 10000
    },
  })

  // 手动报名指定课程
  const handleSelectClass = async (c: ClassItem) => {
    // F26-03：在飞幂等守卫——与 login submit 的 F19-02 / 激活的 F21-04
    // 同款短路：disabled 渲染落地前双击/连点会发出两个并发报名，后端 TryAcquireSubmit
    // 拒绝第二个（"该课程正在提交中"），但第一个已 MarkDone 成功、第二个的 finally 仍
    // invalidateQueries 造成假失败 toast；退选同源。入口先查在飞标记即停。
    // M29-01：守卫与置位改用 Set 按课程独立跟踪，只拦"本课程在飞"。
    if (actionLoading.has(c.id)) return
    setActionLoading((prev) => new Set(prev).add(c.id))
    try {
      const res = await selectElective(c.id, sessionToken, account, c.course_name)
      toast({ title: "报名成功", description: res.msg || "已成功选报该课程", variant: "success" })
    } catch (err: any) {
      toast({ title: "报名失败", description: err.message || "请求被拒绝", variant: "destructive" })
    } finally {
      // N2：无论成功失败都强制失效 electives/state 缓存——
      // 报名失败（满员/窗口关闭）后名额与按钮状态同样已变化，必须立即刷新，
      // 否则前端显示"还可报名"实则已满，用户看到的是过期数据。
      queryClient.invalidateQueries({ queryKey: ["electives"] })
      queryClient.invalidateQueries({ queryKey: ["state"] })
      // M29-01：函数式清除只删自己的 id——绝不抹掉其他仍在飞的课程标记。
      setActionLoading((prev) => {
        const n = new Set(prev)
        n.delete(c.id)
        return n
      })
    }
  }

  // 手动退选指定课程二次确认提交
  const handleConfirmExit = async (c: ClassItem) => {
    // F26-03：与 handleSelectClass 同款在飞幂等守卫（双击退选第二个请求
    // 会被后端 TryAcquireSubmit 拒、假失败 toast）。
    // M29-01：与报名同款 Set 在飞跟踪——退选/报名并发互不覆盖。
    if (actionLoading.has(c.id)) return
    setActionLoading((prev) => new Set(prev).add(c.id))
    try {
      const res = await exitElective(c.id, sessionToken, account)
      toast({ title: "退选成功", description: res.msg || "已成功退选该课程", variant: "success" })
      setExitModalClass(null)
    } catch (err: any) {
      toast({ title: "退选失败", description: err.message || "请求被拒绝", variant: "destructive" })
    } finally {
      queryClient.invalidateQueries({ queryKey: ["electives"] })
      queryClient.invalidateQueries({ queryKey: ["state"] })
      // M29-01：与报名同款函数式清除，只删自己的退选在飞标记。
      setActionLoading((prev) => {
        const n = new Set(prev)
        n.delete(c.id)
        return n
      })
    }
  }

  // 查询当前调度器已保存的目标课程并自动回显（会话绑定当前账号）。
  // M-8：加 2s 自轮询——electives 的升频判定依赖 window_opened 信号，
  // 若此查询被动等 electives invalidate 才刷新，开窗瞬间（publishes 短暂为空）会把
  // 10s 慢轮询带进黄金期；独立轮询让 window_opened 一开窗立即升频 2s，两信号同源。
  // n12：窗口已关闭后降回 30s——与 Dashboard 同一信号同一次序，
  // 避免窗口关闭后仍 2s 高频打 /state 刷屏日志。
  const { data: stateData } = useQuery({
    queryKey: ["state", account, sessionToken],
    queryFn: () => api<SchedulerState>("/state?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: (query) => {
      // F40-M3：失败态/无数据时统一降频 30s——react-query 失败后 data 为最后一次
      // 成功值或 undefined，原回调查询失败时恒取 2000ms，网络挂断/后端重启期间
      // /state + /electives 双查询叠加固定 2s 轰炸日志与代理层（与"失败分级退避"防
      // 轰炸理念相悖）。error 或 status==="error" 即降频，成功态按 window_closed 升/降频。
      if (query.state.error || query.state.status === "error") return 30000
      return query.state.data?.window_closed ? 30000 : 2000
    },
  })

  const [selected, setSelected] = useState<Record<number, ClassItem[]>>({})
  // M30-03：回显一次性标记——回显 effect 只合并一次，绝不重放。
  // rev>0 守卫保护的"用户清空目标后轮询旧 courses 再次回填撤销清空"语义
  // 在这里由"只合并一次"延续：用户改动后的轮询不再重放回显（见 166 行 effect）。
  const echoedRef = useRef(false)
  // 回显完成状态（state 而非 ref）：防抖 effect 依赖必须能感知"回显流程完成"以驱动
  // 重跑——ref 变化不触发 effect。仅由回显 effect 置位一次，作为守卫拦下改动的自愈
  // 信号（见防抖回调内的回显未完成守卫与 echo effect 的空 courses 分支）。
  const [echoDone, setEchoDone] = useState(false)
  // 用户真实改动计数：驱动自动保存的 400ms 防抖；回显数据不经过它，故不会触发无意义保存。
  // 注意：F7 修复后它只归 pick()/清空操作自增——轮询拉回的 publishes 变化绝不触发保存。
  const [rev, setRev] = useState(0)
  // 镜像 ref：handleBack/flushTargets 是渲染闭包捕获的 async 函数，等待循环期间
  // 用户新改动触发重渲染不会更新闭包里的 selected/rev 快照——flush 在消费时刻
  // 必须读 ref 拿最新状态，否则旧快照会把等待期间的新改动覆盖删除（32-01）。
  const selectedRef = useRef(selected)
  selectedRef.current = selected
  const revRef = useRef(rev)
  revRef.current = rev
  // /state 到达状态镜像：防抖 effect 依赖不含 stateData（轮询刷新不得重置 400ms 窗口），
  // 回调闭包捕获的 stateData 恒为 effect 创建时的旧值——回显未完成守卫必须读 ref
  // 拿"首帧是否已到达"的最新判断（首帧未到 = 后端旧目标尚未经回显合并进 selected）。
  const stateDataRef = useRef(stateData)
  stateDataRef.current = stateData
  const [search, setSearch] = useState("")
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [sortTightest, setSortTightest] = useState(false)
  // F18-03：激活 Tab 受控兜底——发布集合整体重建（开窗瞬间平台清空又
  // 恢复、publish_id 全变）时，非受控 defaultValue 只首次生效、激活 Tab 对应的 Trigger
  // 从列表消失后 Radix Tabs 无 fallback，主区空白直到用户手点。受控 value 跟随
  // tabs[0]，发布重建即回落首个 Tab，绝不悬空。
  const [activeTab, setActiveTab] = useState<string | null>(null)
  // F36-01：跨账号组件实例复用防护——App 两处挂载点已加 key={account}，账号切换即整体
  // 重建实例（selected/echoedRef/rev 全复位），此处守卫只兜底"未来改为不重置挂载"的
  // 意外回归：account 变化时同步复位回显/编辑态，绝不让旧账号残留目标污染新账号
  // （401 被动吊销自动切剩余账号时，旧账号 selected 会被防抖 PUT 整包覆盖掉新账号目标）。
  // 声明于 echoedRef/rev/setRev/setEchoDone 之后（本文件顶部状态区），TDZ 不触发。
  const [accountKey, setAccountKey] = useState(account)
  if (accountKey !== account) {
    setAccountKey(account)
    setSelected({})
    setRev(0)
    echoedRef.current = false
    setEchoDone(false)
  }

  // 本地每秒刷新倒计时：F8-04/F9-07 收敛到 lib/useTickingCountdown 自 tick 组件，
  // 整页只重渲染倒计时一处，Tab 徽章"已锁定"计数随 selected 变化即时更新，无需每秒重算。

  // 进入页面时自动回显已保存的目标课程（含多备选优先级）。
  // M-9：函数体内统一用 prev 构造初始值，杜绝 `const initial` 遮蔽
  // 外部 `selected` 导致数据重取后回显永久失效的问题。
  // 用户已编辑过目标（rev>0）时跳过回显——用户"清空全部目标"后 2s 轮询
  // 返回的旧 courses 若再次回填，会把清空静默撤销并重新保存旧目标（回显与防抖保存竞态）。
  // F19-01：回显的 courses 可能携带"不属于当前发布集合"的 publish_id（旧学期
  // 残留/发布集合整体重建后后端 /state courses 仍按旧 publish_id 下发）——此前照单全收
  // 构建的 initial 也带幽灵 publish_id；此时 flushTargets/防抖保存的 targetsUseCurrentPublishes
  // 校验必失败，一路置脏跳过（安全方向：绝不假清空），但也永远不落库——目标被静默"锁死"
  // 在读不出的旧条目上，用户改不了也存不上。修复：回显即过滤，只用当前 publishes 集合内的
  // publish_id 构建 initial（与消费时刻校验同一判据），幽灵条目根本进不了 selected。
  // 回显合并：进页后课程列表（/electives 内存快照）先渲染，/state 首次加载慢于 electives
  // （或首帧失败 retry 拉长到秒级）时，用户在 stateData 到达前先点选课程 → 若按"rev>0 即
  // 跳过回显"处理，后端已保存的旧目标永远不进 selected，防抖 PUT 只含用户新点的课程 →
  // 旧目标被静默覆盖删除（本意"添加一门"变"替换全部"）。合并按 publish_id 区分：用户已触碰
  // （key 存在）的发布保留用户现状（含用户主动清空过的空数组，清空语义绝不复活），未触碰的
  // 发布把后端旧目标补进；echoedRef 保证只合并一次（防轮询旧 courses 再次回填撤销清空）。
  // 全清空（selectedCount===0 且 rev>0，用户明确清掉全部目标）绝不合并——否则"选 A,B→保存
  // 成功→全清空"后首次非空 courses 响应会把 [A,B] 合并回来并重新写回后端，清空被静默撤销。
  useEffect(() => {
    if (echoedRef.current) return
    // 首帧未到（stateData===undefined）：绝不提前置位回显完成——否则 /state 晚于
    // 用户首次点击到达时，防抖/flush 用"只含用户新改动"的 selected 整包覆盖删除后端
    // 旧目标（"添加一门"变"替换全部"），且 M30-03/31-01/35-01 真合并、33-01 等待、
    // 34-01 守卫全部失效。继续等待 /state；持续失败由轮询自愈，改动滞留不覆盖（安全方向）。
    if (stateData === undefined) return
    const courses = stateData.courses ?? []
    if (courses.length === 0) {
      // /state 首帧到达且确证后端无旧目标（courses 空）：echoed 完成——否则全程无旧
      // 目标的账号用户改动会被防抖回显守卫永久拦下（置脏无自愈信号）。清空语义/全
      // 清空守卫不受影响（echoDone 只做放行信号，不写 selected）。
      echoedRef.current = true
      setEchoDone(true)
      return
    }
    // effect 声明于 `const publishes` 之前（TDZ），必须用已声明的 data 自行推导，
    // 与回显合并同款构建中断陷阱——绝不能反向引用 effect 之后声明的 publishes。
    const pubs = data?.publishes ?? []
    if (pubs.length === 0) return
    const currentIds = new Set(pubs.map((p) => p.publish_id))
    const ordered = [...courses]
      .sort((a, b) => a.priority - b.priority)
      .filter((c) => currentIds.has(c.publish_id)) // 幽灵 publish_id 不进 selected
    setSelected((prev) => {
      const anyHas = Object.values(prev).some((arr) => arr.length > 0)
      const hasTouched = Object.keys(prev).length > 0
      // 用户已明确全清空（rev>0 且无任何条目）→ 绝不合并回显（清空语义不可侵犯）
      if (rev > 0 && !anyHas) return prev
      const next = { ...prev }
      let merged = false
      for (const c of ordered) {
        const list = next[c.publish_id]
        if (list) {
          // 已触碰发布：空数组 = 用户显式清空该发布（清空语义绝不复活）；
          // 非空数组 = 用户添加了课程——把后端旧目标中用户未勾选的补进（同发布
          // "添加一门"语义：慢首帧下旧目标尚未回显，不补就会被防抖 PUT 整包覆盖删掉）。
          // 补进按 priority 序排在用户勾选之后，用户可随后自行调整首选顺序。
          if (list.length > 0 && !list.some((item) => item.id === c.class_id)) {
            next[c.publish_id] = [
              ...list,
              { id: c.class_id, publish_id: c.publish_id, course_name: c.course_name } as ClassItem,
            ]
            merged = true
          }
          continue
        }
        ;(next[c.publish_id] ??= []).push({ id: c.class_id, publish_id: c.publish_id, course_name: c.course_name } as ClassItem)
        merged = true
      }
      // 无任何新发布可补（用户已触碰全部有目标的发布）→ 保持现状
      if (!hasTouched && !merged) return prev
      return next
    })
    // F40-M1：发布集合整体重建（开窗瞬间平台清空又恢复、publish_id 全变）后，
    // selected 仍残留旧 publish_id 的非空 key——对应 Tab 已消失、用户无法通过界面
    // 清除，守卫命中的"置脏跳过"会把保存链永久静默拦截（黄金期改目标永不落库）。
    // F41-M1：随重建清理已下沉为独立 effect（见 publishes 声明之后）——原本挂在
    // 本 effect 内却被首行 echoedRef 短路、只覆盖首次回显，已回显账号的发布重建
    // 后 stale 永不清理。此处只做回显数据过滤（currentIds 与消费时刻守卫同判据），
    // 清理职责移交给独立 effect。
    if (selectedHasStalePublish(selected, pubs)) {
      setSelected((prev) => cleanStaleSelected(prev, currentIds))
      toast({
        title: "发布已更新",
        description: "旧批次目标已失效并自动清理，新批次目标将重新保存",
        variant: "warning",
      })
    }
    echoedRef.current = true
    setEchoDone(true)
  }, [stateData, data, rev, selected, toast])

  const publishes = data?.publishes ?? []
  // F41-M1：发布集合重建清理独立 effect——F40-M1 原挂在回显 effect（首行
  // `if (echoedRef.current) return`）内，已完成回显的账号 echoedRef 恒 true，回显
  // effect 不再执行，之后发布集合整体重建（开窗瞬间平台清空又恢复、publish_id 全变）
  // 残留旧 publish_id 的 selected 永不被清理；挡在它前面的 stale 守卫把保存链静默
  // 锁死至整页刷新（黄金期最不该打断用户的操作）。
  // 本 effect 只依赖 [publishes, stateData, echoedRef, toast]——不依赖 echoedRef 的
  // 反向逻辑，而是正向条件"已回显过才清理"：发布重建瞬间本 effect 随 publishes
  // 变化重跑，命中 stale 即 setSelected 清理 + toast 提示；selected 变化触发防抖
  // effect（依赖含 selected）重跑，自动落库当前目标（自愈链与 F40-M1 同款）。
  // 未回显（echoedRef=false）时不清理——回显合并与 currentIds 同判据过滤幽灵条目，
  // 此刻抢先清理可能干扰重建前旧目标的合并/回显时序，绝无必要。
  // 只删"非空且不在当前发布集合"的 key（空数组键 = 用户主动清空，保留语义）。
  // 声明在 `const publishes` 之后合理使用已声明常量（F18-01 同款 TDZ 防护）。
  useEffect(() => {
    if (!echoedRef.current || publishes.length === 0) return
    const currentIds = new Set(publishes.map((p) => p.publish_id))
    if (!selectedHasStalePublish(selected, publishes)) return
    setSelected((prev) => cleanStaleSelected(prev, currentIds))
    toast({
      title: "发布已更新",
      description: "旧批次目标已失效并自动清理，新批次目标将重新保存",
      variant: "warning",
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [publishes, echoedRef, toast])

  const tabs = useMemo(
    () =>
      publishes.map((p) => ({
        ...p,
        label: p.publish_name || `发布 #${p.publish_id}`,
        open: p.in_date_range,
        tip: `可选 ${p.can_select} 门 · 已选 ${p.has_selected} 门 · 共 ${p.total_count} 门班次`,
      })),
    [publishes]
  )

  // 备选目标挑选：同发布下按点击顺序排优先级，取消后后续自动升级
  // F10-02：toast 移出 setSelected 的 updater——React 明令 updater 必须是
  // 纯函数，StrictMode 会对 updater 双调、并发渲染下也可能丢弃并重放；updater 内调
  // toast()（内部即另一组件的 setState）会让「已设为首选/已取消目标」重复弹出。
  // 事件处理器内 selected 恒为最近已提交渲染值（两次独立点击之间有渲染提交），
  // 先算 next 快照再 setSelected(next) 与函数式更新等价，且无副作用混入。
  const pick = (publishId: number, classItem: ClassItem) => {
    const arr = [...(selected[publishId] ?? [])]
    const idx = arr.findIndex((c) => c.id === classItem.id)
    if (idx >= 0) {
      arr.splice(idx, 1)
      setSelected({ ...selected, [publishId]: arr })
      toast({
        title: "已取消目标",
        description:
          arr.length > 0
            ? `已移出【${classItem.course_name}】，后续备选自动升级（当前首选：${arr[0].course_name}）`
            : `已移出【${classItem.course_name}】，该发布已无预选目标`,
        variant: "default",
      })
    } else {
      setSelected({ ...selected, [publishId]: [...arr, classItem] })
      toast({
        title: arr.length === 0 ? "已设为首选" : "已设为备选目标",
        description:
          arr.length === 0
            ? `【${classItem.course_name}】为首选，可继续添加同发布备选`
            : `已选中【${classItem.course_name}】为备选 ${arr.length}，可继续添加同发布备选`,
        variant: "default",
      })
    }
    setRev((r) => r + 1) // 标记选课改动，触发自动保存防抖
  }

  // 自动保存：选课一变（仅用户点击），400ms 防抖后整包 PUT 到后端；成功静默，失败仅提示
  // MAJOR-H：保存串行化——飞行中的 PUT 完成后立即补发一次最新快照，绝不出现
  // "旧 PUT 后到覆盖新数据"的乱序丢失；内存 target 与后端最终一致。
  // C-1：body 必须包成后端 TargetsRequest 期望的 {"targets":[...]} 对象——
  // 此前发裸数组 100% 解码失败（后端 json 解码进 struct 直接报错），目标永远存不进库。
    const lastJson = useRef("")
  const targetRef = useRef<Target[]>([])
  const savingRef = useRef(false)
  const dirtyRef = useRef(false)
  // F13-C2：已卸载标记——组件卸载后（返回控制台）绝不再发起新的网络请求
  // 或重发退避。此前卸载 cleanup 只清"当时挂着"的退避 timer，flush 补发失败后再
  // scheduleRetry 挂的新 timer 无人清理 → 组件卸载后 2/4/8/16/16s 最多 5 次孤儿请求，
  // 每次失败都全局 toast 轰炸已回到 Dashboard 的用户。卸载后重试也毫无意义（目标
  // 后端已有、改动已尽力）——直接停手。
  const unmountedRef = useRef(false)
  // n14：失败重发状态——attempt 累计连续失败次数、timer 为退避重发定时器。
  // 成功或用户产生新改动都会清零；连续失败 5 次停手，等下一次改动重新驱动。
  const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })
  // 卸载清理：中断仍在排队的退避重发定时器，防止 onDone 返回后副作用残留
  // F20-01：挂载时复位 unmountedRef——StrictMode 开发态会对组件执行
  // mount→unmount→remount 两遍，原实现只有置 true 的 cleanup、二次挂载时已卸载标记
  // 恒真，saveNow/scheduleRetry 全部短路，目标改动静默丢失且无任何报错。重挂载后
  // 复位到 false，本轮会话保存链路恢复正常；真正卸载时 cleanup 依旧置位停手（F13-C2
  // 卸载后防孤儿重试契约不变）。
  useEffect(
    () => {
      unmountedRef.current = false
      return () => {
        unmountedRef.current = true
        if (retryState.current.timer) clearTimeout(retryState.current.timer)
      }
    },
    []
  )
  const resetRetry = () => {
    if (retryState.current.timer) clearTimeout(retryState.current.timer)
    retryState.current.timer = null
    retryState.current.attempt = 0
  }
  const scheduleRetry = () => {
    const attempt = retryState.current.attempt
    if (attempt >= 5) return // 连续失败 5 次后停止自动重发（等用户改动触发新一轮）
    const delay = Math.min(2 ** attempt, 16) * 2000 // 指数退避：2s / 4s / 8s / 16s / 16s
    retryState.current.attempt = attempt + 1
    retryState.current.timer = setTimeout(() => {
      retryState.current.timer = null
      void saveNow()
    }, delay)
  }
  const saveNow = async () => {
    if (unmountedRef.current) return // 已卸载（返回控制台）：不再发起/继续重试
    savingRef.current = true
    try {
      const targets = targetRef.current
      const json = JSON.stringify(targets)
      if (json === lastJson.current) return // 回显等非用户改动：跳过重复保存
      await api("/targets?account=" + encodeURIComponent(account), {
        method: "PUT",
        body: JSON.stringify({ targets }),
        session: sessionToken,
      })
      if (unmountedRef.current) return // 卸载后成功也不落 lastJson（避免干扰后续）
      lastJson.current = json
      resetRetry() // 保存成功：清掉退避重发状态
    } catch (e: any) {
      // F15-02：卸载后失败也绝不 toast——与 F13-C2"卸载后不轰炸"意图对齐
      if (unmountedRef.current) return
      toast({
        title: "目标保存失败",
        description: e.message || "通信异常，请重试",
        variant: "destructive",
      })
      //n14：失败保留 dirty（内存目标仍未持久化），并安排带退避的重发——
      //网络抖动/瞬时故障下不再退化为"尽力而为"，直至成功或用户新改动接管。
      dirtyRef.current = true
      scheduleRetry()
    } finally {
      savingRef.current = false
      if (unmountedRef.current) return // 已卸载：不再补发
      // 保存期间用户又改了目标（且非失败重试态）：立即补发一次最新快照，
      // 避免旧 PUT 后到覆盖新数据。失败重发走上面的退避定时器，不在此紧循环。
      if (dirtyRef.current && retryState.current.attempt === 0) {
        dirtyRef.current = false
        void saveNow()
      }
    }
  }
  // 退出前立即保存挂起的目标改动：返回按钮的防抖窗口（<400ms）内最后一次点选
  // 或重发退避排队中的改动，不在此刻落库就永失（F12-M1）。复用 lastJson 去重 +
  // savingRef/dirtyRef 串行化，绝不与飞行中的 PUT 乱序覆盖。
  // F13-C1：无用户改动（rev===0）时绝不整包覆盖——回显数据本就是后端
  // 目标的镜像、无需回写；而进页数据未就绪时 selected/publishes 为空，此时 PUT
  // {"targets":[]} 会把后端已有目标整包抹除（窗口关闭后 publishes 恒空时必现）。
  // 清空全部目标仍是用户改动（rev>0），仍正确落库。
  // F15-01：rev>0 但 publishes 已空（窗口开启瞬间平台短暂清空 / 关闭后
  // 恒空）时也不能整包覆盖——targets 由 [publishes × selected] 联查构建，任一为空则
  // targets=[] 是一个"假清空"，会把已落库目标永久抹除。守卫"发布缺席 + 已有选中
  // 目标"=数据缺席绝非用户意图；只有 selected 全空（用户明确清空全部）才合法 PUT []。
  // F16-01：此守卫的渲染期常量判据会被防抖/flush 回调在 400ms 后读旧闭包
  // 值；F17-02 已把判据全部移入消费时刻（见 flushTargets 与防抖回调内的
  // "selectedCount>0 却构建出空集 = 假清空"守卫），渲染期常量已无引用，删除。
  const targetsUseCurrentPublishes = (targets: Target[], pubs: readonly Publish[]) => {
    const ids = new Set(pubs.map((p) => p.publish_id))
    // F17-01：空 targets 时 every 恒真——空集防御已由各消费点的
    // "联查产物为空 + 已有选中 = 假清空"守卫覆盖（防抖回调 + flushTargets 双闸）。
    return targets.every((t) => ids.has(t.publish_id))
  }
  const flushTargets = () => {
    // 消费时刻读 ref：handleBack 等待循环内用户新改动后，渲染闭包的 rev/selected
    // 是旧快照，必须取 ref 里的最新值（32-01）——否则等待窗口内新增的课程被忽略。
    const latestSelected = selectedRef.current
    const latestRev = revRef.current
    const latestSelectedCount = Object.values(latestSelected).reduce(
      (n, arr) => n + arr.length,
      0
    )
    if (latestRev === 0) return
    // F41-M2：回显未完成守卫——与防抖回调同款判据（见防抖 effect 内 618 行）：
    // /state 首帧未到达（stateData===undefined）或首帧携带旧目标（courses 非空）时，
    // 后端旧目标尚未经回显 effect 合并进 selected，此刻 flush 拿"只含用户新改动"的
    // selected 整包 PUT 会把后端旧目标覆盖删除（"加一门"变"替换全部"）。handleBack
    // 的 5s 等待只保证"等待期间合并完成"——/state 首帧持续失败超时后，守卫在这里
    // 兜住：置脏跳过、不 PUT，脏块保留（dirtyRef=true），下次进入/刷新/回显完成
    // 后再落库（安全方向：绝不静默丢改动）。消费时刻读 ref 判首帧，与防抖同源。
    if (
      !echoedRef.current &&
      (stateDataRef.current === undefined ||
        (stateDataRef.current.courses?.length ?? 0) > 0)
    ) {
      dirtyRef.current = true
      return
    }
    // F17-01：与防抖回调同款消费时刻守卫（同 F15-01 意图，判据从渲染期
    // publishesMissing 升级为最新 publishesRef）——"发布缺席 + 已有选中"= 数据缺席
    // 绝非用户清空意图，保留脏绝不 PUT [] 假清空；selectedCount 偏保守安全。
    if (publishesRef.current.length === 0 && latestSelectedCount > 0) {
      dirtyRef.current = true // 发布缺席：保留脏，绝不假清空覆盖；下次进入/恢复后再落库
      return
    }
    // C1：发布集合整体重建后 selected 仍残留旧 publish_id 的非空条目——build()
    // 只遍历当前发布集合会静默丢弃它们，产出"仅含新发布课程"的整包 PUT 覆盖删除
    // 后端已保存的旧目标（数据丢失）。前置守卫判有过期条目即置脏跳过；空数组键 =
    // 用户主动清空（清空语义绝不复活），不判过期。与回显 effect 的 currentIds 过滤同判据。
    // F40-M1：命中给明确提示——守卫本身正确（保数据 > 可保存），但旧残留 key 的
    // 对应 Tab 已消失、用户无法通过界面清除，若全程静默保存链就被锁死（黄金期改
    // 目标永不落库且无任何反馈）。发布重建路径已在回显 effect 随建随清（首选出路），
    // 此处 toast 兜底"清理未覆盖到的旧残留"，并把恢复路径指给用户（刷新后重新选择）。
    if (selectedHasStalePublish(latestSelected, publishesRef.current)) {
      dirtyRef.current = true // selected 残留旧发布：保留脏，绝不整包覆盖后端旧目标
      if (!unmountedRef.current) {
        toast({
          title: "发布已更新",
          description: "旧批次目标已失效，已停止保存。请刷新页面重新选择",
          variant: "warning",
        })
      }
      return
    }
    const targets: Target[] = []
    for (const p of publishesRef.current) {
      const list = latestSelected[p.publish_id] ?? []
      list.forEach((cls, i) => {
        targets.push({
          publish_id: p.publish_id,
          class_id: cls.id,
          course_name: cls.course_name,
          priority: i,
        })
      })
    }
    // F17-01：统一"用户有勾选但联查产物为空 = 假清空"守卫——目标集由
    // [publishes × selected] 联查构建，任一为空即 targets=[]。F15-01 判据是渲染期
    // publishesMissing（回调时读旧闭包）；F16-01 的 every 校验对空 targets 恒真。
    // 这里在消费时刻校验"selectedCount>0 却构建出空集"：数据缺席/错位绝非用户清空
    // 意图，保留脏跳过；selectedCount 只随用户改动所在渲染更新，只会偏保守绝不放过。
    if (targets.length === 0 && latestSelectedCount > 0) {
      dirtyRef.current = true // 联查为空：保留脏，绝不假清空覆盖；下次进入/恢复后再落库
      return
    }
    // F16-01：发布集合在"渲染→回调"窗口内重建（id 漂移）时，targets 的 publish_id 已
    // 不属于当前发布集 → 这份快照是错位假清空，绝不 PUT，置脏等下次正确联查再落库。
    if (!targetsUseCurrentPublishes(targets, publishesRef.current)) {
      dirtyRef.current = true
      return
    }
    targetRef.current = targets
    if (savingRef.current) {
      dirtyRef.current = true // 保存进行中：标记脏，让飞行中的 PUT 完成后补发本次快照
      return
    }
    void saveNow()
  }
  // F21-01：返回控制台前必须把"在飞 PUT 的补发窗口"关掉——直接 onDone
  // 会同步卸载：若点击返回时上一条目标保存仍在飞行（savingRef=true）而用户又改动过
  // 目标，flushTargets 只置脏就返回；飞行 PUT 完成后 finally 发现已卸载（F13-C2 契约）
  // 跳过补发，最后一批改动静默丢失。修复：先 flush，再等飞行中 PUT 结束（其 finally
  // 会在卸载前自动补发最新快照），直到保存链静止才真正卸载。守卫拦下的假清空脏块
  // （publishes 恒空）不在此列——那是 F15/F16/F17 链的刻意安全方向，等无可等，绝不
  // 强行假清空。api 20s 超时兜底，返回按钮绝不无限挂起。
  const handleBack = async () => {
    // 回显合并先行：/state 首帧晚于用户首次点击到达时（stateData 仍为 undefined），
    // 后端旧目标尚未经回显 effect 合并进 selected——此刻直接 flush 会用当前 selected
    // （只含用户新改动）整包 PUT 覆盖删掉后端旧目标（"添加一门"变"替换全部"）。
    // 只有回显已完成（echoedRef 置位）或确证后端无旧目标（/state 已到且 courses 为空）
    // 才可立即开始保存；等待期间回显 effect 把旧目标补进 selected，flush 自然全量提交。
    // 关键：等待只在"用户实际有改动"（revRef>0）时才需要——纯浏览（rev===0，SELECTED 0）
    // 时 flush 本就在 F13-C1 的 rev===0 处直接跳过、零覆盖风险，绝无理由等首帧。
    // 5s 兜底：/state 持续失败时合并永不发生，等无可等继续——flush 内假清空守卫仍拦截
    // 发布缺席的覆盖（安全方向）。注意 5s 等待只在"首帧未到"（stateData===undefined）
    // 或首帧确实携带旧目标（courses 非空）时才会发生——courses 为空（窗口已关/无目标）
    // 时 echoedRef 已在回显 effect 空分支置位、条件不成立，点击返回立即放行。
    if (
      revRef.current > 0 &&
      !echoedRef.current &&
      (stateData === undefined || (stateData.courses?.length ?? 0) > 0)
    ) {
      const deadline = Date.now() + 5000
      // 轮询间隔 50ms：回显合并是 React 状态更新+渲染（一帧约 16ms），50ms 足够感知
      // 完成且不抢调度；10ms 会让 5s 窗口内连开约 500 个定时器空转主线程。
      while (!echoedRef.current && Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, 50))
      }
      // 等合并 effect 的 setSelected 渲染提交落地，selectedRef 同步到含旧目标的合并结果
      await new Promise((r) => setTimeout(r, 0))
    }
    for (let i = 0; i < 3; i++) {
      // 33-01：首帧未到等满 5s 后，若回显合并仍未发生（/state 持续失败），继续 flush
      // 是唯一合法路径——flush 内的假清空守卫（发布缺席 + 已有选中）仍拦截覆盖；若
      // /state 已经成功但 courses 非空，回显 effect 必然已合并完成，5s 内 echoedRef 已
      // 置位，条件不成立。本循环 flush 消费最新 ref 快照。
      flushTargets()
      // 32-01：flush 已消费本轮最新 ref 快照，但 break 前必须等 React 下一帧落地——
      // 若等待窗口刚有用户改动（pick 的 setRev → effect 挂 400ms 防抖 timer，异步）
      // 此刻还没触发，onDone 同步卸载会清掉 timer，改动静默丢失。等一帧后复查
      // revRef：与本轮 flush 消费的一致才真正静止；又变了就多等一轮 flush 收敛。
      const flushedRev = revRef.current
      if (!dirtyRef.current && !savingRef.current) {
        await new Promise((r) => setTimeout(r, 0))
        if (revRef.current === flushedRev) break
        continue // 更晚的改动涌进来：多等一轮防抖/flush 收敛再卸载
      }
      // F26-01：保存链"待定工作"不只在飞 PUT——退避重试 timer 排队中
      // （scheduleRetry 已挂 2/4/8/16s）同样表示内存与后端分叉、改动未落库。此前只等
      // savingRef，退避 timer 在飞时被误判"已静止"→ 三轮后无条件 onDone 卸载、
      // cleanup clearTimeout 取消排队重试 → 最后一批改动静默丢失且无任何提示
      // （比 F21-01 的"飞行 PUT 补发"少覆盖了失败重试路径）。此处等待 timer 触发后
      // saveNow 的 finally 自接补发链收敛；持续失败则超时兜底，绝不无限挂起。
      const pendingSaving = () => savingRef.current || retryState.current.timer !== null
      if (pendingSaving()) {
        const deadline = Date.now() + 21000
        while (pendingSaving() && Date.now() < deadline) {
          await new Promise((r) => setTimeout(r, 30))
        }
        continue // 收敛（或超时）后下一轮再 flush，拿最新目标再真实发一次保存
      }
      // 脏块被守卫拦下（等无可等）：下轮再试即放行
    }
    onDone()
  }
  // F13-C1：退出前 flush 已由"无用户改动即跳过"收敛（见 flushTargets），
  // 防抖 effect 仍只由 rev 驱动（与 F7-01 同款守卫）——轮询/回显/窗口收缩绝不触发保存。
  // F30-01：附加 hasPublishes 布尔信号——开窗瞬间平台清空 publishes（F15/F16/
  // F17 假清空守卫拦下置脏）后发布恢复，只有 rev 驱动的话 effect 不重跑、无新 timer，
  // 置脏的改动永不落库（"等发布恢复再落库"的注释承诺从未实现）。hasPublishes 从 false→
  // true 时 effect 重跑 → 新 400ms timer → 消费时刻守卫通过 → 正常保存。publishes 非空期间
  // 轮询刷新布尔值不变、effect 不重跑，绝不把 400ms 防抖窗口无限重置。
  const hasPublishes = (data?.publishes?.length ?? 0) > 0
  const publishesRef = useRef<readonly Publish[]>(publishes)
  publishesRef.current = publishes
  useEffect(() => {
    if (rev === 0) return
    // n14：用户新改动接管——中断失败重发退避，下一轮保存由正常防抖路径驱动
    resetRetry()
    const build = (): Target[] => {
      const targets: Target[] = []
      for (const p of publishesRef.current) {
        const list = selected[p.publish_id] ?? []
        list.forEach((cls, i) => {
          targets.push({
            publish_id: p.publish_id,
            class_id: cls.id,
            course_name: cls.course_name,
            priority: i,
          })
        })
      }
      return targets
    }
    const timer = setTimeout(async () => {
      // 回显未完成守卫：/state 首帧尚未到达或首个回显携带旧目标（courses 非空）时，
      // 防抖回调不能拿"只含用户新改动"的 selected 整包 PUT 覆盖后端旧目标（"添加一门"
      // 变"替换全部"）。渲染期的 stateData 是 effect 创建时的旧闭包，必须读 ref 判
      // "首帧是否已到/是否存在旧目标"——首帧未到或有旧目标 → 置脏等回显合并（合并
      // 触发 selected 变化 → effect 重跑 → 新 timer 携带完整目标落库，自愈）；courses
      // 为空 = 确证后端无旧目标，直接放行。
      if (
        !echoedRef.current &&
        (stateDataRef.current === undefined ||
          (stateDataRef.current.courses?.length ?? 0) > 0)
      ) {
        dirtyRef.current = true
        return
      }
      // F15-01 + F17-01：防抖回调在 400ms 后执行，读到的是渲染期旧闭包
      // （publishesMissing 恒为本次渲染推算值）。若这期间发布集被清空（开窗瞬间平台
      // 清空 / 窗口关闭），旧守卫失效且 targetsUseCurrentPublishes 对空 targets 恒真，
      // build() 拿空 publishesRef 产出 [] 即"假清空"照常 PUT 抹掉后端目标。
      // 于是在消费时刻用最新 publishesRef 判"发布缺席 + 已有选中"——数据缺席绝非用户
      // 意图，跳过本次保存保留脏（等发布恢复/下次改动再落库）；selectedCount 只可能
      // 偏保守（用户已清空时为假阳守卫，安全方向），绝不会放过真实假清空。
      if (publishesRef.current.length === 0 && selectedCount > 0) {
        dirtyRef.current = true
        return
      }
      // C1：防抖消费时刻同款前置守卫——发布集合整体重建后 selected 残留旧 publish_id
      // 非空条目时，build() 只产出新发布课程，整包 PUT 覆盖删除后端已保存的旧目标。
      // 判定置于构建之前（残留旧发布时根本不该产出可 PUT 的目标）；空数组键 =
      // 用户主动清空该发布（清空语义绝不复活），不判过期。
      // F40-M1：命中给明确提示（防抖回调可能迟于卸载执行，卸载后绝不弹 toast 轰炸）。
      if (selectedHasStalePublish(selected, publishesRef.current)) {
        dirtyRef.current = true
        if (!unmountedRef.current) {
          toast({
            title: "发布已更新",
            description: "旧批次目标已失效，已停止保存。请刷新页面重新选择",
            variant: "warning",
          })
        }
        return
      }
      const next = build()
      // F17-01：防抖消费时刻同款"联查产物为空 = 假清空"守卫——F16-01 的
      // every 校验对空 targets 恒真，必须独立判"selectedCount>0 却产出空集"。仅在
      // 发布全缺席（构建来源为空的极限情况）时，selectedCount 可能滞后于本次清空
      // 为用户误伤守卫（仅多等一次防抖），安全方向；真实假清空绝不放过。
      if (next.length === 0 && selectedCount > 0) {
        dirtyRef.current = true
        return
      }
      // F16-01：防抖消费时刻同样过"发布 id 全数校验"——publishesMissing 是渲染期旧值，只在
      // load 时一次，防抖回调窗口内发布重建会让 next 携带漂移 id，错位假清空绝不 PUT。
      if (!targetsUseCurrentPublishes(next, publishesRef.current)) {
        dirtyRef.current = true
        return
      }
      targetRef.current = next
      if (savingRef.current) {
        dirtyRef.current = true // 保存进行中：标记脏，完成后补发
        return
      }
      void saveNow()
    }, 400)
    return () => clearTimeout(timer)
    // echoDone：回显完成驱动 effect 重跑——场景 B（后端确证无旧目标，courses 空）
    // 回显 effect 只置 echoedRef/echoDone、不改 selected，若无此依赖置脏的改动永不
    // 重试落库；非空合并场景由 selected 变化驱动（双路并保）。回显只完成一次，不会
    // 重置 400ms 防抖窗口。
  }, [rev, selected, sessionToken, toast, hasPublishes, echoDone])

  const selectedCount = Object.values(selected).reduce((n, arr) => n + arr.length, 0)

  // 选课开放时间倒计时（来自调度器状态——平台 beginTimes 自动识别唯一事实源，不可配置；
  // 识别不到或识别过期 = 未知，绝不显示编造时间）。
  const openTimeStr =
    stateData?.open_time_known && stateData.open_time
      ? stateData.open_time
      : null
  // F9-07：统一用 lib 共享 useTickingCountdown——与 Dashboard 同一实现、
  // 秒级自 tick 只重建倒计时一处，删除 Select 旧的 parseCountdown + 每秒 setTick 双份。
  const cd = useTickingCountdown(openTimeStr)

  return (
    <div className="min-h-screen text-white p-4 sm:p-6 lg:p-8 select-none pb-28 sm:pb-24">
      <div className="max-w-6xl mx-auto flex flex-col gap-6">
        {/* 顶部纯黑白极简顶栏 */}
        <header className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-neutral-900 pb-5">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <h1 className="text-lg sm:text-xl font-medium tracking-tight text-white">
                选修课程大厅
              </h1>
              <Badge variant="outline" className="text-[10px] font-mono uppercase">
                COURSES
              </Badge>
            </div>
            <p className="text-xs text-neutral-400">
              各批次预选课程配置与实时名额
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Badge variant="outline" className="text-xs py-1 px-3 font-mono">
              {account ? account : "默认"} · SELECTED {selectedCount}
              {publishes.length > 0 ? `/${publishes.length}` : ""}
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                // F12-M1：返回前先 flush 挂起的防抖/重发目标保存——
                // 直接 onDone 会卸载组件、400ms 防抖 timer 被清理，最后一次点选
                // 到返回间隔 <400ms 时整批目标永不 PUT。
                // F21-01：改为等待保存链静止的异步句柄——flush 后若
                // 在飞 PUT 完成会经 finally 自动补发（见 handleBack），全部落定才卸载。
                void handleBack()
              }}
              className="flex items-center gap-1.5 text-xs text-neutral-400 hover:text-white"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>返回控制台</span>
            </Button>
          </div>
        </header>

        {/* 选课开放倒计时（人性化：一眼看清距开放还有多久） */}
        <div className="glass rounded-[var(--radius-lg)] border border-neutral-800 px-4 py-2.5 flex items-center justify-between gap-2 text-xs">
          <div className="flex items-center gap-2 text-neutral-400 min-w-0">
            <Clock className="h-3.5 w-3.5 text-neutral-500 shrink-0" />
            {/* N8：已开放状态以调度器 window_opened 为准（服务端有 ~640ms 校准偏差，
                本地倒计时到点 ≠ 平台开窗）；window_opened 才显示"已开放"高亮。
                F15-03：此前 `|| cd.isExpired` 让窗口关闭后（open_time 为
                过去时刻 → isExpired 恒 true）横幅永远显示"已开放"，与同屏 F14-03 空态卡
                自相矛盾——isExpired 只是本地"倒计时走到 0"，不说明窗口开放或已关闭；
                改为 window_closed 优先显"已关闭"，否则按 window_opened 判定。 */}
            {!stateData ? (
              <span>正在同步选课开放时间...</span>
            ) : stateData.window_closed ? (
              <span className="text-neutral-400 font-medium flex items-center gap-1.5 whitespace-nowrap">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-neutral-500" />
                选课窗口已关闭
              </span>
            ) : stateData.window_opened ? (
              <span className="text-white font-medium flex items-center gap-1.5 whitespace-nowrap">
                <span className="inline-block w-1.5 h-1.5 rounded-full bg-white animate-pulse" />
                选课窗口已开放
              </span>
            ) : !openTimeStr ? (
              <span className="truncate">
                未识别到开放时间
                <span className="text-neutral-500">（平台尚未下发或识别已过期）</span>
              </span>
            ) : cd.isExpired ? (
              <span className="truncate">本地已到开窗点，等待平台窗口开放...</span>
            ) : (
              <span className="truncate">
                距开放还有{" "}
                <span className="text-white font-mono tabular-nums">
                  {cd.days} 天 {cd.hours} 时 {cd.minutes} 分 {cd.seconds} 秒
                </span>
              </span>
            )}
          </div>
          <span className="text-neutral-500 font-mono hidden sm:block shrink-0">
            {stateData?.open_time_known && openTimeStr
              ? new Date(openTimeStr).toLocaleString("zh-CN", { hour12: false })
              : "未知"}
          </span>
        </div>

        {/* 搜索与条件过滤栏 */}
        <div className="flex flex-col sm:flex-row items-center gap-3 p-3 rounded-[var(--radius-lg)] glass border border-neutral-800">
          <div className="relative flex-1 w-full">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
            <Input
              placeholder="搜索课程名称、教师或教室"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-10 text-xs sm:text-sm h-10 glass-input border-neutral-800 text-white placeholder:text-neutral-600 focus:border-white transition-colors"
            />
          </div>

          <div className="flex items-center gap-2 w-full sm:w-auto">
            <Button
              variant={sortTightest ? "primary" : "outline"}
              size="sm"
              onClick={() => setSortTightest(!sortTightest)}
              className="flex items-center gap-1.5 text-xs whitespace-nowrap h-10 px-4 flex-1 sm:flex-none"
            >
              <ArrowDownWideNarrow className="h-3.5 w-3.5" />
              <span>{sortTightest ? "剩余名额正序" : "按剩余排序"}</span>
            </Button>

            <Button
              variant={onlyAvailable ? "primary" : "outline"}
              size="sm"
              onClick={() => setOnlyAvailable(!onlyAvailable)}
              className="flex items-center gap-1.5 text-xs whitespace-nowrap h-10 px-4 flex-1 sm:flex-none"
            >
              <Filter className="h-3.5 w-3.5" />
              <span>{onlyAvailable ? "仅看有余量" : "显示全部"}</span>
            </Button>
          </div>
        </div>

        {/* 加载中与错误反馈 */}
        {isLoading && (
          <div className="rounded-[var(--radius-lg)] glass border border-neutral-800 p-16 text-center text-xs text-neutral-400 flex flex-col items-center justify-center gap-3">
            <span className="inline-block w-2 h-2 rounded-full bg-white animate-ping" />
            <span>正在同步最新课程列表与名额...</span>
          </div>
        )}

        {isError && (
          <div className="rounded-[var(--radius-sm)] glass border border-neutral-800 p-4 text-center text-xs text-neutral-300">
            拉取课程数据异常: {(error as Error).message}
          </div>
        )}

        {/* F14-03：窗口关闭/学期无发布空态——publishes 恒空时给出明确说明，
            不再是"无提示空白主体"（此前徽章还误显 n/0） */}
        {!isLoading && !isError && tabs.length === 0 && (
          <div className="rounded-[var(--radius-lg)] glass border border-neutral-800 p-16 text-center text-xs text-neutral-400 flex flex-col items-center justify-center gap-3">
            <span className="inline-block w-2 h-2 rounded-full bg-neutral-600" />
            <span>当前无可选课程批次（选课窗口未开放或已关闭）</span>
            <span className="text-neutral-600">窗口开放后课程列表将自动出现</span>
          </div>
        )}

        {/* 主选课 Tab 分段控制器
            F14-03：窗口关闭/学期无发布时 publishes 恒空 →
            tabs.length===0，此前整个主区不渲染且顶部徽章显示 n/0；补空态与徽章分母兜底 */}
        {!isLoading && !isError && tabs.length > 0 && (
          <Tabs
            value={activeTab && tabs.some((t) => String(t.publish_id) === activeTab) ? activeTab : String(tabs[0].publish_id)}
            onValueChange={setActiveTab}
            className="space-y-4"
          >
            <TabsList className="w-full sm:w-auto flex flex-wrap h-auto gap-1.5 p-1 glass border border-neutral-800 rounded-[var(--radius-lg)]">
              {tabs.map((t) => (
                <TabsTrigger
                  key={t.publish_id}
                  value={String(t.publish_id)}
                  className="flex items-center gap-2 py-2 px-4 text-xs sm:text-sm font-medium"
                >
                  <span className="font-semibold text-white">{t.label}</span>
                  <span
                    className={`inline-block w-2 h-2 rounded-full ${
                      t.open ? "bg-[var(--emerald)]" : "bg-[var(--fg-dim)]"
                    }`}
                  />
                  {(selected[t.publish_id] ?? []).length > 0 && (
                    <Badge variant="primary" className="text-[10px] px-1.5 py-0 ml-1">
                      已锁定 {(selected[t.publish_id] ?? []).length}
                    </Badge>
                  )}
                </TabsTrigger>
              ))}
            </TabsList>

            {tabs.map((t) => {
              let filteredClasses = t.classes.filter((c) => {
                const matchSearch =
                  !search ||
                  c.course_name.toLowerCase().includes(search.toLowerCase()) ||
                  (c.teacher_name_list &&
                    c.teacher_name_list.toLowerCase().includes(search.toLowerCase())) ||
                  (c.class_room_name &&
                    c.class_room_name.toLowerCase().includes(search.toLowerCase()))

                // 32-02：max_count=0（名额未公布，与 isFull 判据同源）不能被
                // "仅看有余量"当已满滤掉——0 表示未公布而非满员，课程照常显示。
                const matchAvailable = !onlyAvailable || c.max_count === 0 || c.selected_count < c.max_count
                return matchSearch && matchAvailable
              })

              // 剩余名额正序：名额越少越靠前，抢手课程一眼可见。
              // 排序键必须按"剩余名额 = max_count - selected_count"而非已报名数——
              // 两课上限不同时已报少≠剩余少（1/5 余4 与 10/100 余90，应前者靠前）。
              // max_count=0（名额未公布）映射为 0（最紧张），与筛选/徽章同源语义。
              if (sortTightest) {
                const remaining = (c: ClassItem) =>
                  c.max_count > 0 ? c.max_count - c.selected_count : 0
                filteredClasses = [...filteredClasses].sort(
                  (a, b) => remaining(a) - remaining(b)
                )
              }

              return (
                <TabsContent key={t.publish_id} value={String(t.publish_id)} className="space-y-4">
                  {/* 分类说明与概况 */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between text-xs text-neutral-400 p-3 rounded-[var(--radius-lg)] glass border border-neutral-800 gap-2 font-mono">
                    <span className="flex items-center gap-2">
                      <span className="inline-block w-1.5 h-1.5 rounded-full bg-white" />
                      <span>{t.tip}</span>
                    </span>
                    <span className="text-neutral-500">
                      SHOWING {filteredClasses.length}/{t.classes.length}
                    </span>
                  </div>

                  {/* 课程卡片网格阵列（桌面 4 列更密集） */}
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                    {filteredClasses.map((c) => {
                      const selArr = selected[t.publish_id] ?? []
                      const selIdx = selArr.findIndex((x) => x.id === c.id)
                      const isSelected = selIdx >= 0
                      const rate = fillRate(c)
                      // F30-01：max_count=0（未公布名额的新课程）时 `0>=0` 恒真
                      // 会误显"已满额"徽章并把进度条染红——与后端 IsClassFull 的
                      // `MaxCount>0 && SelectedCount>=MaxCount` 判据同源，0 表示名额未公布而非满员。
                      const isFull = c.max_count > 0 && c.selected_count >= c.max_count
                      // 32-02：max_count=0 时 remaining 恒 0 会误显"余 0 席"琥珀警示——
                      // 名额未公布（0）不是快满，徽章改显"名额未公布"、进度色回 emerald。
                      const remaining = c.max_count > 0 ? Math.max(0, c.max_count - c.selected_count) : 0
                      const unannounced = c.max_count <= 0

                      // 进度条与徽章色彩分配
                      let progressColor: "emerald" | "amber" | "rose" | "cyan" = "emerald"
                      if (isFull) progressColor = "rose"
                      else if (!unannounced && remaining <= 5) progressColor = "amber"
                      else if (isSelected) progressColor = "cyan"

                      return (
                        <Card
                          key={c.id}
                          className={`relative rounded-[var(--radius-lg)] border transition-all duration-200 flex flex-col justify-between shadow-none ${
                            isSelected
                              ? "border-white bg-black/15"
                              : "bg-black/10 border-neutral-800/70 hover:border-neutral-600"
                          }`}
                        >
                          <CardContent className="p-3.5 flex flex-col gap-2.5">
                            {/* 顶部标签行 */}
                            <div className="flex items-center justify-between">
                              <span className="text-xs text-neutral-500 font-mono">
                                ID: {c.id}
                              </span>
                              {isSelected ? (
                                <Badge variant="primary" className="text-[11px] font-medium">
                                  {priorityName(selIdx)}
                                </Badge>
                              ) : isFull ? (
                                <Badge variant="outline" className="text-[11px] text-neutral-500 border-neutral-800">
                                  已满额
                                </Badge>
                              ) : unannounced ? (
                                <Badge variant="outline" className="text-[11px] text-neutral-400 border-neutral-800">
                                  名额未公布
                                </Badge>
                              ) : remaining <= 5 ? (
                                <Badge variant="outline" className="text-[11px] text-neutral-400 border-neutral-700">
                                  余 {remaining} 席
                                </Badge>
                              ) : (
                                <Badge variant="outline" className="text-[11px] text-neutral-400 border-neutral-800">
                                  名额充足
                                </Badge>
                              )}
                            </div>

                            {/* 课程名称 */}
                            <div>
                              <h3 className="font-medium text-sm text-white tracking-tight line-clamp-1">
                                {c.course_name}
                              </h3>
                              {c.class_name && c.class_name !== c.course_name && (
                                <p className="text-[11px] text-neutral-500 truncate mt-0.5 font-mono">
                                  {c.class_name}
                                </p>
                              )}
                            </div>

                            {/* 地点与教师信息 */}
                            <div className="space-y-1 text-xs text-neutral-400 pt-2 border-t border-neutral-800">
                              <div className="flex items-center gap-2 truncate">
                                <User className="h-3.5 w-3.5 text-neutral-500 shrink-0" />
                                <span className="truncate">
                                  教师：{c.teacher_name_list || "待定"}
                                </span>
                              </div>
                              <div className="flex items-center gap-2 truncate">
                                <MapPin className="h-3.5 w-3.5 text-neutral-500 shrink-0" />
                                <span className="truncate">
                                  地点：{c.class_room_name || "待教室分配"}
                                </span>
                              </div>
                            </div>

                            {/* 容量统计 */}
                            <div className="space-y-1.5 pt-1.5">
                              <div className="flex items-center justify-between text-[11px]">
                                <span className="text-neutral-500 flex items-center gap-1 font-mono">
                                  <Users className="h-3 w-3" />
                                  <span>已报容量</span>
                                </span>
                                <span className="text-white font-mono tabular-nums">
                                  {unannounced
                                    ? `${c.selected_count} 人已报 · 名额未公布`
                                    : `${c.selected_count} / ${c.max_count} 人 (${rate}%)`}
                                </span>
                              </div>
                              <Progress
                                value={c.selected_count}
                                max={c.max_count || 1}
                                indicatorColor={progressColor}
                              />
                            </div>

                            {/* 操作按钮区 */}
                            <div className="pt-2 border-t border-neutral-800/70 mt-0.5 flex flex-col gap-1.5">
                              {/* 窗口开启后呈现官网同款【报名】或【退选】主操作按钮 */}
                              {t.in_date_range || stateData?.window_opened ? (
                                <>
                                  {c.btn_type === 1 ? (
                                    <Button
                                      variant="outline"
                                      size="sm"
                                      disabled={actionLoading.has(c.id) || !c.can_select}
                                      onClick={() => setExitModalClass(c)}
                                      title={c.title || (c.can_select ? "点击退选此课程" : "当前无法退选")}
                                      className="w-full flex items-center justify-center gap-1.5 text-xs h-8 border-red-500/40 text-red-400 hover:bg-red-500/10 hover:border-red-500/60 transition-colors"
                                    >
                                      <LogOut className="h-3.5 w-3.5" />
                                      <span>{actionLoading.has(c.id) ? "退选中..." : (c.btn_text || "退选")}</span>
                                    </Button>
                                  ) : c.btn_type === 2 ? (
                                    <Button
                                      variant="primary"
                                      size="sm"
                                      disabled={actionLoading.has(c.id) || !c.can_select}
                                      onClick={() => handleSelectClass(c)}
                                      title={c.title || (c.can_select ? "点击立即报名" : "不在选修报名时间范围内，无法选课！")}
                                      className="w-full flex items-center justify-center gap-1.5 text-xs h-8 disabled:opacity-40"
                                    >
                                      <Check className="h-3.5 w-3.5" />
                                      <span>{actionLoading.has(c.id) ? "报名中..." : (c.btn_text || "报名")}</span>
                                    </Button>
                                  ) : null /* btn_type 非 1/2（异常值）：官网契约不渲染操作按钮，只留后台冲刺目标 */}
                                  <Button
                                    variant={isSelected ? "outline" : "ghost"}
                                    size="sm"
                                    onClick={() => pick(t.publish_id, c)}
                                    className="w-full flex items-center justify-center gap-1 text-[11px] h-7 text-neutral-400 hover:text-white"
                                  >
                                    <BookMarked className="h-3 w-3" />
                                    <span>{isSelected ? `已设为后台冲刺${priorityName(selIdx)}` : "设为后台冲刺目标"}</span>
                                  </Button>
                                </>
                              ) : (
                                /* 窗口开启前：标准自动预选设置 */
                                <Button
                                  variant={isSelected ? "outline" : "primary"}
                                  size="sm"
                                  onClick={() => pick(t.publish_id, c)}
                                  className="w-full flex items-center justify-center gap-1.5 text-xs h-8"
                                >
                                  {isSelected ? (
                                    <>
                                      <Check className="h-3.5 w-3.5 text-white" />
                                      <span>{priorityName(selIdx)}</span>
                                    </>
                                  ) : (
                                    <>
                                      <BookMarked className="h-3.5 w-3.5" />
                                      <span>设为预选目标</span>
                                    </>
                                  )}
                                </Button>
                              )}
                            </div>
                          </CardContent>
                        </Card>
                      )
                    })}

                    {filteredClasses.length === 0 && (
                      <div className="col-span-full rounded-[var(--radius-lg)] border border-dashed border-neutral-700 bg-black/10 p-10 text-center text-xs text-neutral-400">
                        没有符合当前搜索或筛选条件的选修课程
                      </div>
                    )}
                  </div>
                </TabsContent>
              )
            })}
          </Tabs>
        )}

        {/* 选课改动自动保存，无需手动按钮；底部留白避免内容被遮挡 */}
        <div className="h-20 sm:h-16" aria-hidden />

        {/* 退选二次确认极简黑白 Modal (复刻官网 layer.confirm("确认退选该选修课?")) */}
        {/* F7-03：补对话语义——role=dialog/aria-modal/aria-labelledby，读屏可识别 */}
        {/* F21-03：补 Esc 关闭——有 role=dialog 却无 keydown 处理，键盘用户只能
            Tab 到按钮；与取消按钮同逻辑，退选中（actionLoading）不响应防误关 */}
        {exitModalClass && (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md p-4 animate-in fade-in duration-150"
            role="dialog"
            aria-modal="true"
            aria-labelledby="exit-modal-title"
            onKeyDown={(e) => {
              if (e.key === "Escape" && !actionLoading.has(exitModalClass.id)) {
                setExitModalClass(null)
              }
            }}
          >
            <div className="relative w-full max-w-sm rounded-[var(--radius-lg)] border border-neutral-800 bg-[#09090b] p-5 shadow-2xl space-y-4">
              <div className="flex items-start gap-3">
                <div className="p-2 rounded-full bg-red-500/10 text-red-400 border border-red-500/20 shrink-0">
                  <AlertTriangle className="h-4 w-4" />
                </div>
                <div className="space-y-1">
                  <h3 id="exit-modal-title" className="text-sm font-medium text-white tracking-wide">确认退选该选修课？</h3>
                  <p className="text-xs text-neutral-400 leading-relaxed">
                    课程：<span className="text-white font-mono">{exitModalClass.course_name}</span>
                    <br />
                    退选后名额将被立即释放，您可以重新选择其他空余课程。
                  </p>
                </div>
              </div>
              <div className="flex items-center justify-end gap-2 pt-2 border-t border-neutral-900">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setExitModalClass(null)}
                  disabled={actionLoading.has(exitModalClass.id)}
                  className="text-xs h-8"
                  autoFocus
                >
                  取消
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => handleConfirmExit(exitModalClass)}
                  disabled={actionLoading.has(exitModalClass.id)}
                  className="text-xs h-8 bg-red-600 hover:bg-red-500 text-white border-none"
                >
                  {actionLoading.has(exitModalClass.id) ? "退选中..." : "确认退选"}
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

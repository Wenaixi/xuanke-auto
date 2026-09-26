import { memo, useMemo, useState, useEffect, useRef } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { api, selectElective, exitElective } from "../api/client"
import type { Account, ClassItem, ElectivesData, SchedulerState } from "../types"
import { Button } from "../components/ui/Button"
import { Input } from "../components/ui/Input"
import { Card, CardContent } from "../components/ui/Card"
import { Badge } from "../components/ui/Badge"
import { Progress } from "../components/ui/Progress"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "../components/ui/Tabs"
import { useToast } from "../components/ui/Toast"
import { useTickingCountdown } from "../lib/useTickingCountdown"
import { cleanStaleSelected, selectedHasStalePublish } from "../lib/targetGuard"
import { formatOpenMoment, priorityName, resolveCountdownTarget } from "../lib/courseView"
import { useTargetSave } from "../lib/useTargetSave"
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


// CountdownLeaf 选课大厅倒计时叶子（memo 化）：每秒 cd.* 的 tick 只重渲染这
// 六个数字文本节点。与 Dashboard 的 MemoCountdownMatrix 同构——useTickingCountdown
// 每秒 setNow 归属路由宿主即重渲染宿主，宿主因自身 props/state 无变化而快速 bail out，
// 重渲染成本从 1290 行整树降到这个叶子。props 仅六个数独数字 + isExpired 布尔，
// 每秒传入的四个数字串字符串引用稳定（padStart 返回同值字符串），memo 浅比较全命中。
function CountdownLeaf({
  days,
  hours,
  minutes,
  seconds,
  isExpired,
}: {
  days: string
  hours: string
  minutes: string
  seconds: string
  isExpired: boolean
}) {
  return (
    <span className="text-white font-mono tabular-nums">
      {isExpired ? (
        "00 天 00 时 00 分 00 秒"
      ) : (
        <>
          {days} 天 {hours} 时 {minutes} 分 {seconds} 秒
        </>
      )}
    </span>
  )
}
const MemoCountdownLeaf = memo(CountdownLeaf)

export default function Select({ account, sessionToken, onDone }: Props) {
  const queryClient = useQueryClient()
  const { toast } = useToast()
  // 在飞操作从单值改 Set<number> 按课程 id 独立跟踪——
  // 单值 actionLoading 被并发不同课程操作互相覆盖（A 在飞时点 B 会覆盖 A 的标记，
  // A 的 finally 清 null 又把 B 的在飞态抹掉，用户再点 B 发第三发请求被后端
  // TryAcquireSubmit 拒绝 → 假失败 toast 在"不同课程"维度复发）。
  const [actionLoading, setActionLoading] = useState<ReadonlySet<number>>(new Set())
  const [exitModalClass, setExitModalClass] = useState<ClassItem | null>(null)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["electives", account, sessionToken],
    queryFn: () => api<ElectivesData>("/electives?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: (query) => {
      // 轮询判定来源：in_date_range 与 window_opened 双信号合并。
      // 窗口即将开启的瞬间平台会短暂返回空 publishes，此时仅凭 in_date_range 会把
      // 10s 慢轮询带到黄金期——必须并入调度器侧 window_opened 信号，一开窗立即升频 2s。
      // 窗口已关闭（window_closed）并入降频——关闭后课程列表已被平台
      // 清空，继续 10s 高频打 findElectivesData 纯浪费；与 /state 同信号降 30s，全站统一。
      // 自身失败态优先降频——react-query 失败后 data 为最后一次成功值或
      // undefined（失败不清缓存 data），原回调在 /state 失败（缓存无 data，st===undefined）
      // 期间只看 inRange：开窗瞬间 publishes 短暂为空时 inRange=false → 每 10s 慢轮询进
      // 黄金期，窗口状态模糊；且失败态恒不降频（同失败降频未覆盖 /electives 的另一半）。
      // 失败即 30s 降频（不再 2s/10s 轰炸代理层），成功态才走升/降频逻辑。
      // 澄清：window_closed 读组件闭包 stateData（/state 查询数据）——
      // electives 自身响应（ElectivesData）无 window_closed 字段，且 /state 每 2s 刷新
      // 触发组件重渲染，react-query 用最新闭包重调度轮询间隔，闭包永不陈旧。
      // 注意：此处绝不直接读组件顶部的 stateData（声明在下方）——refetchInterval 回调
      // 在 useQuery 创建实例时即被同步调用，此刻 stateData 的 const 声明尚未执行，
      // 直读会命中 JS 暂存死区（TDZ）抛 ReferenceError，整个组件渲染中断黑屏。
      // 改从 react-query 缓存按查询 key 读取 /state 最新值，与 stateData 同源且零时序依赖。
      if (query.state.error || query.state.status === "error") return 30000
      const st = queryClient.getQueryData<SchedulerState>(["state", account, sessionToken])
      const pubs = query.state.data?.publishes ?? []
      const inRange = pubs.some((p) => p.in_date_range)
      if (st?.window_closed) return 30000
      return inRange || st?.window_opened ? 2000 : 10000
    },
  })

  // 手动报名指定课程
  const handleSelectClass = async (c: ClassItem) => {
    // 在飞幂等守卫——与 login submit / 激活
    // 同款短路：disabled 渲染落地前双击/连点会发出两个并发报名，后端 TryAcquireSubmit
    // 拒绝第二个（"该课程正在提交中"），但第一个已 MarkDone 成功、第二个的 finally 仍
    // invalidateQueries 造成假失败 toast；退选同源。入口先查在飞标记即停。
    // 守卫与置位改用 Set 按课程独立跟踪，只拦"本课程在飞"。
    if (actionLoading.has(c.id)) return
    setActionLoading((prev) => new Set(prev).add(c.id))
    try {
      const res = await selectElective(c.id, sessionToken, account, c.course_name)
      toast({ title: "报名成功", description: res.msg || "已成功选报该课程", variant: "success" })
    } catch (err: any) {
      toast({ title: "报名失败", description: err.message || "请求被拒绝", variant: "destructive" })
    } finally {
      // 无论成功失败都强制失效 electives/state 缓存——
      // 报名失败（满员/窗口关闭）后名额与按钮状态同样已变化，必须立即刷新，
      // 否则前端显示"还可报名"实则已满，用户看到的是过期数据。
      queryClient.invalidateQueries({ queryKey: ["electives"] })
      queryClient.invalidateQueries({ queryKey: ["state"] })
      // 函数式清除只删自己的 id——绝不抹掉其他仍在飞的课程标记。
      setActionLoading((prev) => {
        const n = new Set(prev)
        n.delete(c.id)
        return n
      })
    }
  }

  // 手动退选指定课程二次确认提交
  const handleConfirmExit = async (c: ClassItem) => {
    // 与 handleSelectClass 同款在飞幂等守卫（双击退选第二个请求
    // 会被后端 TryAcquireSubmit 拒、假失败 toast）。
    // 与报名同款 Set 在飞跟踪——退选/报名并发互不覆盖。
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
      // 与报名同款函数式清除，只删自己的退选在飞标记。
      setActionLoading((prev) => {
        const n = new Set(prev)
        n.delete(c.id)
        return n
      })
    }
  }

  // 查询当前调度器已保存的目标课程并自动回显（会话绑定当前账号）。
  // 加 2s 自轮询——electives 的升频判定依赖 window_opened 信号，
  // 若此查询被动等 electives invalidate 才刷新，开窗瞬间（publishes 短暂为空）会把
  // 10s 慢轮询带进黄金期；独立轮询让 window_opened 一开窗立即升频 2s，两信号同源。
  // 窗口已关闭后降回 30s——与 Dashboard 同一信号同一次序，
  // 避免窗口关闭后仍 2s 高频打 /state 刷屏日志。
  const { data: stateData } = useQuery({
    queryKey: ["state", account, sessionToken],
    queryFn: () => api<SchedulerState>("/state?account=" + encodeURIComponent(account), { session: sessionToken }),
    refetchInterval: (query) => {
      // 失败态/无数据时统一降频 30s——react-query 失败后 data 为最后一次
      // 成功值或 undefined，原回调查询失败时恒取 2000ms，网络挂断/后端重启期间
      // /state + /electives 双查询叠加固定 2s 轰炸日志与代理层（与"失败分级退避"防
      // 轰炸理念相悖）。error 或 status==="error" 即降频，成功态按 window_closed 升/降频。
      if (query.state.error || query.state.status === "error") return 30000
      return query.state.data?.window_closed ? 30000 : 2000
    },
  })

  const [selected, setSelected] = useState<Record<number, ClassItem[]>>({})
  // 回显一次性标记——回显 effect 只合并一次，绝不重放。
  // rev>0 守卫保护的"用户清空目标后轮询旧 courses 再次回填撤销清空"语义
  // 在这里由"只合并一次"延续：用户改动后的轮询不再重放回显（见下方回显 effect）。
  const echoedRef = useRef(false)
  // 用户真实改动计数：驱动自动保存的 400ms 防抖；回显数据不经过它，故不会触发无意义保存。
  // 注意：修复后它只归 pick()/清空操作自增——轮询拉回的 publishes 变化绝不触发保存。
  const [rev, setRev] = useState(0)
  const [search, setSearch] = useState("")
  const [onlyAvailable, setOnlyAvailable] = useState(false)
  const [sortTightest, setSortTightest] = useState(false)
  // 激活 Tab 受控兜底——发布集合整体重建（开窗瞬间平台清空又
  // 恢复、publish_id 全变）时，非受控 defaultValue 只首次生效、激活 Tab 对应的 Trigger
  // 从列表消失后 Radix Tabs 无 fallback，主区空白直到用户手点。受控 value 跟随
  // tabs[0]，发布重建即回落首个 Tab，绝不悬空。
  const [activeTab, setActiveTab] = useState<string | null>(null)
  // 跨账号组件实例复用防护——App 两处挂载点已加 key={account}，账号切换即整体
  // 重建实例（selected/echoedRef/rev 全复位），此处守卫只兜底"未来改为不重置挂载"的
  // 意外回归：account 变化时同步复位回显/编辑态，绝不让旧账号残留目标污染新账号
  // （401 被动吊销自动切剩余账号时，旧账号 selected 会被防抖 PUT 整包覆盖掉新账号目标）。
  // 声明于 echoedRef/rev/setRev 之后（本文件顶部状态区），TDZ 不触发。
  const [accountKey, setAccountKey] = useState(account)
  if (accountKey !== account) {
    setAccountKey(account)
    setSelected({})
    setRev(0)
    echoedRef.current = false
  }

  // 本地每秒刷新倒计时：收敛到 lib/useTickingCountdown 自 tick 组件。
  // 注意：hook 在路由组件顶层调用，每秒 setNow 触发的是本路由组件整树重渲染
  // （React 语义：useState 归属宿主即重渲染宿主），DOM 差分成本可忽略；
  // 如需真正做到「只重渲染倒计时一处」需拆独立 memo 叶子组件（潜在优化，非当前承诺）。

  // 进入页面时自动回显已保存的目标课程（含多备选优先级）。
  // 函数体内统一用 prev 构造初始值，杜绝 `const initial` 遮蔽
  // 外部 `selected` 导致数据重取后回显永久失效的问题。
  // 用户已编辑过目标（rev>0）时跳过回显——用户"清空全部目标"后 2s 轮询
  // 返回的旧 courses 若再次回填，会把清空静默撤销并重新保存旧目标（回显与防抖保存竞态）。
  // 回显的 courses 可能携带"不属于当前发布集合"的 publish_id（旧学期
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
    // 旧目标（"添加一门"变"替换全部"），且真合并、等待、
    // 守卫全部失效。继续等待 /state；持续失败由轮询自愈，改动滞留不覆盖（安全方向）。
    if (stateData === undefined) return
    const courses = stateData.courses ?? []
    if (courses.length === 0) {
      // /state 首帧到达且确证后端无旧目标（courses 空）：echoed 完成——否则全程无旧
      // 目标的账号用户改动会被防抖回显守卫永久拦下（置脏无自愈信号）。清空语义/全
      // 清空守卫不受影响（echoDone 布尔只做放行信号，不写 selected）。
      echoedRef.current = true
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
    // 发布集合整体重建（开窗瞬间平台清空又恢复、publish_id 全变）后，
    // selected 仍残留旧 publish_id 的非空 key——对应 Tab 已消失、用户无法通过界面
    // 清除，守卫命中的"置脏跳过"会把保存链永久静默拦截（黄金期改目标永不落库）。
    // 随重建清理已下沉为独立 effect（见 publishes 声明之后）——原本挂在
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
    // 回显完成信号（echoDone）已由 useTargetSave 内纯数据判据重建
    //（stateData 到达 && courses 字段存在）——/state 到达驱动防抖 effect 重跑自愈，
    // 语义与旧 setEchoDone(true) 等价（见 hook 防抖 effect 依赖注释）。
  }, [stateData, data, rev, selected, toast])

  const publishes = data?.publishes ?? []
  // 发布集合重建清理独立 effect——原挂在回显 effect（首行
  // `if (echoedRef.current) return`）内，已完成回显的账号 echoedRef 恒 true，回显
  // effect 不再执行，之后发布集合整体重建（开窗瞬间平台清空又恢复、publish_id 全变）
  // 残留旧 publish_id 的 selected 永不被清理；挡在它前面的 stale 守卫把保存链静默
  // 锁死至整页刷新（黄金期最不该打断用户的操作）。
  // 本 effect 依赖 [publishes, selected, echoedRef, toast]——selected 加入依赖：
  // 发布重建瞬间（publishes 引用变）effect 用"重建前的旧 selected"清理一次，若同一
  // 时刻 /state 首帧交错到达（回显 effect 其后把旧 publish_id 课程合并进 selected），
  // 合并必然带出 stale 非空 key——依赖补 selected 后合并的那次渲染本 effect 随
  // selected 变化重跑，掉队的旧残留及时被清理，不再依赖"清理先于回显合并"的时序
  // 巧合；cleanStaleSelected 无变更返回原引用、不引出不必要的重渲染，清理也会随
  // selected 变化自然地触发防抖 effect（依赖含 selected）重跑落库当前目标。
  // 未回显（echoedRef=false）时不清理——回显合并与 currentIds 同判据过滤幽灵条目，
  // 此刻抢先清理可能干扰重建前旧目标的合并/回显时序，绝无必要。
  // 只删"非空且不在当前发布集合"的 key（空数组键 = 用户主动清空，保留语义）。
  // 声明在 `const publishes` 之后合理使用已声明常量（同款 TDZ 防护）。
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
  }, [publishes, selected, echoedRef, toast])

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
  // toast 移出 setSelected 的 updater——React 明令 updater 必须是
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

  // 目标自动保存收权 useTargetSave：保存链（镜像 ref/串行化/退避/守卫/防抖/
  // handleBack 收敛）全部移入 hook，本组件瘦回渲染职责。echoedRef 仍归回显 effect
  //（渲染态职责），hook 只读它做守卫第三参。
  const save = useTargetSave({
    account,
    sessionToken,
    selected,
    rev,
    publishes,
    stateData,
    echoedRef,
    toast,
  })

  const selectedCount = Object.values(selected).reduce((n, arr) => n + arr.length, 0)

  // 选课开放时间倒计时（来自调度器状态——平台 beginTimes 自动识别唯一事实源，不可配置；
  // 识别不到或识别过期 = 未知，绝不显示编造时间）。
  const openTimeStr =
    stateData?.open_time_known && stateData.open_time
      ? stateData.open_time
      : null
  // 识别缺席时用 begin_times[0] 兜底（与 Dashboard 同源收 lib/courseView）——
  // 同一浏览器主看板有确切倒计时、选课大厅全 00 的跨页矛盾收口；识别槽建立后
  // 仍以识别真值为准。
  const cd = useTickingCountdown(resolveCountdownTarget(openTimeStr, data?.begin_times))

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
                // 返回前先 flush 挂起的防抖/重发目标保存——
                // 直接 onDone 会卸载组件、400ms 防抖 timer 被清理，最后一次点选
                // 到返回间隔 <400ms 时整批目标永不 PUT。
                // 改为等待保存链静止的异步句柄——flush 后若
                // 在飞 PUT 完成会经 finally 自动补发（见 hook 内 handleBack），全部落定才卸载。
                void save.handleBack(onDone)
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
            {/* 已开放状态以调度器 window_opened 为准（服务端有 ~640ms 校准偏差，
                本地倒计时到点 ≠ 平台开窗）；window_opened 才显示"已开放"高亮。
                此前 `|| cd.isExpired` 让窗口关闭后（open_time 为
                过去时刻 → isExpired 恒 true）横幅永远显示"已开放"，与同屏空态卡
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
            ) : !openTimeStr && data?.begin_times?.[0] == null ? (
              <span className="truncate">
                未识别到开放时间
                <span className="text-neutral-500">（平台尚未下发或识别已过期）</span>
              </span>
            ) : cd.isExpired ? (
              <span className="truncate">本地已到开窗点，等待平台窗口开放...</span>
            ) : (
              <span className="truncate">
                距开放还有{" "}
                <MemoCountdownLeaf
                  days={cd.days}
                  hours={cd.hours}
                  minutes={cd.minutes}
                  seconds={cd.seconds}
                  isExpired={false}
                />
              </span>
            )}
          </div>
          <span className="text-neutral-500 font-mono hidden sm:block shrink-0">
            {formatOpenMoment(openTimeStr, data?.begin_times, "未知")}
          </span>
        </div>

        {/* 搜索与条件过滤栏 */}
        <div className="flex flex-col sm:flex-row items-center gap-3 p-3 rounded-[var(--radius-lg)] glass border border-neutral-800">
          <div className="relative flex-1 w-full">
            <Search className="absolute left-3.5 top-3 h-4 w-4 text-neutral-500" />
            <Input
              placeholder="搜索课程名称、教师或教室"
              aria-label="搜索课程名称、教师或教室"
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

        {/* 窗口关闭/学期无发布空态——publishes 恒空时给出明确说明，
            不再是"无提示空白主体"（此前徽章还误显 n/0） */}
        {!isLoading && !isError && tabs.length === 0 && (
          <div className="rounded-[var(--radius-lg)] glass border border-neutral-800 p-16 text-center text-xs text-neutral-400 flex flex-col items-center justify-center gap-3">
            <span className="inline-block w-2 h-2 rounded-full bg-neutral-600" />
            <span>当前无可选课程批次（选课窗口未开放或已关闭）</span>
            <span className="text-neutral-600">窗口开放后课程列表将自动出现</span>
          </div>
        )}

        {/* 主选课 Tab 分段控制器
            窗口关闭/学期无发布时 publishes 恒空 →
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

                // 满员与否直接读后端派生 class_full（单一记忆点，架构深化 C）——
                // "仅看有余量"滤掉已满课程；max_count=0（名额未公布）时 class_full=false 照常显示。
                const matchAvailable = !onlyAvailable || !c.class_full
                return matchSearch && matchAvailable
              })

              // 剩余名额正序：名额越少越靠前，抢手课程一眼可见。
              // 排序键必须按"剩余名额 = max_count - selected_count"而非已报名数——
              // 两课上限不同时已报少≠剩余少（1/5 余4 与 10/100 余90，应前者靠前）。
              // max_count=0（名额未公布）映射为 0（最紧张），与 class_full 同源语义。
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
                      // 满员与否直接读后端派生 class_full（单一记忆点，架构深化 C）：
                      // 后端解析端按 max_count>0 && selected_count>=max_count 算一次，
                      // 0=名额未公布（class_full=false）绝不误显"已满额"徽章。
                      const isFull = c.class_full
                      // 剩余名额展示：max_count>0 才可算余量，未公布（0）显示"余 0"会误导，
                      // 与 class_full 同源语义——未公布不是快满，徽章改显"名额未公布"。
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
                              {/* 名额未公布（max_count=0）时 Progress 空条——原实现
                                  max={c.max_count || 1} 把分母变 1，selected_count 数十到数百
                                  渲染成满条，与"名额未公布"文案并存误导。value=0 恒空条，
                                  aria-valuenow 亦如实反映"未公布无进度"语义。 */}
                              <Progress
                                value={unannounced ? 0 : c.selected_count}
                                max={unannounced ? 1 : c.max_count}
                                indicatorColor={progressColor}
                              />
                            </div>

                            {/* 操作按钮区 */}
                            <div className="pt-2 border-t border-neutral-800/70 mt-0.5 flex flex-col gap-1.5">
                              {/* 官网按钮以 btn_type 为唯一渲染判据（官网逆向契约：1=退选、2=报名、
                                  其他值不渲染操作按钮）：platform 窗口未开照样下发 btn_type=2 +
                                  can_select=false（title="不在选修报名时间范围内，无法选课！"），
                                  本项目隐藏官网按钮后若开窗瞬间窗口信号缺失（识别槽未建立 /
                                  in_date_range 刹那 false）手动抢课通道会被锁死——故按钮置灰与否
                                  由 can_select 决定（disabled + title），不再依赖窗口信号。 */}
                              {c.btn_type === 1 && (
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
                              )}
                              {c.btn_type === 2 && (
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
                              )}
                              {/* 本项目特冲刺/预选目标按钮：以窗口信号决定形态（开窗后收敛为
                                  ghost 小按钮避免淹没了官网报名主操作；闭窗前 primary 预选），
                                  only affects itself */}
                              {t.in_date_range || stateData?.window_opened ? (
                                <Button
                                  variant={isSelected ? "outline" : "ghost"}
                                  size="sm"
                                  onClick={() => pick(t.publish_id, c)}
                                  className="w-full flex items-center justify-center gap-1 text-[11px] h-7 text-neutral-400 hover:text-white"
                                >
                                  <BookMarked className="h-3 w-3" />
                                  <span>{isSelected ? `已设为后台冲刺${priorityName(selIdx)}` : "设为后台冲刺目标"}</span>
                                </Button>
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
        {/* 补对话语义——role=dialog/aria-modal/aria-labelledby，读屏可识别 */}
        {/* 补 Esc 关闭——有 role=dialog 却无 keydown 处理，键盘用户只能
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

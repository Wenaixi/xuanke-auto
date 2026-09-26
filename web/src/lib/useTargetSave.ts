import { useCallback, useEffect, useRef } from "react"
import { api } from "../api/client"
import { buildTargets, commitTargetsGuards, guardCommit, shouldDeferSave } from "./targetGuard"
import type { Publish, SchedulerState, Target } from "../types"

// useTargetSave 目标自动保存深 hook（收权：Select.tsx 保存链逐行搬移，零语义变化）。
// 收敛内容（原 Select.tsx 413-787 区间）：
//   - 镜像 ref 配对：selectedRef/revRef/stateDataRef/publishesRef（消费时刻读 ref 纪律）
//   - 保存串行化：lastJson/targetRef/savingRef/dirtyRef + saveNow（飞行中补发，绝不乱序覆盖）
//   - 退避重发：retryState/resetRetry/scheduleRetry（2/4/8/16/16s，连续 5 次停手）
//   - 卸载防护：unmountedRef + 挂载复位（StrictMode 双挂载契约）
//   - 消费时刻守卫三族：假清空（发布缺席/联查空/id 漂移）+ 回显未完成 + 发布重建 stale
//   - 防抖 effect（rev 驱动 + hasPublishes/echoDone/stateData 依赖，400ms）+ handleBack 收敛
// echoedRef 由调用方传入（回显 effect 属渲染态职责，hook 只读它做守卫第三参）。
export function useTargetSave(opts: {
  account: string
  sessionToken: string
  selected: Record<number, { id: number; publish_id: number; course_name: string }[]>
  rev: number
  publishes: readonly Publish[]
  stateData: SchedulerState | undefined
  echoedRef: React.MutableRefObject<boolean>
  toast: (t: { title: string; description: string; variant: "default" | "destructive" | "warning" }) => void
}): {
  flushTargets: () => void
  handleBack: (onDone: () => void) => Promise<void>
} {
  const { account, sessionToken, selected, rev, publishes, stateData, echoedRef, toast } = opts

  // 消费时刻读 ref 镜像（防抖/flush/handleBack 是渲染闭包捕获的异步函数，等待期间
  // 用户新改动不会更新闭包里的 selected/rev 快照——必须读 ref 拿最新状态）
  const selectedRef = useRef(selected)
  selectedRef.current = selected
  const revRef = useRef(rev)
  revRef.current = rev
  const stateDataRef = useRef(stateData)
  stateDataRef.current = stateData
  const publishesRef = useRef<readonly Publish[]>(publishes)
  publishesRef.current = publishes

  const lastJson = useRef("")
  const targetRef = useRef<Target[]>([])
  const savingRef = useRef(false)
  const dirtyRef = useRef(false)
  // 已卸载标记——组件卸载后（返回控制台）绝不再发起新的网络请求
  // 或重发退避。此前卸载 cleanup 只清"当时挂着"的退避 timer，flush 补发失败后再
  // scheduleRetry 挂的新 timer 无人清理 → 组件卸载后 2/4/8/16/16s 最多 5 次孤儿请求，
  // 每次失败都全局 toast 轰炸已回到 Dashboard 的用户。卸载后重试也毫无意义（目标
  // 后端已有、改动已尽力）——直接停手。
  const unmountedRef = useRef(false)
  // 失败重发状态——attempt 累计连续失败次数、timer 为退避重发定时器。
  // 成功或用户产生新改动都会清零；连续失败 5 次停手，等下一次改动重新驱动。
  const retryState = useRef({ attempt: 0, timer: null as ReturnType<typeof setTimeout> | null })
  // 卸载清理：中断仍在排队的退避重发定时器，防止 onDone 返回后副作用残留
  // 挂载时复位 unmountedRef——StrictMode 开发态会对组件执行
  // mount→unmount→remount 两遍，原实现只有置 true 的 cleanup、二次挂载时已卸载标记
  // 恒真，saveNow/scheduleRetry 全部短路，目标改动静默丢失且无任何报错。重挂载后
  // 复位到 false，本轮会话保存链路恢复正常；真正卸载时 cleanup 依旧置位停手（卸载后防孤儿重试契约不变）。
  useEffect(() => {
    unmountedRef.current = false
    return () => {
      unmountedRef.current = true
      if (retryState.current.timer) clearTimeout(retryState.current.timer)
    }
  }, [])

  const resetRetry = () => {
    if (retryState.current.timer) clearTimeout(retryState.current.timer)
    retryState.current.timer = null
    retryState.current.attempt = 0
  }
  // 守卫拦下（数据缺席/发布重建/联查空/漂移）是安全拦截：目标安全、后端旧目标未被
  // 抹除，且守卫命中不置 dirtyRef——终局 toast 只对"真实保存失败"（dirtyRef，仅
  // saveNow catch 与飞行中标记补发会置）触发，守卫场景自然不弹，绝无"目标保存失败"
  // 误导归因（rev>0 亦然）。
  const pendingUnsaved = () =>
    dirtyRef.current || savingRef.current || retryState.current.timer !== null

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
      // 卸载后失败也绝不 toast——与"卸载后不轰炸"意图对齐
      if (unmountedRef.current) return
      toast({
        title: "目标保存失败",
        description: e.message || "通信异常，请重试",
        variant: "destructive",
      })
      // 失败保留 dirty（内存目标仍未持久化），并安排带退避的重发——
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




  // 守卫尾段共用执行：flush 与防抖两处的联查空/id 漂移守卫逐字重复，现由纯函数
  // commitTargetsGuards 承担（判据可单测）；本壳只保留与 React 状态耦合的收尾——
  // targetRef/savingRef/dirtyRef/saveNow。两处消费点的 selected 数据源差异
  // （ref 镜像 vs 渲染闭包）保留在调用方（消费时刻读镜像纪律）。
  const commitTargets = (next: Target[], selectedCount: number, pubs: readonly Publish[]): boolean => {
    // 拦截原因不外传：两条都属"安全拦截的静默跳过"，不置 dirtyRef（终局绝不误报
    // 保存失败），等数据到达/发布恢复自愈。emptyWithSelection = 联查为空（假清空，
    // selectedCount 滞后时属误伤守卫，多等一次防抖，安全方向）；stalePublishId =
    // 发布 id 漂移（错位假清空）。
    if (!commitTargetsGuards(next, selectedCount, pubs).ok) {
      return false
    }
    targetRef.current = next
    if (savingRef.current) {
      dirtyRef.current = true // 保存进行中：标记脏，让飞行中的 PUT 完成后补发本次快照
      return true
    }
    void saveNow()
    return true
  }
  const flushTargets = () => {
    // 消费时刻读 ref：handleBack 等待循环内用户新改动后，渲染闭包的 rev/selected
    // 是旧快照，必须取 ref 里的最新值——否则等待窗口内新增的课程被忽略。
    const latestSelected = selectedRef.current
    const latestRev = revRef.current
    const latestSelectedCount = Object.values(latestSelected).reduce((n, arr) => n + arr.length, 0)
    if (latestRev === 0) return
    // 消费时刻守卫三段（回显未完成 → 发布缺席 → 旧 publish_id 残留）编排收口到
    // guardCommit：判据与顺序各只有一处定义，本处只负责"拦截时要不要提示"。
    //   defer / missingPublishes = 安全拦截的静默跳过（守卫不置 dirtyRef——终局绝不
    //   误报保存失败），等数据到达/发布恢复自愈；stalePublish 是唯一需要提示的一因
    //   （残留目标用户无法通过界面自行清除，须告知刷新解锁）。
    const verdict = guardCommit(
      stateDataRef.current,
      latestSelectedCount > 0,
      echoedRef.current,
      latestSelected,
      publishesRef.current
    )
    if (!verdict.ok) {
      if (verdict.reason === "stalePublish" && !unmountedRef.current) {
        toast({
          title: "发布已更新",
          description: "旧批次目标已失效，已停止保存。请刷新页面重新选择",
          variant: "warning",
        })
      }
      return
    }
    const targets = buildTargets(publishesRef.current, latestSelected)
    // 联查空/id 漂移守卫 + targetRef/savingRef/saveNow 收尾（架构深化 D：与防抖共用）
    commitTargets(targets, latestSelectedCount, publishesRef.current)
  }

  const handleBack = async (onDone: () => void) => {
    // 消费时刻读 selectedRef 算"当前是否留有选中"（handleBack 无渲染闭包可直接用）：
    // shouldDeferSave 第二参数——首帧携带旧目标但用户已全清空（hasSelected=false）时
    // 放行立即保存，绝不等 5s 又当"待回显"打回。
    const hasSelectedNow = () =>
      Object.values(selectedRef.current).reduce((n, arr) => n + arr.length, 0) > 0
    // 回显合并先行：/state 首帧晚于用户首次点击到达时，后端旧目标尚未经回显 effect
    // 合并进 selected——此刻直接 flush 会用当前 selected（只含用户新改动）整包 PUT
    // 覆盖删掉后端旧目标。只有回显合并完成（/state 已到且 courses 为空）或首帧携带
    // 旧目标已合并完毕才可立即开始保存；等待期间回显 effect 把旧目标补进 selected。
    // 5s 兜底：/state 持续失败时合并永不发生，等无可等继续——flush 内假清空守卫仍拦截。
    if (revRef.current > 0 && shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current)) {
      const deadline = Date.now() + 5000
      // 轮询间隔 50ms：回显合并是 React 状态更新+渲染（一帧约 16ms），50ms 足够感知
      // 完成且不抢调度；10ms 会让 5s 窗口内连开约 500 个定时器空转主线程。
      while (shouldDeferSave(stateDataRef.current, hasSelectedNow(), echoedRef.current) && Date.now() < deadline) {
        await new Promise((r) => setTimeout(r, 50))
      }
      // 等合并 effect 的 setSelected 渲染提交落地，selectedRef 同步到含旧目标的合并结果
      await new Promise((r) => setTimeout(r, 0))
    }
    for (let i = 0; i < 3; i++) {
      // 首帧未到等满 5s 后，若回显合并仍未发生（/state 持续失败），继续 flush
      // 是唯一合法路径——flush 内的假清空守卫（发布缺席 + 已有选中）仍拦截覆盖。
      flushTargets()
      // flush 已消费本轮最新 ref 快照，但 break 前必须等 React 下一帧落地——
      // 若等待窗口刚有用户改动（pick 的 setRev → effect 挂 400ms 防抖 timer，异步）
      // 此刻还没触发，onDone 同步卸载会清掉 timer，改动静默丢失。等一帧后复查
      // revRef：与本轮 flush 消费的一致才真正静止；又变了就多等一轮 flush 收敛。
      const flushedRev = revRef.current
      if (!dirtyRef.current && !savingRef.current) {
        await new Promise((r) => setTimeout(r, 0))
        if (revRef.current === flushedRev) break
        continue // 更晚的改动涌进来：多等一轮防抖/flush 收敛再卸载
      }
      // 保存链"待定工作"不只在飞 PUT——退避重试 timer 排队中同样表示内存与后端
      // 分叉、改动未落库。等待 timer 触发后 saveNow 的 finally 自接补发链收敛；
      // 持续失败则超时兜底，绝不无限挂起。
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
    // 终局提示：仅当"用户真实改动过（rev>0）且保存链有真实失败"才提示。判据
    // pendingUnsaved() 三信号（dirtyRef 真实失败/savingRef 在飞/timer 退避排队）为
    // "确实未落库"；守卫拦下的假清空脏块是安全拦截（守卫不置 dirtyRef）绝不误报；
    // rev=0（纯浏览/无改动）绝不报"改动未落库"。同 title 去重合并机制防轰炸。
    if (revRef.current > 0 && pendingUnsaved()) {
      toast({
        title: "目标保存失败",
        description: "改动未落库，返回后将以服务端保存的目标为准",
        variant: "destructive",
      })
    }
    onDone()
  }

  // 自动保存防抖 effect：选课一变（仅用户点击 rev），400ms 后整包 PUT 到后端。
  // hasPublishes 布尔信号——开窗瞬间平台清空 publishes（假清空守卫拦下置脏）后发布恢复，
  // 只有 rev 驱动的话 effect 不重跑、无新 timer，置脏的改动永不落库；hasPublishes 从
  // false→true 时 effect 重跑 → 新 400ms timer → 消费时刻守卫通过 → 正常保存。
  // echoDone：回显完成驱动 effect 重跑（场景 B 后端确证无旧目标只置 echoedRef 不改 selected）。
  // stateData：/state 数据到达触发 effect 重跑自愈（守卫命中置脏后 selected 无变化
  // bailout 不重跑，/state 到达后重跑挂新 timer 守卫通过即落库）。
  const hasPublishes = (publishes?.length ?? 0) > 0
  const echoDone = stateData !== undefined && stateData.courses !== undefined
  useEffect(() => {
    if (rev === 0) return
    // 用户新改动接管——中断失败重发退避，下一轮保存由正常防抖路径驱动
    resetRetry()
    const timer = setTimeout(async () => {
      // 消费时刻守卫三段与 flushTargets 同源（guardCommit）——判据与顺序单点定义。
      // 守卫读 ref 镜像 selectedRef.current（timer 是异步回调，渲染闭包的 selected
      // 可能是旧快照），而下方 buildTargets 仍用渲染闭包 selected：数据源
      const selectedCount = Object.values(selectedRef.current).reduce((n, arr) => n + arr.length, 0)
      const verdict = guardCommit(
        stateDataRef.current,
        selectedCount > 0,
        echoedRef.current,
        selectedRef.current,
        publishesRef.current
      )
      if (!verdict.ok) {
        if (verdict.reason === "stalePublish" && !unmountedRef.current) {
          toast({
            title: "发布已更新",
            description: "旧批次目标已失效，已停止保存。请刷新页面重新选择",
            variant: "warning",
          })
        }
        return
      }
      const next = buildTargets(publishesRef.current, selected)
      // 联查空/id 漂移守卫 + targetRef/savingRef/saveNow 收尾（架构深化 D：与 flush 共用）
      commitTargets(next, selectedCount, publishesRef.current)
    }, 400)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rev, selected, sessionToken, toast, hasPublishes, echoDone, stateData])

  return { flushTargets: useCallback(flushTargets, [echoedRef, toast]), handleBack: useCallback(handleBack, [echoedRef, toast]) }
}
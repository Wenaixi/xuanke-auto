// TDD 守护：selectedHasStalePublish 纯函数断言——
// 发布集合整体重建（平台开窗瞬间清空又恢复、publish_id 全变）后，selected 仍残留
// 旧 publish_id 的非空数组；构建目标只遍历当前发布集合会静默丢弃它们，产出"仅含新
// 发布课程"的整包 PUT 整包覆盖删除后端旧目标（数据丢失）。守卫必须判"有过期条目"
// 并置脏跳过。空数组键 = 用户主动清空该发布（清空语义绝不复活），绝不判过期。
// 用法：node --import tsx scripts/target-guard-check.ts（退出码非 0 即断言失败）
import { selectedHasStalePublish, cleanStaleSelected } from "../src/lib/targetGuard"

let failed = 0
const assert = (name: string, got: boolean, want: boolean) => {
  const ok = got === want
  console.log(`${ok ? "  ✓" : "  ✗"} ${name}${ok ? "" : `（期望 ${want}，实际 ${got}）`}`)
  if (!ok) failed++
}

const pubs = [
  {
    publish_id: 9,
    publish_name: "校本2",
    begin_date: "2026-09-20",
    in_date_range: true,
    can_select: 0,
    has_selected: 0,
    group_count: 0,
    total_count: 1,
    classes: [],
  },
]

// 场景 A（缺陷触发）：旧发布 P1 仍有课 + 新发布 P9 新课 → 必须判"有过期条目"置脏，
// 绝不让只含 P9 课程的整包 PUT 覆盖删除后端 [P1课,P9课]
assert("旧发布 P1 非空残留 → 置脏", selectedHasStalePublish({ 9: [{ id: 11 }], 1: [{ id: 2 }] }, pubs), true)
// 场景 B：全部 key 都属于当前发布 → 无过期条目，正常放行
assert("全部 key 属当前发布 → 放行", selectedHasStalePublish({ 9: [{ id: 11 }] }, pubs), false)
// 场景 C（清空语义绝不复活）：旧发布 key 为空数组 = 用户主动清空 → 不判过期
assert("空数组 key = 用户清空 → 放行", selectedHasStalePublish({ 1: [] }, pubs), false)
// 场景 D：新发布非空 + 旧发布已清空 → 无非空过期条目
assert("新发布非空 + 旧发布清空 → 放行", selectedHasStalePublish({ 9: [{ id: 11 }], 1: [] }, pubs), false)
// 场景 E：selected 为空对象 → 无过期条目
assert("selected 全空 → 放行", selectedHasStalePublish({}, pubs), false)

// F40-M1：cleanStaleSelected 清理断言——发布集合重建时删除"非空且不在当前集合"的 key
const cleanAssert = (name: string, got: Record<number, unknown[]>, want: Record<number, unknown[]>) => {
  const ok = JSON.stringify(got) === JSON.stringify(want)
  console.log(`${ok ? "  ✓" : "  ✗"} ${name}${ok ? "" : `（期望 ${JSON.stringify(want)}，实际 ${JSON.stringify(got)}）`}`)
  if (!ok) failed++
}

const currentIds = pubs.map((p) => p.publish_id)
// 场景 F：旧发布 P1 非空 + 不在当前集合 → 被清理，只留当前集合 key
cleanAssert(
  "旧 key 非空不在集合 → 清理",
  cleanStaleSelected({ 9: [{ id: 11 }], 1: [{ id: 2 }] }, currentIds),
  { 9: [{ id: 11 }] }
)
// 场景 G：旧发布 key 空数组（用户清空语义）→ 保留
cleanAssert("空 key 不在集合 → 保留", cleanStaleSelected({ 1: [] }, currentIds), { 1: [] })
// 场景 H：当前集合 key → 保留原样
cleanAssert(
  "当前集合 key → 保留",
  cleanStaleSelected({ 9: [{ id: 11 }] }, currentIds),
  { 9: [{ id: 11 }] }
)
// 场景 I：混合——新发布非空保留、旧发布空数组保留、旧发布非空清理
cleanAssert(
  "混合 → 只清非空旧 key",
  cleanStaleSelected({ 9: [{ id: 11 }], 1: [{ id: 2 }], 3: [] }, currentIds),
  { 9: [{ id: 11 }], 3: [] }
)
// 场景 J：cleanStaleSelected 不产生任何变更时返回原对象引用（防不必要重渲染）
const original = { 9: [{ id: 11 }] }
cleanAssert(
  "无变更 → 返回原引用",
  cleanStaleSelected(original, currentIds) === original ? original : { __changed__: true },
  original
)

if (failed > 0) {
  console.error(`\ntarget-guard 断言失败 ${failed} 项`)
  process.exit(1)
}
console.log("\ntarget-guard 断言全绿")
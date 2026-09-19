// TDD 守护：extractAccountFromPath 纯函数断言——401 广播前置化（F41-N2）后，
// "?account= 穿透目标"反查逻辑从响应处理分支抽出为纯函数，供 HTTP 状态码 401 分支
// 在 r.json() 之前复用。缺陷背景：反向代理/网关可能返回 HTML/文本 401（非 JSON 响应
// 体），原实现 r.json() 先执行会抛错走 -2 文案、UNAUTHORIZED_EVENT 永不广播，失效会话
// 账号在前端永久残留。修复后 401 事件先广播、JSON 解析失败仍抛 -2 文案但事件已到达。
// 本脚本验证账号反查解析逻辑；广播副作用（window.dispatchEvent）由逻辑走查论证。
// 用法：node --import jiti scripts/unauthorized-check.ts（退出码非 0 即断言失败）
import { extractAccountFromPath } from "../src/api/client"

let failed = 0
const assert = (name: string, got: string, want: string) => {
  const ok = got === want
  console.log(`${ok ? "  ✓" : "  ✗"} ${name}${ok ? "" : `（期望 "${want}"，实际 "${got}"）`}`)
  if (!ok) failed++
}

// 场景 A：?account= 居中参数 → 反查出穿透目标账号名
assert("居中 account 参数 → 账号名", extractAccountFromPath("/state?account=%E6%9D%8E%E5%9B%9B&p=1"), "李四")
// 场景 B：末尾 account 参数
assert("末尾 account 参数 → 账号名", extractAccountFromPath("/targets?account=admin"), "admin")
// 场景 C：无 account 参数 → 空串（事件 detail.account 留空，App 按 session 令牌反查）
assert("无 account 参数 → 空串", extractAccountFromPath("/electives?x=1"), "")
// 场景 D：路径含字面 account= 但不在 query（如伪路径）→ 空串
assert("非 query 段 account= → 空串", extractAccountFromPath("/account=xxx/state"), "")
// 场景 E：空路径 → 空串
assert("空路径 → 空串", extractAccountFromPath(""), "")

if (failed > 0) {
  console.error(`\nunauthorized 断言失败 ${failed} 项`)
  process.exit(1)
}
console.log("\nunauthorized 断言全绿")

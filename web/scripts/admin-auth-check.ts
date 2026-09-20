// TDD 守护：isCurrentAdminSession 纯函数断言——刷新后恢复管理页的唯一依据是
// 「曾标记为管理会话的 token === 当前管理员名账号的会话 token」。
// 缺陷背景：渲染判据曾用「账号名 === 管理员名」冒充管理标志（App `inAdmin ||
// current === adminName`），撞名学生（B43-04 放行的普通教务会话）账号名恰等于
// 管理员名时恒命中 → 掉进 Admin 页五 Tab 全 403、403 不广播 401 → 循环死锁，
// 该学生永远无法使用学生功能。修复后管理态绑定会话 token：撞名学生的普通会话
// token 永远匹配不上被标记的管理 token，刷新不误进管理页。
// 用法：node --import jiti scripts/admin-auth-check.ts（退出码非 0 即断言失败）
import { isCurrentAdminSession } from "../src/lib/adminAuth"

let failed = 0
const assert = (name: string, got: boolean, want: boolean) => {
  const ok = got === want
  console.log(`${ok ? "  ✓" : "  ✗"} ${name}${ok ? "" : `（期望 ${want}，实际 ${got}）`}`)
  if (!ok) failed++
}

// 场景 A：管理员会话 token === 标记的管理 token → 刷新恢复管理页
assert("管理会话 token 一致 → 恢复", isCurrentAdminSession({ admin: "T1" }, "admin", "T1"), true)
// 场景 B（撞名学生刷新）：账号名=admin 但会话是普通 token（≠ 管理 token）→ 退出管理态
assert("撞名学生 token ≠ 管理 token → 学生端", isCurrentAdminSession({ admin: "T2" }, "admin", "T1"), false)
// 场景 C：管理会话已被吊销剔除（sessions 无 admin）→ 退出管理态
assert("管理会话已吊销 → 学生端", isCurrentAdminSession({ stu: "T9" }, "admin", "T1"), false)
// 场景 D：从未登录过管理（无标记 token，旧版本升级首刷）→ 退出管理态，重新登录恢复
assert("无标记管理 token → 学生端", isCurrentAdminSession({ admin: "T1" }, "admin", ""), false)
// 场景 E：普通学生账号 + 无标记 → 学生端
assert("普通学生无标记 → 学生端", isCurrentAdminSession({ stu: "T2" }, "admin", ""), false)
// 场景 F：管理员自定义名场景——管理 token 匹配自定义名账号会话
assert("自定义管理员名 token 一致 → 恢复", isCurrentAdminSession({ boss: "T5" }, "boss", "T5"), true)

if (failed > 0) {
  console.error(`\nadmin-auth 断言失败 ${failed} 项`)
  process.exit(1)
}
console.log("\nadmin-auth 断言全绿")
// 管理态判定：刷新后是否恢复管理页的唯一依据 = 「曾标记为管理会话的 token」
// 与「当前管理员名账号的会话 token」一致。账号名等于管理员名绝不能当判定——
// 撞名学生（后端 B43-04 放行的普通教务会话）账号名恰等于管理员名，拿名字判定
// 等于把普通会话误认成管理员会话，永久锁死 403 管理页。
// TDD 守护：web/scripts/admin-auth-check.ts（先红后绿）。
export function isCurrentAdminSession(
  sessions: Record<string, string>,
  adminName: string,
  adminToken: string
): boolean {
  return adminToken !== "" && sessions[adminName] === adminToken
}
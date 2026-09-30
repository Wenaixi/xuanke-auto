// 与后端 API JSON 对应的类型定义。字段与后端 upstream.Class 严格对齐：
// lessons_date/apply_date/plan_count/audited_count 前后端均零消费，已剔除
//（后端 json "-" 剔除后响应体不再下发，类型同步瘦身——绝不声明永不消费的幻影字段）。

export interface ClassItem {
  id: number
  publish_id: number
  course_name: string
  class_name: string
  teacher_name_list: string
  class_room_name: string
  selected_count: number
  max_count: number
  // 满员派生字段（后端解析端算一次，单一记忆点）：= max_count>0 && selected_count>=max_count。
  // 0=名额未公布绝不误判满员；前端 isFull/筛选/排序/徽章一律读本字段，不再各自手写判据。
  class_full: boolean
  can_select: boolean
  btn_type: number
  btn_text: string
  title: string
}

export interface Publish {
  publish_id: number
  publish_name: string
  begin_date: string
  in_date_range: boolean
  can_select: number
  has_selected: number
  group_count: number
  total_count: number
  classes: ClassItem[]
}

export interface ElectivesData {
  begin_times: number[]
  publishes: Publish[]
}

export interface CourseStatus {
  publish_id: number
  class_id: number
  course_name: string
  priority: number
  status: string // pending|in_range|submitted|success|failed
  result: string
  // 发布元数据（调度器随目标持久化/透传）：窗口关闭后 /state.courses 仍自带
  // 发布名与日期前缀，Dashboard 分组零依赖 /electives（关闭≠元数据丢失）。
  publish_name?: string
  begin_date?: string
  // 满员事实（后端快照 ClassFull 派生字段的直接投影）：看板判满员只读本字段，
  // 绝不匹配 result 中文文案——文案一改就静默失效，且平台原文含「已满员」的
  // 非满员失败会被误判。快照缺失（窗口关闭）时为 false，即不显满员徽章。
  class_full?: boolean
}

export interface SchedulerState {
  // 当前账号的"预计开放时间"（唯一事实源 = 平台 beginTimes 自动识别，不可配置）；
  // open_time_known=false = 未识别到或识别值已过期（识别不到/过期就是未知），open_time 为零值字符串。
  open_time: string
  open_time_known: boolean
  window_opened: boolean
  // 窗口已关闭信号（后端探测到空快照且从未开过窗即置真）。
  // 前端据此降频轮询——状态已定型，不必再 3 秒高频打接口。
  window_closed?: boolean
  token_valid: boolean
  courses: CourseStatus[]
}

export interface Target {
  publish_id: number
  class_id: number
  course_name: string
  priority?: number
}

export interface LogEntry {
  id: number
  class_id: number
  action: string
  result: string
  is_ok: boolean
  created_at: string
}

// 账号名（多账号下拉列表项）
export type Account = string

// 会话令牌映射：账号名 -> 服务端签发令牌（localStorage 持久化）
export type Sessions = Record<string, string>

// 激活码记录（管理面板展示）
export interface ActivationCode {
  code: string
  total_uses: number
  used_uses: number
  created_at: string
}

// ---- 管理员后台类型 ----

// 系统配置（GET /api/admin/config，Vision key 脱敏回显）。
// 开放时间已从配置项移除：只走平台 beginTimes 自动识别（识别态在 SchedulerState/AdminStats）。
export interface AdminConfig {
  activation_enabled: boolean
  vision_base_url: string
  vision_api_key_masked: string
  vision_model: string
  captcha_engine: string
  // 识别引擎兜底开关（默认 false）：关闭时 ddddocr 与 Vision 严格互不回退，
  // 开启后本机 ddddocr 不可用回退 Vision、Vision 无密钥回退本机 ddddocr。
  captcha_fallback: boolean
  captcha_concurrency: number
  // 监听地址（默认 127.0.0.1 仅本机；填内网 IP 开局域网、域名走穿透/公网、
  // 0.0.0.0 所有网卡）。改动会热重绑监听，无需重启。
  listen_host: string
  listen_port: string
  // 平台内置形态（APK）：端口输入置灰（App 内页面按 3091 连接，改端口会失联）。
  platform_embedded: boolean

  // 选课平台档案（上游站点的接口形态）。平台差异全在档案里：切换档案 = 换一套
  // 接口路径/键名/解码，流程（探测/提交/退避/黄金期）与前端一行不动。
  platform_id: string
  platform_name: string
  platform_note: string
  // 站点地址覆盖：空 = 用档案默认地址（换域名/镜像时填，免发版）。
  platform_base_url: string
  platform_default_base_url: string
  // 全部内置档案（下拉直接渲染，新增档案零前端改动）。
  platforms: PlatformInfo[]
}

// 一个内置选课平台档案的管理员可见元数据。
export interface PlatformInfo {
  id: string
  name: string
  note: string
  default_base_url: string
}

// 运行状态总览（GET /api/admin/stats）
export interface AdminStats {
  // open_time/open_time_set = 调度器平台 beginTimes 自动识别态（不可配置）；
  // open_time_set=false = 未识别/识别过期，open_time 为空串。
  open_time: string
  activation_on: boolean
  window_opened: boolean
  // 窗口已关闭信号（后端 /api/admin/stats 补发后生效；未下发时 undefined 走"待命中"，
  // 绝不假报关闭）。
  window_closed?: boolean
  account_count: number
  targets_count: number
  success_count: number
  log_count: number
  vision_model: string
  vision_base_url: string
  captcha_engine?: string
  // 实际生效引擎（兜底解析结果，与配置值分列）：
  // 兜底关闭后"配置 ddddocr 而本机无引擎"会让两者不一致，管理后台靠它展示运行真相。
  captcha_active_engine?: string
  captcha_concurrency?: number
  open_time_set?: boolean
  // 各账号教务 token 有效性（账号名 -> 是否有效），缺省视作全部有效
  token_valid?: Record<string, boolean>
  // 当前选课平台（管理员确认"跑的是哪一套接口"的唯一核对点）
  platform_id?: string
  platform_name?: string
}

// 账号管理条目（GET /api/admin/accounts）
export interface AdminAccount {
  account: string
  targets: Target[]
  success: number[]
}

// 日志总览条目（GET /api/admin/logs，全量含账号）
export interface AdminLog {
  id: number
  account: string
  class_id: number
  action: string
  result: string
  is_ok: boolean
  created_at: string
}

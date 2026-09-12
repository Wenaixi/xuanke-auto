// 与后端 API JSON 对应的类型定义

export interface ClassItem {
  id: number
  publish_id: number
  course_name: string
  class_name: string
  teacher_name_list: string
  class_room_name: string
  lessons_date: string
  selected_count: number
  audited_count: number
  max_count: number
  plan_count: number
  can_select: boolean
  btn_type: number
  btn_text: string
  title: string
  apply_date: string
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
}

export interface SchedulerState {
  open_time: string
  window_opened: boolean
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

// 系统配置（GET /api/admin/config，Vision key 脱敏回显）
export interface AdminConfig {
  activation_enabled: boolean
  vision_base_url: string
  vision_api_key_masked: string
  vision_model: string
  captcha_engine: string
  captcha_concurrency: number
  open_time: string
}

// 运行状态总览（GET /api/admin/stats）
export interface AdminStats {
  open_time: string
  activation_on: boolean
  window_opened: boolean
  account_count: number
  targets_count: number
  success_count: number
  log_count: number
  vision_model: string
  vision_base_url: string
  captcha_engine?: string
  captcha_concurrency?: number
  open_time_set?: boolean
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

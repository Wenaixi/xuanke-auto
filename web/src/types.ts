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

export interface ClassDetail {
  id: number
  course_name: string
  class_name: string
  teacher_name: string
  classroom_name: string
  lessons_date: string
  school_year_term: string
  course_type_name: string
  method_name: string
  evaluate_type_name: string
  audited_count: number
  plan_count: number
  class_status_str: string
  // 真实接口 key 为 shareUrl（HAR 实测：https://www.zhidao.fj.cn/electives/detail/elecClass/<hex>）
  shareUrl: string
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

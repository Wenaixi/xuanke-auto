-- 至道选课自动化 SQLite 表结构

-- 账号与 token（本地自用，明文存储；API 永不回传密码）
CREATE TABLE IF NOT EXISTS account (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL,
  password TEXT NOT NULL,
  id_token TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 目标课程（每个发布 1 门，先删后插保证唯一）
CREATE TABLE IF NOT EXISTS targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  publish_id INTEGER NOT NULL,
  class_id INTEGER NOT NULL,
  course_name TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 报名日志
CREATE TABLE IF NOT EXISTS task_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  class_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  result TEXT,
  is_ok INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 任务状态快照（重启恢复用：最近一次扫描的发布窗口与已成功课程）
CREATE TABLE IF NOT EXISTS task_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  school_year INTEGER NOT NULL DEFAULT 2026,
  school_term INTEGER NOT NULL DEFAULT 1,
  open_time TEXT NOT NULL,
  window_opened INTEGER NOT NULL DEFAULT 0,
  success_class_ids TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

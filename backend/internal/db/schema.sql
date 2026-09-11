-- 至道选课自动化 SQLite 表结构

-- 账号与 token（本地自用，明文存储；API 永不回传密码）
CREATE TABLE IF NOT EXISTS account (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL,
  password TEXT NOT NULL,
  id_token TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 多账号名表：登录成功即 upsert 账号名（密码不入库），account 唯一
CREATE TABLE IF NOT EXISTS accounts (
  account TEXT PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 目标课程（按账号隔离：account='' 为默认/旧单账号数据；先删后插保证唯一）
CREATE TABLE IF NOT EXISTS targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL DEFAULT '',
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

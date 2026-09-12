-- 至道选课自动化 SQLite 表结构（v2：多账号物理隔离 + 凭据加密）

-- 账号凭据（密码 AES-GCM 加密后入库，明文只在内存中出现）
CREATE TABLE IF NOT EXISTS credentials (
  account TEXT PRIMARY KEY,
  password_enc TEXT NOT NULL,
  id_token TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 已知账号名（登录成功即 upsert，账号唯一）
CREATE TABLE IF NOT EXISTS accounts (
  account TEXT PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 目标课程（按账号隔离 + 备选优先级，priority 越小越先提交）
CREATE TABLE IF NOT EXISTS targets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL,
  publish_id INTEGER NOT NULL,
  class_id INTEGER NOT NULL,
  course_name TEXT NOT NULL,
  priority INTEGER NOT NULL DEFAULT 0,
  allow_swap INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 报名日志（按账号隔离）
CREATE TABLE IF NOT EXISTS task_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account TEXT NOT NULL DEFAULT '',
  class_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  result TEXT,
  is_ok INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 激活码（可设总可用次数）
CREATE TABLE IF NOT EXISTS activation_codes (
  code TEXT PRIMARY KEY,
  total_uses INTEGER NOT NULL,
  used_uses INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 已激活账号（激活一次永久有效）
CREATE TABLE IF NOT EXISTS activations (
  account TEXT PRIMARY KEY,
  activated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 已成功课程（重启后禁止重复报名，按账号独立）
CREATE TABLE IF NOT EXISTS success (
  account TEXT NOT NULL,
  class_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (account, class_id)
);

-- 系统配置（管理员热重载持久化，k/v 存储）
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
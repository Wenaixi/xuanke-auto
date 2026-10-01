package store

import (
	"database/sql"
	"sort"

	"xuanke-auto/backend/internal/accounts"
	"xuanke-auto/backend/internal/scheduler"
)

// Store SQLite 持久化：凭据（加密）/目标/日志/成功记录。
type Store struct {
	db *sql.DB
}

// New 创建 store。
func New(db *sql.DB) *Store {
	return &Store{db: db}
}
// ---- 深查询管线：rows 生命周期与遍历归一 ----
// 全仓 10 个 Load*/List* 查询此前各自重写「Query → defer Close → Next 循环 →
// Scan → rows.Err」样板（LoadSuccess 与 LoadRefused 归一化后逐字相同）。
// 收进三个 helper 后，调用点只剩 SQL + scan 闭包；rows 的「先关后查 Err」陷阱、
// Scan 失败路径全部集中一处（查询深模块化）。
func queryRows(db *sql.DB, query string, args []any, each func(*sql.Rows) error) error {
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// querySlice 单类型多行查询：scan 闭包把一行转成 T。
func querySlice[T any](db *sql.DB, query string, args []any, scan func(*sql.Rows) (T, error)) ([]T, error) {
	var out []T
	err := queryRows(db, query, args, func(rows *sql.Rows) error {
		v, err := scan(rows)
		if err != nil {
			return err
		}
		out = append(out, v)
		return nil
	})
	return out, err
}

// queryMapKV 键值对查询（settings 表）。
func queryMapKV(db *sql.DB, query string, args []any) (map[string]string, error) {
	out := map[string]string{}
	err := queryRows(db, query, args, func(rows *sql.Rows) error {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		out[k] = v
		return nil
	})
	return out, err
}

// queryAcctClass 账号→课程 ID 聚合（success / refused 两表共用）。
// 表名来自包内调用点白名单（"success" / "refused"），非外部输入。
func queryAcctClass(db *sql.DB, table string) (map[string][]int, error) {
	out := map[string][]int{}
	err := queryRows(db, "SELECT account, class_id FROM "+table, nil, func(rows *sql.Rows) error {
		var a string
		var cid int
		if err := rows.Scan(&a, &cid); err != nil {
			return err
		}
		out[a] = append(out[a], cid)
		return nil
	})
	return out, err
}


// Credential 账号凭据（密码列为密文，明文只在内存）。
type Credential struct {
	Account     string
	PasswordEnc string
	IDToken     string
	PlatformID  string
}

// SaveCredential upsert 账号凭据（加密密码 + 当前 token + token 所属平台档案）。
// platformID 与 token 一次写入：切换平台时正是用「空 token + 新平台 ID」清跨平台会话。
func (s *Store) SaveCredential(acct, pwdEnc, idToken, platformID string) error {
	_, err := s.db.Exec(
		"INSERT INTO credentials (account, password_enc, id_token, platform_id) VALUES (?, ?, ?, ?) "+
			"ON CONFLICT(account) DO UPDATE SET password_enc=excluded.password_enc, id_token=excluded.id_token, "+
			"platform_id=excluded.platform_id, updated_at=datetime('now')",
		acct, pwdEnc, idToken, platformID)
	return err
}

// LoadCredentials 读取全部账号凭据（按账号排序）。
func (s *Store) LoadCredentials() ([]accounts.Credential, error) {
	return querySlice(s.db, "SELECT account, password_enc, id_token, platform_id FROM credentials ORDER BY account", nil, func(rows *sql.Rows) (accounts.Credential, error) {
		var c accounts.Credential
		if err := rows.Scan(&c.Account, &c.PasswordEnc, &c.IDToken, &c.PlatformID); err != nil {
			return accounts.Credential{}, err
		}
		return c, nil
	})
}

// UpdateIDToken 仅刷新 token（账密不变时调用）。
func (s *Store) UpdateIDToken(acct, idToken string) error {
	_, err := s.db.Exec("UPDATE credentials SET id_token = ?, updated_at = datetime('now') WHERE account = ?", idToken, acct)
	return err
}

// SaveAccountName 记录账号名（登录成功调用；账号名即主键，幂等）。
func (s *Store) SaveAccountName(acct string) error {
	if acct == "" {
		return nil
	}
	_, err := s.db.Exec("INSERT OR IGNORE INTO accounts (account) VALUES (?)", acct)
	return err
}

// ListAccounts 返回所有账号名（按账号排序）。
func (s *Store) ListAccounts() ([]string, error) {
	return querySlice(s.db, "SELECT account FROM accounts ORDER BY account", nil, func(rows *sql.Rows) (string, error) {
		var a string
		if err := rows.Scan(&a); err != nil {
			return "", err
		}
		return a, nil
	})
}

// SetTargetsForAccount 按账号保存目标课程（先删后插；按 priority 排序保证备选顺序）。
func (s *Store) SetTargetsForAccount(acct string, targets []scheduler.Target) error {
	sort.SliceStable(targets, func(i, j int) bool { return targets[i].Priority < targets[j].Priority })
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM targets WHERE account = ?", acct); err != nil {
		return err
	}
	for _, t := range targets {
		// publish_name/begin_date：发布元数据随目标落库——窗口关闭后 /state 重建
		// 课程状态仍自带日期/发布名（scheduler.SetTargetsForAccount 已在保存前补全）
		if _, err := tx.Exec(
			"INSERT INTO targets (account, publish_id, class_id, course_name, priority, publish_name, begin_date) VALUES (?, ?, ?, ?, ?, ?, ?)",
			acct, t.PublishID, t.ClassID, t.CourseName, t.Priority, t.PublishName, t.BeginDate); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadTargetsForAccount 读取指定账号的目标课程（按 priority 排序）。
func (s *Store) LoadTargetsForAccount(acct string) ([]scheduler.Target, error) {
	return querySlice(s.db, "SELECT publish_id, class_id, course_name, priority, publish_name, begin_date FROM targets WHERE account = ? ORDER BY priority", []any{acct}, func(rows *sql.Rows) (scheduler.Target, error) {
		var t scheduler.Target
		if err := rows.Scan(&t.PublishID, &t.ClassID, &t.CourseName, &t.Priority, &t.PublishName, &t.BeginDate); err != nil {
			return scheduler.Target{}, err
		}
		return t, nil
	})
}

// SaveSuccess 记录某账号某课程已报名成功（幂等）。
func (s *Store) SaveSuccess(acct string, classID int) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO success (account, class_id) VALUES (?, ?)", acct, classID)
	return err
}

// DeleteSuccess 手动退选后删除 success 行——否则重启后
// RestoreDone 会把用户已退选的课恢复成"已报名成功"，退选意图丢失。
func (s *Store) DeleteSuccess(acct string, classID int) error {
	_, err := s.db.Exec("DELETE FROM success WHERE account = ? AND class_id = ?", acct, classID)
	return err
}

// LoadSuccess 读取全部成功记录（map[账号][]classID）。
func (s *Store) LoadSuccess() (map[string][]int, error) {
	return queryAcctClass(s.db, "success")
}

// SaveRefused 记录某账号某课程已手动退选（幂等，持久化 refused）。
func (s *Store) SaveRefused(acct string, classID int) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO refused (account, class_id) VALUES (?, ?)", acct, classID)
	return err
}

// DeleteRefused 清空某账号的全部已退选记录（重设目标 = 主动重新接管）。
func (s *Store) DeleteRefused(acct string) error {
	_, err := s.db.Exec("DELETE FROM refused WHERE account = ?", acct)
	return err
}

// DeleteRefusedClass 删除某账号某课程的单条已退选记录——手动重报成功后同步清库行，
// 否则库内残留会让重启恢复序 LoadRefused+RestoreRefused 把已报名成功的课程恢复成
// "已手动退选"，自动引擎永久跳过该课（与内存侧 MarkDone 解除 refused 必须对称）。
func (s *Store) DeleteRefusedClass(acct string, classID int) error {
	_, err := s.db.Exec("DELETE FROM refused WHERE account = ? AND class_id = ?", acct, classID)
	return err
}

// LoadRefused 读取全部已退选记录（map[账号][]classID，重启恢复用）。
func (s *Store) LoadRefused() (map[string][]int, error) {
	return queryAcctClass(s.db, "refused")
}

// AppendLog 追加报名日志（account 标识来源账号，日志按账号隔离）。
func (s *Store) AppendLog(acct string, classID int, action, result string, isOK bool) error {
	ok := 0
	if isOK {
		ok = 1
	}
	_, err := s.db.Exec("INSERT INTO task_log (account, class_id, action, result, is_ok) VALUES (?, ?, ?, ?, ?)",
		acct, classID, action, result, ok)
	return err
}

// LogEntry 日志条目。
type LogEntry struct {
	ID        int64  `json:"id"`
	Account   string `json:"account"`
	ClassID   int    `json:"class_id"`
	Action    string `json:"action"`
	Result    string `json:"result"`
	IsOK      bool   `json:"is_ok"`
	CreatedAt string `json:"created_at"`
}

// LoadLogs 读取指定账号最近 limit 条日志。
func (s *Store) LoadLogs(acct string, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	// 保留最近日志窗口的查询前置——task_log 无清理无限增长，
	// 百万行后全表扫描退化为秒级。自增主键 max(id) 走 O(1) 索引，窗口恒为
	// 最近 2 万条（日志仅展示用途，审计无合规要求），零删除零 DDL 契约不变。
	return querySlice(s.db,
		"SELECT id, account, class_id, action, result, is_ok, created_at FROM task_log WHERE account = ? AND id > (SELECT max(id) - 20000 FROM task_log) ORDER BY id DESC LIMIT ?",
		[]any{acct, limit}, func(rows *sql.Rows) (LogEntry, error) {
			var e LogEntry
			var ok int
			if err := rows.Scan(&e.ID, &e.Account, &e.ClassID, &e.Action, &e.Result, &ok, &e.CreatedAt); err != nil {
				return LogEntry{}, err
			}
			e.IsOK = ok == 1
			return e, nil
		})
}

// ActivationCode 激活码记录。
type ActivationCode struct {
	Code      string `json:"code"`
	TotalUses int    `json:"total_uses"`
	UsedUses  int    `json:"used_uses"`
	CreatedAt string `json:"created_at"`
}

// CreateActivationCode 新建激活码。
func (s *Store) CreateActivationCode(code string, totalUses int) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO activation_codes (code, total_uses) VALUES (?, ?)", code, totalUses)
	return err
}

// CreateActivationCodes 批量新建激活码：全部码在同一事务内原子落库——
// 任一条 INSERT 失败整体回滚，绝不产生"前 N-1 个已入库、响应报错"的隐身码滞留。
// SQLite 单写者串行化，事务无并发锁成本；失败时调用方得到一致性错误并重试整批。
func (s *Store) CreateActivationCodes(codes []string, totalUses int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, code := range codes {
		if _, err := tx.Exec("INSERT OR IGNORE INTO activation_codes (code, total_uses) VALUES (?, ?)", code, totalUses); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IsActivated 查询账号是否已激活。
func (s *Store) IsActivated(acct string) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT count(*) FROM activations WHERE account = ?", acct).Scan(&n)
	return n > 0, err
}

// ConsumeActivationCode 激活账号：原子扣减激活码次数 + 记录激活。返回是否成功。
// 激活码不存在或次数用尽返回 (false, nil)。
// 原子性：扣减与"次数未用尽"判定压进单条 UPDATE（used_uses < total_uses 条件），
// 并发消费同一激活码时由数据库原子保证绝不超卖——不存在"读到旧次数再改"的读改写竞态。
func (s *Store) ConsumeActivationCode(code, acct string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec("UPDATE activation_codes SET used_uses = used_uses + 1 WHERE code = ? AND used_uses < total_uses", code)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil // 激活码不存在或次数已用尽
	}
	// 已激活账号绝不重复扣次。INSERT OR IGNORE 对已激活账号静默跳过，
	// 但这里仍返回 (true, nil)——次数被扣、账号无变化、前端显示"激活成功"实未生效。
	// 先查询是否已激活：已激活直接返回 (false, nil) 且不扣次（事务回滚），
	// 让前端提示"该账号已激活"，杜绝双扣。
	var nAct int
	if err := tx.QueryRow("SELECT count(*) FROM activations WHERE account = ?", acct).Scan(&nAct); err != nil {
		return false, err
	}
	if nAct > 0 {
		return false, nil
	}
	if _, err := tx.Exec("INSERT INTO activations (account) VALUES (?)", acct); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// ListActivationCodes 列出全部激活码。
func (s *Store) ListActivationCodes() ([]ActivationCode, error) {
	return querySlice(s.db, "SELECT code, total_uses, used_uses, created_at FROM activation_codes ORDER BY created_at DESC", nil, func(rows *sql.Rows) (ActivationCode, error) {
		var a ActivationCode
		if err := rows.Scan(&a.Code, &a.TotalUses, &a.UsedUses, &a.CreatedAt); err != nil {
			return ActivationCode{}, err
		}
		return a, nil
	})
}

// DeleteActivationCode 删除激活码。
func (s *Store) DeleteActivationCode(code string) error {
	_, err := s.db.Exec("DELETE FROM activation_codes WHERE code = ?", code)
	return err
}

// SaveSettings 全量替换系统配置（管理员热重载落库；k/v 字符串）。
func (s *Store) SaveSettings(kv map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM settings"); err != nil {
		return err
	}
	for k, v := range kv {
		if _, err := tx.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadSettings 读取全部系统配置。
func (s *Store) LoadSettings() (map[string]string, error) {
	return queryMapKV(s.db, "SELECT key, value FROM settings", nil)
}

// DeleteAccount 管理员删除账号：清其凭据/账号名/目标/成功记录/已退选记录/激活状态。
// 报名日志保留（审计用途），仅重新登录即可重建凭据与客户端。
func (s *Store) DeleteAccount(acct string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 凭据与已知账号名（学生客户端按需重新登录）
	if _, err := tx.Exec("DELETE FROM credentials WHERE account = ?", acct); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM accounts WHERE account = ?", acct); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM targets WHERE account = ?", acct); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM success WHERE account = ?", acct); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM refused WHERE account = ?", acct); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM activations WHERE account = ?", acct); err != nil {
		return err
	}
	return tx.Commit()
}

// LoadAllLogs 读取全量日志（管理员日志总览用，不按账号过滤）。
func (s *Store) LoadAllLogs(limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	return querySlice(s.db,
		"SELECT id, account, class_id, action, result, is_ok, created_at FROM task_log WHERE id > (SELECT max(id) - 20000 FROM task_log) ORDER BY id DESC LIMIT ?",
		[]any{limit}, func(rows *sql.Rows) (LogEntry, error) {
			var e LogEntry
			var ok int
			if err := rows.Scan(&e.ID, &e.Account, &e.ClassID, &e.Action, &e.Result, &ok, &e.CreatedAt); err != nil {
				return LogEntry{}, err
			}
			e.IsOK = ok == 1
			return e, nil
		})
}

// CountAllLogs 日志总数（COUNT，不加载行）。
// /admin/stats 只需 log_count 计数——此前全量 LoadAllLogs(1000) 逐行反序列化只为
// 数个数，5s 轮询下每次白读 1000 行。COUNT(*) 走 SQLite 表级 O(1)（主键索引无关），
// 零行加载零反序列化。与 LoadAllLogs 同口径：同一时刻两查询对新日志/超窗日志判定一致。
func (s *Store) CountAllLogs() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM task_log").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// AdminAccount 账号管理条目：账号名 + 目标 + 已成功课程。
type AdminAccount struct {
	Account string             `json:"account"`
	Targets []scheduler.Target `json:"targets"`
	Success []int              `json:"success"`
}

// ListAdminAccounts 列出全部账号及目标/成功记录（管理员账号管理用）。
func (s *Store) ListAdminAccounts() ([]AdminAccount, error) {
	names, err := s.ListAccounts()
	if err != nil {
		return nil, err
	}
	out := make([]AdminAccount, 0, len(names))
	for _, n := range names {
		ts, err := s.LoadTargetsForAccount(n)
		if err != nil {
			return nil, err
		}
		ids, err := querySlice(s.db, "SELECT class_id FROM success WHERE account = ? ORDER BY class_id", []any{n}, func(rows *sql.Rows) (int, error) {
			var id int
			if err := rows.Scan(&id); err != nil {
				return 0, err
			}
			return id, nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, AdminAccount{Account: n, Targets: ts, Success: ids})
	}
	return out, nil
}

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

// Credential 账号凭据（密码列为密文，明文只在内存）。
type Credential struct {
	Account     string
	PasswordEnc string
	IDToken     string
}

// SaveCredential upsert 账号凭据（加密密码 + 当前 token）。
func (s *Store) SaveCredential(acct, pwdEnc, idToken string) error {
	_, err := s.db.Exec(
		"INSERT INTO credentials (account, password_enc, id_token) VALUES (?, ?, ?) "+
			"ON CONFLICT(account) DO UPDATE SET password_enc=excluded.password_enc, id_token=excluded.id_token, updated_at=datetime('now')",
		acct, pwdEnc, idToken)
	return err
}

// LoadCredentials 读取全部账号凭据（按账号排序）。
func (s *Store) LoadCredentials() ([]accounts.Credential, error) {
	rows, err := s.db.Query("SELECT account, password_enc, id_token FROM credentials ORDER BY account")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []accounts.Credential
	for rows.Next() {
		var c accounts.Credential
		if err := rows.Scan(&c.Account, &c.PasswordEnc, &c.IDToken); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query("SELECT account FROM accounts ORDER BY account")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query("SELECT publish_id, class_id, course_name, priority, publish_name, begin_date FROM targets WHERE account = ? ORDER BY priority", acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduler.Target
	for rows.Next() {
		var t scheduler.Target
		if err := rows.Scan(&t.PublishID, &t.ClassID, &t.CourseName, &t.Priority, &t.PublishName, &t.BeginDate); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query("SELECT account, class_id FROM success")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]int{}
	for rows.Next() {
		var a string
		var cid int
		if err := rows.Scan(&a, &cid); err != nil {
			return nil, err
		}
		out[a] = append(out[a], cid)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query("SELECT account, class_id FROM refused")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]int{}
	for rows.Next() {
		var a string
		var cid int
		if err := rows.Scan(&a, &cid); err != nil {
			return nil, err
		}
		out[a] = append(out[a], cid)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query(
		"SELECT id, account, class_id, action, result, is_ok, created_at FROM task_log WHERE account = ? AND id > (SELECT max(id) - 20000 FROM task_log) ORDER BY id DESC LIMIT ?",
		acct, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var ok int
		if err := rows.Scan(&e.ID, &e.Account, &e.ClassID, &e.Action, &e.Result, &ok, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IsOK = ok == 1
		out = append(out, e)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query("SELECT code, total_uses, used_uses, created_at FROM activation_codes ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActivationCode
	for rows.Next() {
		var a ActivationCode
		if err := rows.Scan(&a.Code, &a.TotalUses, &a.UsedUses, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
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
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
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
	rows, err := s.db.Query(
		"SELECT id, account, class_id, action, result, is_ok, created_at FROM task_log WHERE id > (SELECT max(id) - 20000 FROM task_log) ORDER BY id DESC LIMIT ?",
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var ok int
		if err := rows.Scan(&e.ID, &e.Account, &e.ClassID, &e.Action, &e.Result, &ok, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IsOK = ok == 1
		out = append(out, e)
	}
	return out, rows.Err()
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
		rows, err := s.db.Query("SELECT class_id FROM success WHERE account = ? ORDER BY class_id", n)
		if err != nil {
			return nil, err
		}
		var ids []int
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		out = append(out, AdminAccount{Account: n, Targets: ts, Success: ids})
	}
	return out, nil
}

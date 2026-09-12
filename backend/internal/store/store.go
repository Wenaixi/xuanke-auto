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
		if _, err := tx.Exec(
			"INSERT INTO targets (account, publish_id, class_id, course_name, priority) VALUES (?, ?, ?, ?, ?)",
			acct, t.PublishID, t.ClassID, t.CourseName, t.Priority); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadTargetsForAccount 读取指定账号的目标课程（按 priority 排序）。
func (s *Store) LoadTargetsForAccount(acct string) ([]scheduler.Target, error) {
	rows, err := s.db.Query("SELECT publish_id, class_id, course_name, priority FROM targets WHERE account = ? ORDER BY priority", acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduler.Target
	for rows.Next() {
		var t scheduler.Target
		if err := rows.Scan(&t.PublishID, &t.ClassID, &t.CourseName, &t.Priority); err != nil {
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
	rows, err := s.db.Query(
		"SELECT id, account, class_id, action, result, is_ok, created_at FROM task_log WHERE account = ? ORDER BY id DESC LIMIT ?",
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

// IsActivated 查询账号是否已激活。
func (s *Store) IsActivated(acct string) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT count(*) FROM activations WHERE account = ?", acct).Scan(&n)
	return n > 0, err
}

// ConsumeActivationCode 激活账号：事务内扣减激活码次数 + 记录激活。返回是否成功。
// 激活码不存在或次数用尽返回 (false, nil)。
func (s *Store) ConsumeActivationCode(code, acct string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var total, used int
	err = tx.QueryRow("SELECT total_uses, used_uses FROM activation_codes WHERE code = ?", code).Scan(&total, &used)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if used >= total {
		return false, nil
	}
	if _, err := tx.Exec("UPDATE activation_codes SET used_uses = used_uses + 1 WHERE code = ?", code); err != nil {
		return false, err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO activations (account) VALUES (?)", acct); err != nil {
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

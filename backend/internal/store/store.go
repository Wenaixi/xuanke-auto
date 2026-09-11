package store

import (
	"database/sql"

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
func (s *Store) LoadCredentials() ([]Credential, error) {
	rows, err := s.db.Query("SELECT account, password_enc, id_token FROM credentials ORDER BY account")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Credential
	for rows.Next() {
		var c Credential
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

// SetTargetsForAccount 按账号保存目标课程（先删后插保证唯一；账号必填）。
func (s *Store) SetTargetsForAccount(acct string, targets []scheduler.Target) error {
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
			"INSERT INTO targets (account, publish_id, class_id, course_name) VALUES (?, ?, ?, ?)",
			acct, t.PublishID, t.ClassID, t.CourseName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadTargetsForAccount 读取指定账号的目标课程。
func (s *Store) LoadTargetsForAccount(acct string) ([]scheduler.Target, error) {
	rows, err := s.db.Query("SELECT publish_id, class_id, course_name FROM targets WHERE account = ? ORDER BY id", acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduler.Target
	for rows.Next() {
		var t scheduler.Target
		if err := rows.Scan(&t.PublishID, &t.ClassID, &t.CourseName); err != nil {
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

// AppendLog 追加报名日志。
func (s *Store) AppendLog(classID int, action, result string, isOK bool) error {
	ok := 0
	if isOK {
		ok = 1
	}
	_, err := s.db.Exec("INSERT INTO task_log (class_id, action, result, is_ok) VALUES (?, ?, ?, ?)",
		classID, action, result, ok)
	return err
}

// LogEntry 日志条目。
type LogEntry struct {
	ID        int64  `json:"id"`
	ClassID   int    `json:"class_id"`
	Action    string `json:"action"`
	Result    string `json:"result"`
	IsOK      bool   `json:"is_ok"`
	CreatedAt string `json:"created_at"`
}

// LoadLogs 读取最近 limit 条日志。
func (s *Store) LoadLogs(limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(
		"SELECT id, class_id, action, result, is_ok, created_at FROM task_log ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var ok int
		if err := rows.Scan(&e.ID, &e.ClassID, &e.Action, &e.Result, &ok, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IsOK = ok == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

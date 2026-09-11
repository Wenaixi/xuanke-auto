package store

import (
	"database/sql"
	"fmt"
	"strings"

	"xuanke-auto/backend/internal/scheduler"
)

// Store SQLite 持久化：账密/token/目标/日志。
type Store struct {
	db *sql.DB
}

// New 创建 store。
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// SaveAccount 保存账密与 token（单行，先删后插）。
func (s *Store) SaveAccount(acct, pwd, token string) error {
	if _, err := s.db.Exec("DELETE FROM account"); err != nil {
		return err
	}
	_, err := s.db.Exec("INSERT INTO account (account, password, id_token) VALUES (?, ?, ?)", acct, pwd, token)
	return err
}

// SaveTokenOnly 保存纯 token 会话（无账密，环境变量注入路径）。
// 先清空旧记录再插入，保证唯一一行。
func (s *Store) SaveTokenOnly(token string) error {
	if token == "" {
		return nil
	}
	if _, err := s.db.Exec("DELETE FROM account"); err != nil {
		return err
	}
	_, err := s.db.Exec("INSERT INTO account (account, password, id_token) VALUES ('', '', ?)", token)
	return err
}

// UpdateToken 更新已保存记录的 token（环境变量注入的会话持久化）。
// 无已有记录时无操作（不创建空账密记录）。
func (s *Store) UpdateToken(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec("UPDATE account SET id_token = ? WHERE id = (SELECT id FROM account ORDER BY id DESC LIMIT 1)", token)
	return err
}

// LoadAccount 读取保存的账密与 token。
func (s *Store) LoadAccount() (acct, pwd, token string, err error) {
	err = s.db.QueryRow("SELECT account, password, COALESCE(id_token,'') FROM account ORDER BY id DESC LIMIT 1").
		Scan(&acct, &pwd, &token)
	if err == sql.ErrNoRows {
		return "", "", "", nil
	}
	return
}

// SaveAccountName 记录账号名（登录成功调用；账号名即主键，幂等，密码不入库）。
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

// SetTargets 替换目标课程（默认账号，先删后插保证唯一）。
func (s *Store) SetTargets(targets []scheduler.Target) error {
	return s.SetTargetsForAccount("", targets)
}

// LoadTargets 读取目标课程（默认账号）。
func (s *Store) LoadTargets() ([]scheduler.Target, error) {
	return s.LoadTargetsForAccount("")
}

// SetTargetsForAccount 按账号保存目标课程（先删后插保证唯一；account='' 为默认账号）。
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

// SaveState 保存任务状态快照（窗口开启时间 + 已成功课程）。
func (s *Store) SaveState(openTime string, windowOpened bool, successClassIDs []int) error {
	ids := make([]string, len(successClassIDs))
	for i, id := range successClassIDs {
		ids[i] = fmt.Sprintf("%d", id)
	}
	opened := 0
	if windowOpened {
		opened = 1
	}
	_, err := s.db.Exec("INSERT INTO task_state (id, open_time, window_opened, success_class_ids) VALUES (1, ?, ?, ?) "+
		"ON CONFLICT(id) DO UPDATE SET open_time=excluded.open_time, window_opened=excluded.window_opened, "+
		"success_class_ids=excluded.success_class_ids, updated_at=datetime('now')",
		openTime, opened, strings.Join(ids, ","))
	return err
}

// LoadState 读取任务状态快照。
func (s *Store) LoadState() (openTime string, windowOpened bool, successClassIDs []int, err error) {
	var opened int
	var ids string
	err = s.db.QueryRow("SELECT open_time, window_opened, success_class_ids FROM task_state WHERE id=1").
		Scan(&openTime, &opened, &ids)
	if err == sql.ErrNoRows {
		return "", false, nil, nil
	}
	if err != nil {
		return "", false, nil, err
	}
	windowOpened = opened == 1
	for _, p := range strings.Split(ids, ",") {
		if p == "" {
			continue
		}
		var id int
		if _, err2 := fmt.Sscanf(p, "%d", &id); err2 == nil {
			successClassIDs = append(successClassIDs, id)
		}
	}
	return openTime, windowOpened, successClassIDs, nil
}
package db

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open 打开（必要时创建）SQLite 数据库并确保表结构存在。
// modernc.org/sqlite 为纯 Go 实现，免 CGO，可交叉编译单二进制。
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1) // SQLite 单写者，串行化连接避免锁冲突
	if _, err := d.Exec(schemaSQL); err != nil {
		d.Close()
		return nil, err
	}
	if err := ensureTargetsAccountColumn(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// ensureTargetsAccountColumn 老库 targets 表无 account 列时补列，保证按账号隔离目标可用。
func ensureTargetsAccountColumn(d *sql.DB) error {
	rows, err := d.Query("PRAGMA table_info(targets)")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "account" {
			return nil // 已有列，无需迁移
		}
	}
	_, err = d.Exec("ALTER TABLE targets ADD COLUMN account TEXT NOT NULL DEFAULT ''")
	return err
}

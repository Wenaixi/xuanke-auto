package db

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open 打开（必要时创建）SQLite 数据库并确保表结构存在。
// 检测到旧版数据形状（account 表 / 空账号目标）时直接报错拒绝启动——政策：不兼容旧数据。
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
	if err := refuseLegacy(d); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// refuseLegacy 兼容性检查：检测到旧版数据形状直接拒绝启动（政策：不兼容旧数据）。
func refuseLegacy(d *sql.DB) error {
	var n int
	if err := d.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='account'").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errors.New("检测到旧版数据库（account 表），本版本不兼容旧数据。请删除 " + "data/xuanke.db" + " 后重新启动")
	}
	var empty int
	if err := d.QueryRow("SELECT count(*) FROM targets WHERE account = ''").Scan(&empty); err != nil {
		return err
	}
	if empty > 0 {
		return errors.New("检测到旧版空账号目标数据，本版本不兼容旧数据。请删除 " + "data/xuanke.db" + " 后重新启动")
	}
	return nil
}
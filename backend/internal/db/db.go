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
	return d, nil
}

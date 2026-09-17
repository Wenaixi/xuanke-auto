package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenAndSchema(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var n int
	d.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&n)
	if n < 4 {
		t.Fatalf("expected >=4 tables, got %d", n)
	}
	// 新表存在性检查
	d.QueryRow("SELECT count(*) FROM activation_codes").Scan(&n)
	d.QueryRow("SELECT count(*) FROM activations").Scan(&n)
	// 新列存在性检查
	d.QueryRow("SELECT priority FROM targets LIMIT 0").Scan(&n)
	d.QueryRow("SELECT account FROM task_log LIMIT 0").Scan(&n)
	d.QueryRow("SELECT publish_name FROM targets LIMIT 0").Scan(&n)
	d.QueryRow("SELECT begin_date FROM targets LIMIT 0").Scan(&n)
}

// TestRefuseOldSchemaMissingPublishMeta 缺 publish_name/begin_date 列的旧库必须被拒绝启动——
// 发布元数据随目标持久化是窗口关闭后日期/发布名仍可显示的数据源，缺列即旧数据形状。
func TestRefuseOldSchemaMissingPublishMeta(t *testing.T) {
	for _, col := range []string{"publish_name", "begin_date"} {
		path := filepath.Join(t.TempDir(), "old.db")
		d, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Exec("CREATE TABLE targets (id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT, publish_id INTEGER, class_id INTEGER, course_name TEXT, priority INTEGER, allow_swap INTEGER, created_at TEXT)"); err != nil {
			t.Fatal(err)
		}
		d.Close()
		if _, err := Open(path); err == nil {
			t.Fatalf("缺 %s 列的旧库应被拒绝启动", col)
		}
	}
}

// TestRefuseOldSchemaMissingColumns v2 库缺 priority/account 列必须被拒绝启动。
func TestRefuseOldSchemaMissingColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("CREATE TABLE targets (id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT, publish_id INTEGER, class_id INTEGER, course_name TEXT)"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("缺 priority 列的旧库应被拒绝启动")
	}
}

// TestRefuseLegacyDB 旧版数据形状（account 表）必须被拒绝启动——政策：不兼容旧数据。
func TestRefuseLegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("CREATE TABLE account (id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT, password TEXT, id_token TEXT)"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("旧库应被拒绝启动")
	}
}

// TestRefuseEmptyAccountTargets 存在空账号目标的旧库必须被拒绝。
func TestRefuseEmptyAccountTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy2.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("CREATE TABLE targets (id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT, publish_id INTEGER, class_id INTEGER, course_name TEXT, created_at TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO targets (account, publish_id, class_id, course_name) VALUES ('', 1, 61115, '健美操')"); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("含空账号目标的旧库应被拒绝启动")
	}
}
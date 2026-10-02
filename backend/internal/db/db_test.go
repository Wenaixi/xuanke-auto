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

// TestMigrateAddsPublishMetaColumns 缺 publish_name/begin_date 列的旧库必须被自动增量迁移
// （ALTER TABLE ADD COLUMN 纯加列、不破坏既有数据），而不是拒绝启动——发布元数据随目标
// 持久化是窗口关闭后日期/发布名仍可显示的数据源，且用户真实库中积累的账号/目标数据
// 绝不能因升级丢失（"数据都要保存好啊"契约）。
func TestMigrateAddsPublishMetaColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("CREATE TABLE targets (id INTEGER PRIMARY KEY AUTOINCREMENT, account TEXT, publish_id INTEGER, class_id INTEGER, course_name TEXT, priority INTEGER, allow_swap INTEGER, created_at TEXT)"); err != nil {
		t.Fatal(err)
	}
	// 旧库先放入一条真实目标行，验证迁移后数据原样保留
	if _, err := d.Exec("INSERT INTO targets (account, publish_id, class_id, course_name, priority) VALUES ('acct1', 1, 61115, '健美操', 0)"); err != nil {
		t.Fatal(err)
	}
	d.Close()

	got, err := Open(path)
	if err != nil {
		t.Fatalf("缺 publish_name/begin_date 列的旧库应被自动迁移而非拒绝启动: %v", err)
	}
	defer got.Close()
	// 迁移后两列已补齐
	var n int
	if err := got.QueryRow("SELECT count(*) FROM pragma_table_info('targets') WHERE name IN ('publish_name','begin_date')").Scan(&n); err != nil || n != 2 {
		t.Fatalf("迁移后应同时补齐 publish_name/begin_date 两列，实际 %d (%v)", n, err)
	}
	// 旧目标行数据保留，且新列可读（默认空串）
	var name string
	if err := got.QueryRow("SELECT course_name FROM targets WHERE class_id=61115").Scan(&name); err != nil || name != "健美操" {
		t.Fatalf("迁移后旧目标数据丢失: %q %v", name, err)
	}
}

// TestMigrateAddsCredentialPlatformColumn 旧库（credentials 无 platform_id）必须被自动
// 增量迁移而非拒绝启动。该列是"跨平台 token 一律丢弃"的判据来源：旧行迁移后为空串
// （= 平台未知），与任何档案 ID 都不相等 → 旧 token 被丢弃、多一次自动重登，
// 但账号/目标/成功记录一行不丢（"数据都要保存好啊"契约）。
func TestMigrateAddsCredentialPlatformColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-cred.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// 上一版形状的 credentials（无 platform_id）+ 一条真实凭据行
	if _, err := d.Exec("CREATE TABLE credentials (account TEXT PRIMARY KEY, password_enc TEXT NOT NULL, id_token TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT (datetime('now')))"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("INSERT INTO credentials (account, password_enc, id_token) VALUES ('acct1','ENC-ABC','tok-1')"); err != nil {
		t.Fatal(err)
	}
	d.Close()

	got, err := Open(path)
	if err != nil {
		t.Fatalf("缺 platform_id 列的旧库应被自动迁移而非拒绝启动: %v", err)
	}
	defer got.Close()
	var n int
	if err := got.QueryRow("SELECT count(*) FROM pragma_table_info('credentials') WHERE name='platform_id'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("迁移后应补齐 platform_id 列，实际 %d (%v)", n, err)
	}
	var acct, enc, tok, platform string
	if err := got.QueryRow("SELECT account, password_enc, id_token, platform_id FROM credentials WHERE account='acct1'").Scan(&acct, &enc, &tok, &platform); err != nil {
		t.Fatalf("迁移后旧凭据行必须可读: %v", err)
	}
	if acct != "acct1" || enc != "ENC-ABC" || tok != "tok-1" {
		t.Fatalf("迁移改变了旧行数据: %q %q %q", acct, enc, tok)
	}
	if platform != "" {
		t.Fatalf("旧行的 platform_id 应为空串（平台未知），实际 %q", platform)
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

package store

import (
	"path/filepath"
	"testing"

	"xuanke-auto/backend/internal/db"
	"xuanke-auto/backend/internal/scheduler"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return New(d)
}

func TestAccountRoundTrip(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveAccount("acct1", "pwd1", "tok1"); err != nil {
		t.Fatal(err)
	}
	acct, pwd, token, err := s.LoadAccount()
	if err != nil {
		t.Fatal(err)
	}
	if acct != "acct1" || pwd != "pwd1" || token != "tok1" {
		t.Fatalf("账密往返失败: %q %q %q", acct, pwd, token)
	}
	// 覆盖保存
	if err := s.SaveAccount("acct2", "pwd2", "tok2"); err != nil {
		t.Fatal(err)
	}
	acct, pwd, token, _ = s.LoadAccount()
	if acct != "acct2" {
		t.Fatalf("覆盖保存失败: %q", acct)
	}
}

func TestSaveTokenOnly(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveTokenOnly("tok-env"); err != nil {
		t.Fatal(err)
	}
	acct, pwd, token, err := s.LoadAccount()
	if err != nil {
		t.Fatal(err)
	}
	if token != "tok-env" {
		t.Fatalf("token 未保存: %q", token)
	}
	// 纯 token 会话：账密为空但 token 存在
	if acct != "" || pwd != "" {
		t.Fatalf("纯 token 会话不应有账密: %q %q", acct, pwd)
	}
	// 覆盖保存
	if err := s.SaveTokenOnly("tok-env-2"); err != nil {
		t.Fatal(err)
	}
	_, _, token, _ = s.LoadAccount()
	if token != "tok-env-2" {
		t.Fatalf("覆盖保存失败: %q", token)
	}
}

func TestTargetsRoundTrip(t *testing.T) {
	s := openTestStore(t)
	targets := []scheduler.Target{
		{PublishID: 1, ClassID: 61115, CourseName: "健美操"},
		{PublishID: 2, ClassID: 61205, CourseName: "篮球"},
	}
	if err := s.SetTargets(targets); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ClassID != 61115 || got[1].CourseName != "篮球" {
		t.Fatalf("目标往返失败: %+v", got)
	}
	// 设置新目标应清空旧的
	if err := s.SetTargets([]scheduler.Target{{PublishID: 3, ClassID: 61276, CourseName: "瑜伽"}}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.LoadTargets()
	if len(got) != 1 || got[0].ClassID != 61276 {
		t.Fatalf("替换目标失败: %+v", got)
	}
}

func TestLogs(t *testing.T) {
	s := openTestStore(t)
	if err := s.AppendLog(61115, "select", "报名成功", true); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendLog(61115, "select", "名额已满", false); err != nil {
		t.Fatal(err)
	}
	logs, err := s.LoadLogs(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("期望 2 条日志，实际 %d", len(logs))
	}
	// LoadLogs 按 id DESC：最新在前
	if logs[0].IsOK || !logs[1].IsOK {
		t.Fatalf("日志 is_ok 标记错误: %+v", logs)
	}
}

func TestStateRoundTrip(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveState("2026-09-13 09:00:00", true, []int{61115, 61205}); err != nil {
		t.Fatal(err)
	}
	openTime, opened, ids, err := s.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if openTime != "2026-09-13 09:00:00" || !opened || len(ids) != 2 || ids[0] != 61115 {
		t.Fatalf("状态往返失败: %q %v %v", openTime, opened, ids)
	}
}
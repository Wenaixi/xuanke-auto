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

func TestCredentialsRoundTrip(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveCredential("acct1", "ENC-ABC", "tok1"); err != nil {
		t.Fatal(err)
	}
	creds, err := s.LoadCredentials()
	if err != nil || len(creds) != 1 || creds[0].Account != "acct1" || creds[0].PasswordEnc != "ENC-ABC" {
		t.Fatalf("凭据往返失败: %+v %v", creds, err)
	}
	// upsert 覆盖
	if err := s.SaveCredential("acct1", "ENC-XYZ", "tok2"); err != nil {
		t.Fatal(err)
	}
	creds, _ = s.LoadCredentials()
	if len(creds) != 1 || creds[0].PasswordEnc != "ENC-XYZ" {
		t.Fatalf("upsert 失败: %+v", creds)
	}
	if err := s.UpdateIDToken("acct1", "tok-2"); err != nil {
		t.Fatal(err)
	}
	creds, _ = s.LoadCredentials()
	if creds[0].IDToken != "tok-2" {
		t.Fatalf("token 更新失败: %+v", creds)
	}
}

func TestSuccessRecords(t *testing.T) {
	s := openTestStore(t)
	s.SaveSuccess("acct1", 61115)
	s.SaveSuccess("acct1", 61115) // 幂等
	s.SaveSuccess("acct2", 61205)
	got, err := s.LoadSuccess()
	if err != nil || len(got["acct1"]) != 1 || len(got["acct2"]) != 1 {
		t.Fatalf("成功记录异常: %+v %v", got, err)
	}
}

func TestTargetsByAccount(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveAccountName("acct1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountName("acct2"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountName("acct1"); err != nil { // 幂等
		t.Fatal(err)
	}
	got, err := s.ListAccounts()
	if err != nil || len(got) != 2 || got[0] != "acct1" || got[1] != "acct2" {
		t.Fatalf("账号列表异常: %v %v", got, err)
	}

	// 账号隔离目标
	t1 := []scheduler.Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操"}}
	t2 := []scheduler.Target{{PublishID: 2, ClassID: 61205, CourseName: "篮球"}}
	if err := s.SetTargetsForAccount("acct1", t1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTargetsForAccount("acct2", t2); err != nil {
		t.Fatal(err)
	}
	l1, _ := s.LoadTargetsForAccount("acct1")
	l2, _ := s.LoadTargetsForAccount("acct2")
	if len(l1) != 1 || l1[0].ClassID != 61115 || len(l2) != 1 || l2[0].ClassID != 61205 {
		t.Fatalf("账号目标隔离失败: %+v %+v", l1, l2)
	}
	// 替换清空
	if err := s.SetTargetsForAccount("acct1", nil); err != nil {
		t.Fatal(err)
	}
	empty, _ := s.LoadTargetsForAccount("acct1")
	if len(empty) != 0 {
		t.Fatalf("清空目标失败: %+v", empty)
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
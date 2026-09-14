package store

import (
	"path/filepath"
	"sync"
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

// openStoreMultiConn 打开一个多连接的 Store（不设单连接上限）。
// 用途：复现激活码并发扣减的"读改写"竞态——生产配置单连接会串行化掩盖竞态，
// 这里放开连接数以暴露真实的并发语义（future-proof：改连接数/跨进程即会触发）。
func openStoreMultiConn(t *testing.T) *Store {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.SetMaxOpenConns(16) // 放开单连接：并发事务真正并行
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

// TestRefusedRecords B9-02：已退选记录的 Save/Delete/Load 往返 + 按账号隔离 +
// DeleteRefused 清空全部（重设目标语义）。
func TestRefusedRecords(t *testing.T) {
	s := openTestStore(t)
	if err := s.SaveRefused("acct1", 61115); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRefused("acct1", 61115); err != nil { // 幂等
		t.Fatal(err)
	}
	if err := s.SaveRefused("acct1", 61205); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRefused("acct2", 61276); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadRefused()
	if err != nil {
		t.Fatal(err)
	}
	if len(got["acct1"]) != 2 || len(got["acct2"]) != 1 {
		t.Fatalf("已退选记录异常: %+v %v", got, err)
	}
	// 重设目标：清空该账号全部退选（acct2 不受影响）
	if err := s.DeleteRefused("acct1"); err != nil {
		t.Fatal(err)
	}
	got, err = s.LoadRefused()
	if err != nil {
		t.Fatal(err)
	}
	if len(got["acct1"]) != 0 || len(got["acct2"]) != 1 {
		t.Fatalf("DeleteRefused 后记录异常: %+v %v", got, err)
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
	t1 := []scheduler.Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}, {PublishID: 1, ClassID: 61205, CourseName: "篮球", Priority: 1}}
	t2 := []scheduler.Target{{PublishID: 2, ClassID: 61205, CourseName: "篮球", Priority: 0}}
	if err := s.SetTargetsForAccount("acct1", t1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTargetsForAccount("acct2", t2); err != nil {
		t.Fatal(err)
	}
	l1, _ := s.LoadTargetsForAccount("acct1")
	l2, _ := s.LoadTargetsForAccount("acct2")
	if len(l1) != 2 || l1[0].ClassID != 61115 || l1[1].ClassID != 61205 || l1[0].Priority != 0 || l1[1].Priority != 1 {
		t.Fatalf("账号目标隔离失败: %+v %+v", l1, l2)
	}
	if len(l2) != 1 || l2[0].ClassID != 61205 {
		t.Fatalf("acct2 目标异常: %+v", l2)
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

func TestLogsByAccount(t *testing.T) {
	s := openTestStore(t)
	if err := s.AppendLog("acct1", 61115, "select", "报名成功", true); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendLog("acct1", 61115, "select", "名额已满", false); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendLog("acct2", 61205, "select", "报名成功", true); err != nil {
		t.Fatal(err)
	}
	logs1, err := s.LoadLogs("acct1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs1) != 2 {
		t.Fatalf("acct1 应只有 2 条日志，实际 %d", len(logs1))
	}
	logs2, _ := s.LoadLogs("acct2", 10)
	if len(logs2) != 1 || logs2[0].ClassID != 61205 {
		t.Fatalf("acct2 日志异常: %+v", logs2)
	}
}

func TestActivationCodeConcurrentConsume(t *testing.T) {
	s := openStoreMultiConn(t) // 多连接：并发事务真正并行，竞态可复现
	if err := s.CreateActivationCode("XK-CONC-0000-0001", 1); err != nil {
		t.Fatal(err)
	}
	// 20 个账号同一时刻抢同一个"可用 1 次"的激活码：
	// 原子扣减保证恰好 1 个成功；其余 19 个返回"无效/用尽"(false, nil)，绝不报锁错。
	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := s.ConsumeActivationCode("XK-CONC-0000-0001", "acct"+string(rune('0'+i)))
			if err != nil {
				t.Errorf("激活出错（锁忙应排队而非报错）: %v", err)
				return
			}
			if ok {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("uses=1 的激活码并发消费应恰好 1 个成功，实际 %d", okCount)
	}
	codes, _ := s.ListActivationCodes()
	if len(codes) != 1 || codes[0].UsedUses != 1 {
		t.Fatalf("并发后 used_uses 应精确为 1（不超卖），实际 %+v", codes)
	}
}

func TestActivationCodes(t *testing.T) {
	s := openTestStore(t)
	// 生成激活码
	if err := s.CreateActivationCode("XK-ABCD-EF12-3456", 2); err != nil {
		t.Fatal(err)
	}
	// 激活前账号未激活
	act, _ := s.IsActivated("acct1")
	if act {
		t.Fatal("新账号不应已激活")
	}
	// 消耗一次激活
	ok, err := s.ConsumeActivationCode("XK-ABCD-EF12-3456", "acct1")
	if err != nil || !ok {
		t.Fatalf("首次激活失败: %v %v", ok, err)
	}
	act, _ = s.IsActivated("acct1")
	if !act {
		t.Fatal("激活后账号应已激活")
	}
	// 再消耗一次（同码）
	ok, err = s.ConsumeActivationCode("XK-ABCD-EF12-3456", "acct2")
	if err != nil || !ok {
		t.Fatalf("第二次激活失败: %v %v", ok, err)
	}
	// 第三次应失败（次数用尽）
	ok, err = s.ConsumeActivationCode("XK-ABCD-EF12-3456", "acct3")
	if err != nil || ok {
		t.Fatalf("次数用尽后不应成功: %v %v", ok, err)
	}
	// 无效码
	ok, err = s.ConsumeActivationCode("XK-INVALID-0000-0000", "acct4")
	if err != nil || ok {
		t.Fatalf("无效激活码不应成功: %v %v", ok, err)
	}
	// 列表
	codes, err := s.ListActivationCodes()
	if err != nil || len(codes) != 1 || codes[0].UsedUses != 2 {
		t.Fatalf("激活码列表异常: %+v %v", codes, err)
	}
	// 删除
	if err := s.DeleteActivationCode("XK-ABCD-EF12-3456"); err != nil {
		t.Fatal(err)
	}
	codes, _ = s.ListActivationCodes()
	if len(codes) != 0 {
		t.Fatalf("删除后应为空: %+v", codes)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	s := openTestStore(t)
	// 空读
	kv, err := s.LoadSettings()
	if err != nil || len(kv) != 0 {
		t.Fatalf("初始应无配置: %v %v", kv, err)
	}
	// 写入全量
	if err := s.SaveSettings(map[string]string{
		"activation_enabled": "true",
		"vision_key":         "***REMOVED***",
	}); err != nil {
		t.Fatal(err)
	}
	kv, _ = s.LoadSettings()
	if kv["activation_enabled"] != "true" || kv["vision_key"] != "***REMOVED***" {
		t.Fatalf("配置往返失败: %v", kv)
	}
	// 全量替换（旧的消失）
	if err := s.SaveSettings(map[string]string{"activation_enabled": "false"}); err != nil {
		t.Fatal(err)
	}
	kv, _ = s.LoadSettings()
	if len(kv) != 1 || kv["activation_enabled"] != "false" {
		t.Fatalf("全量替换失败: %v", kv)
	}
}

func TestLogs(t *testing.T) {
	s := openTestStore(t)
	if err := s.AppendLog("acct1", 61115, "select", "报名成功", true); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendLog("acct1", 61115, "select", "名额已满", false); err != nil {
		t.Fatal(err)
	}
	logs, err := s.LoadLogs("acct1", 10)
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

func TestAdminStore(t *testing.T) {
	s := openTestStore(t)
	// 造数据：两个账号 + 目标 + 成功记录
	if err := s.SaveAccountName("acct1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountName("acct2"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTargetsForAccount("acct1", []scheduler.Target{{PublishID: 1, ClassID: 61115, CourseName: "健美操", Priority: 0}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSuccess("acct1", 61115); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSuccess("acct2", 61205); err != nil {
		t.Fatal(err)
	}

	// 管理员账号列表：账号 + 各自目标 + 成功
	list, err := s.ListAdminAccounts()
	if err != nil || len(list) != 2 {
		t.Fatalf("管理员账号列表异常: %+v %v", list, err)
	}
	for _, a := range list {
		if a.Account == "acct1" {
			if len(a.Targets) != 1 || len(a.Success) != 1 {
				t.Fatalf("acct1 应含目标与成功记录: %+v", a)
			}
		}
		if a.Account == "acct2" {
			if len(a.Targets) != 0 || len(a.Success) != 1 {
				t.Fatalf("acct2 成功记录异常: %+v", a)
			}
		}
	}

	// 全量日志（跨账号）
	if err := s.AppendLog("acct1", 61115, "select", "报名成功", true); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendLog("acct2", 61205, "select", "报名成功", true); err != nil {
		t.Fatal(err)
	}
	all, err := s.LoadAllLogs(10)
	if err != nil || len(all) != 2 {
		t.Fatalf("全量日志异常: %+v %v", all, err)
	}

	// 删除账号：凭据/账号名/目标/成功/激活全清，日志保留
	if err := s.DeleteAccount("acct1"); err != nil {
		t.Fatal(err)
	}
	names, _ := s.ListAccounts()
	if len(names) != 1 || names[0] != "acct2" {
		t.Fatalf("删除后账号列表异常: %v", names)
	}
	ts, _ := s.LoadTargetsForAccount("acct1")
	if len(ts) != 0 {
		t.Fatalf("删除后 acct1 目标应清空: %+v", ts)
	}
	all, _ = s.LoadAllLogs(10)
	if len(all) != 2 {
		t.Fatalf("删除账号不应清日志（审计保留）: %+v", all)
	}
}
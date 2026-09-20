package store

// 空库窗口语义实证（R55 主控独立验证）：
// task_log 档①窗口前置 `WHERE id > (SELECT max(id)-20000)` 在空库时
// max(id)=NULL → id > NULL 恒 false → 查询返回 0 行。本测试钉死该行为：
// 空库返回空 slice 而非报错，且不 panic——这是日志展示层的安全语义。
import (
	"path/filepath"
	"testing"

	"xuanke-auto/backend/internal/db"
)

func TestLoadLogsEmptyDBWindowSemantics(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	s := New(d)

	// 空库 LoadAllLogs：窗口前置下 max(id) 为 NULL，必须返回空而非报错
	all, err := s.LoadAllLogs(2000)
	if err != nil {
		t.Fatalf("空库 LoadAllLogs 不应报错: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("空库 LoadAllLogs 应返回 0 行，实际 %d", len(all))
	}

	// 空库 LoadLogs（账号维度）：同样返回空而非报错
	logs, err := s.LoadLogs("acct1", 100)
	if err != nil {
		t.Fatalf("空库 LoadLogs 不应报错: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("空库 LoadLogs 应返回 0 行，实际 %d", len(logs))
	}
}

package store

import (
	"fmt"
	"testing"
)

// TDD 红灯→绿灯：CountAllLogs 与 LoadAllLogs 的计数必须一致——
// /admin/stats 改用 COUNT 替 LoadAllLogs(1000) 只取 len 的优化前提是两者同口径
// （同一条新日志两查询都算、同一条超窗日志两查询都不算）。
// 写若干条日志后对比两个查询结果，再写一批确认仍一致。
func TestCountAllLogsMatchesLoadAllLogs(t *testing.T) {
	s := openTestStore(t)
	// 空库：两查询都 0
	if n, err := s.CountAllLogs(); err != nil || n != 0 {
		t.Fatalf("空库 CountAllLogs 应 0: n=%d err=%v", n, err)
	}
	// 写 5 条
	for i := 0; i < 5; i++ {
		if err := s.AppendLog("acct1", 61115, "select", fmt.Sprintf("选课第%d次", i), true); err != nil {
			t.Fatal(err)
		}
	}
	// 两查询一致（LoadAllLogs 上限 500 > 5，全量可比）
	if n, err := s.CountAllLogs(); err != nil || n != 5 {
		t.Fatalf("5 条后 CountAllLogs 应 5: n=%d err=%v", n, err)
	}
	logs, err := s.LoadAllLogs(500)
	if err != nil || len(logs) != 5 {
		t.Fatalf("LoadAllLogs 应 5 条: len=%d err=%v", len(logs), err)
	}
}

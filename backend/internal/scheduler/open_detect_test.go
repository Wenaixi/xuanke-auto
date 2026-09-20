package scheduler

import (
	"testing"
	"time"
)

// TestAccountOpenTimeDetection 开放时间自动识别——每账号分别维护识别状态：
// 探测成功且平台返回 beginTimes 时，把该账号的"预计开放时间"记入自己的识别槽；
// 未识别到的账号绝不借用别账号的识别值，也绝不显示编造的默认值（识别不到=未知）。
func TestAccountOpenTimeDetection(t *testing.T) {
	// 场景 1：ProbeForAccount 探测成功且平台返回 beginTimes → 该账号识别到开放时间
	fc := newFakeClient(false)
	// 未来时间戳（+2 小时），保证识别值仍在未来、未触发"识别过期"语义
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)

	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	st := s.StateForAccount("acct1")
	if !st.OpenTimeKnown {
		t.Fatal("平台下发 beginTimes 后该账号必须识别到开放时间（open_time_known=true）")
	}
	want := time.UnixMilli(fc.data.BeginTimes[0]).Format("2006-01-02 15:04:05")
	if got := st.OpenTime.Format("2006-01-02 15:04:05"); got != want {
		t.Fatalf("识别到的时间应为 %s，实际 %s", want, got)
	}

	// 场景 2：账号隔离——acct2 尚未探测（无专属帧触发），不得借用 acct1 的识别值
	st2 := s.StateForAccount("acct2")
	if st2.OpenTimeKnown {
		t.Fatal("未识别到 beginTimes 的账号必须 open_time_known=false，绝不借用其他账号识别值")
	}
	if st2.OpenTime.Year() != 1 {
		t.Fatalf("未识别账号的 open_time 应为零值（前端显示未知），实际 %v", st2.OpenTime)
	}

	// 场景 3：平台未下发 beginTimes（空数组）时保持未知，绝不硬造时间
	fc2 := newFakeClient(false)
	s3 := New(&fakeAccts{c: fc2}, &fakeStore{}, time.Time{}, time.Hour)

	if _, err := s3.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	st3 := s3.StateForAccount("acct1")
	if st3.OpenTimeKnown {
		t.Fatal("平台未下发 beginTimes 时必须 open_time_known=false（识别不到=未知）")
	}
}

// TestAccountOpenTimeScopedPerAccount 识别时间必须按账号隔离——两个账号各自探测，
// 各记各的识别值，绝不互相覆盖（多账号年级/识别状态物理隔离契约）。
func TestAccountOpenTimeScopedPerAccount(t *testing.T) {
	fc := newFakeClient(false)
	tsA := time.Now().Add(2 * time.Hour)
	fc.data.BeginTimes = []int64{tsA.UnixMilli()}
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)

	if _, err := s.ProbeForAccount("acctA"); err != nil {
		t.Fatalf("探测 acctA 失败: %v", err)
	}
	// acctB 探测返回不同批次时间（模拟未来错峰部署——虽然当前平台全校统一，但要防止
	// 识别槽被串写）：两个账号的识别记录必须彼此独立
	tsB := time.Now().Add(3 * time.Hour)
	fc.data.BeginTimes = []int64{tsB.UnixMilli()}
	if _, err := s.ProbeForAccount("acctB"); err != nil {
		t.Fatalf("探测 acctB 失败: %v", err)
	}

	wantA := tsA.Format("2006-01-02 15:04:05")
	if got := s.StateForAccount("acctA").OpenTime.Format("2006-01-02 15:04:05"); got != wantA {
		t.Fatalf("acctA 识别时间被 acctB 串写：应保持 %s，实际 %s", wantA, got)
	}
	wantB := tsB.Format("2006-01-02 15:04:05")
	if got := s.StateForAccount("acctB").OpenTime.Format("2006-01-02 15:04:05"); got != wantB {
		t.Fatalf("acctB 识别时间错误：应为 %s，实际 %s", wantB, got)
	}
}

// TestPurgeAccountClearsOpenTime 删除账号必须同步清空该账号的开放时间识别槽
// （与 PurgeAccount 全量清理契约一致：删号绝不残留任何账号态）。
func TestPurgeAccountClearsOpenTime(t *testing.T) {
	fc := newFakeClient(false)
	fc.data.BeginTimes = []int64{time.Now().Add(2 * time.Hour).UnixMilli()}
	s := New(&fakeAccts{c: fc}, &fakeStore{}, time.Time{}, time.Hour)

	if _, err := s.ProbeForAccount("acct1"); err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	if !s.StateForAccount("acct1").OpenTimeKnown {
		t.Fatal("前置：acct1 应先识别成功")
	}
	s.PurgeAccount("acct1")
	st := s.StateForAccount("acct1")
	if st.OpenTimeKnown {
		t.Fatal("删账号后开放时间识别槽必须清空（open_time_known=false）")
	}
	if st.OpenTime.Year() != 1 {
		t.Fatal("删账号后 open_time 必须归零")
	}
}

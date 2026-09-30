package scheduler

import (
	"testing"
	"time"

	"xuanke-auto/backend/internal/upstream"
)

// TestWindowSignalLocked 开窗判定的四种形态。三态 selectable 的存在意义就在这里：
// 「站点说未开」与「站点无此信号」必须走不同分支，否则无信号平台永远开不了窗。
func TestWindowSignalLocked(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		name       string
		hasSignal  bool
		data       *upstream.ElectivesData
		wantOpened bool
	}{
		{
			name:       "nil 快照判未开",
			hasSignal:  true,
			data:       nil,
			wantOpened: false,
		},
		{
			name:       "有信号且任一发布开窗=开窗",
			hasSignal:  true,
			data:       &upstream.ElectivesData{Publishes: []upstream.Publish{{Selectable: &no}, {Selectable: &yes}}},
			wantOpened: true,
		},
		{
			name:       "有信号且全部未开=未开",
			hasSignal:  true,
			data:       &upstream.ElectivesData{Publishes: []upstream.Publish{{Selectable: &no}, {Selectable: &no}}},
			wantOpened: false,
		},
		{
			name:       "有信号但全为 nil（站点未下发该发布的状态）=未开",
			hasSignal:  true,
			data:       &upstream.ElectivesData{Publishes: []upstream.Publish{{Selectable: nil}, {Selectable: nil}}},
			wantOpened: false,
		},
		{
			name: "无信号+有数据=开窗（退化模式的核心）",
			// 站点完全不下发开窗信号：有课程数据即视为开窗，绝不判未开。
			hasSignal:  false,
			data:       &upstream.ElectivesData{Publishes: []upstream.Publish{{Selectable: nil}}},
			wantOpened: true,
		},
		{
			name:       "无信号+空快照=未开",
			hasSignal:  false,
			data:       &upstream.ElectivesData{},
			wantOpened: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
			s.SetHasWindowSignal(c.hasSignal)
			if got := s.windowSignalLocked(c.data); got != c.wantOpened {
				t.Fatalf("windowSignalLocked = %v, want %v", got, c.wantOpened)
			}
		})
	}
}

// TestWindowSignalNilNotTreatedAsClosed 三态 nil 绝不能被读成 false。
// 若把 nil 当 false，则无信号平台在"有发布"的情况下会被判未开窗——
// 这正是退化模式要修的坑（引擎永远开不了窗、调度器全线停摆）。
func TestWindowSignalNilNotTreatedAsClosed(t *testing.T) {
	yes := true
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	// 档案声明「有开窗信号」，但某平台没给该发布填值（nil）
	s.SetHasWindowSignal(true)
	data := &upstream.ElectivesData{Publishes: []upstream.Publish{
		{Selectable: nil},
		{Selectable: &yes}, // 只要有一个明确开窗就算开窗
	}}
	if !s.windowSignalLocked(data) {
		t.Fatal("有发布明确 selectable=true 时应判开窗（nil 不应拖累）")
	}
}

// TestDefaultHasWindowSignalIsTrue 零值陷阱防线：New 必须默认按"站点有信号"初始化。
// 若默认取零值 false，全部探测会走退化模式（有数据即开窗），"窗口未开"的判定
// 彻底失效——表现为窗口未开时课程状态直接 success、提交守卫永不挂起。
func TestDefaultHasWindowSignalIsTrue(t *testing.T) {
	s := New(&fakeAccts{}, &fakeStore{}, time.Now(), time.Hour)
	no := false
	data := &upstream.ElectivesData{Publishes: []upstream.Publish{{Selectable: &no}}}
	if s.windowSignalLocked(data) {
		t.Fatal("New 默认必须是 hasWindowSignal=true（否则窗口未开判不出来）")
	}
}

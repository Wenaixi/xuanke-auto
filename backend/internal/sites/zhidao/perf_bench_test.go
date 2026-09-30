package zhidao

import (
	"encoding/json"
	"fmt"
	"testing"
)

// 站点解码路径的性能基准：纯解析（不含网络），衡量「匿名 raw struct → 中立模型」的分配成本。
// 契约：解析仍在站点包内用结构体解码（不是按字段名表取值），故分配数不得上升；
// 夹具 shape 自带自检（3 发布 82 门课），否则 benchmark 规模前提不成立、数字失去工程意义。
// 用法：go test -run='^$' -bench=. -benchmem -count=3 ./internal/sites/zhidao/

// benchElectivesJSON 生成接近真实规模的课程响应体。
// 真实站点当前激活学期为 3 个发布、约 82 门课（体育 3 + 校本1 40 + 校本2 39）。
// 夹具字段与真实线格式一致（含产品已剔除的 lessons_date/plan_count——服务端仍下发，
// 解析端直接丢弃），故基准的 B/op 有工程意义。
func benchElectivesJSON() []byte {
	type cls struct {
		ID              int    `json:"id"`
		CourseName      string `json:"course_name"`
		ClassName       string `json:"class_name"`
		TeacherNameList string `json:"teacher_name_list"`
		ClassroomName   string `json:"class_room_name"`
		SelectedCount   int    `json:"selected_count"`
		MaxCount        int    `json:"max_count"`
		CanSelect       bool   `json:"can_select"`
		BtnType         int    `json:"btn_type"`
		BtnText         string `json:"btn_text"`
		Title           string `json:"title"`
	}
	type pub struct {
		PublishID   int    `json:"publishId"`
		PublishName string `json:"publishName"`
		BeginDate   string `json:"beginDate"`
		InDateRange bool   `json:"inDateRange"`
		CanSelect   int    `json:"canSelect"`
		HasSelected int    `json:"hasSelected"`
		GroupCount  int    `json:"groupCount"`
		TotalCount  int    `json:"totalCount"`
		Classes     []cls  `json:"electivesClassList"`
	}
	names := []string{"体育", "校本1", "校本2"}
	counts := []int{3, 40, 39}
	pubs := make([]pub, 0, 3)
	id := 61000
	for i, n := range names {
		classes := make([]cls, 0, counts[i])
		for j := 0; j < counts[i]; j++ {
			id++
			classes = append(classes, cls{
				ID:              id,
				CourseName:      fmt.Sprintf("课程名称示例%d", id),
				ClassName:       fmt.Sprintf("教学班%d", id),
				TeacherNameList: "张三,李四",
				ClassroomName:   "教学楼A101",
				SelectedCount:   17,
				MaxCount:        36,
				CanSelect:       true,
				BtnType:         2,
				BtnText:         "报名",
			})
		}
		pubs = append(pubs, pub{
			PublishID:   100 + i,
			PublishName: n,
			BeginDate:   "2026-09-13 09:00:00",
			InDateRange: true,
			CanSelect:   1,
			GroupCount:  counts[i],
			TotalCount:  counts[i],
			Classes:     classes,
		})
	}
	body, err := json.Marshal(map[string]any{
		"code":                0,
		"beginTimes":          []int64{1789261200000},
		"selectElectivesData": pubs,
	})
	if err != nil {
		panic(err)
	}
	return body
}

// BenchmarkDecodeElectives 纯解析路径（不含网络）。
func BenchmarkDecodeElectives(b *testing.B) {
	payload := benchElectivesJSON()
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decodeElectives(payload); err != nil {
			b.Fatal(err)
		}
	}
}

// TestDecodeElectivesBenchShape 基准夹具自检（3 发布 82 门课）。
func TestDecodeElectivesBenchShape(t *testing.T) {
	d, err := decodeElectives(benchElectivesJSON())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Publishes) != 3 {
		t.Fatalf("发布数应为 3，实际 %d", len(d.Publishes))
	}
	total := 0
	for _, p := range d.Publishes {
		total += len(p.Classes)
	}
	if total != 82 {
		t.Fatalf("课程总数应为 82，实际 %d", total)
	}
}

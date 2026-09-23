package api

import (
	"bytes"
	"encoding/json"
	"testing"
)

// writeJSONBody 当前响应体（map 装箱版，改造前基线）。
type mapBody = map[string]any

// jsonWriteMap 模拟 writeJSON 的 map[string]any 装箱序列化（现状基线）。
func jsonWriteMap(buf *bytes.Buffer, code int, data any, msg string) error {
	return json.NewEncoder(buf).Encode(map[string]any{"code": code, "data": data, "msg": msg})
}

// jsonResponse 显式 struct 版（改造后）：无 map 装箱、无 interface 无序遍历。
type jsonResponse struct {
	Code int `json:"code"`
	Data any `json:"data"`
	Msg  string `json:"msg"`
}

// jsonWriteStruct 模拟 writeJSON 的 struct 序列化（优化后）。
func jsonWriteStruct(buf *bytes.Buffer, code int, data any, msg string) error {
	return json.NewEncoder(buf).Encode(jsonResponse{Code: code, Data: data, Msg: msg})
}

// 行为等价性：两个实现序列化出相同的 JSON 语义（字段值一致，忽略 key 顺序）。
func TestJSONBodyMapStructEquivalent(t *testing.T) {
	cases := []struct {
		code int
		data any
		msg  string
	}{
		{0, map[string]any{"token": "tok", "n": 3}, "ok"},
		{1, nil, "失败"},
		{0, "plain", ""},
	}
	for _, c := range cases {
		var a, b bytes.Buffer
		if err := jsonWriteMap(&a, c.code, c.data, c.msg); err != nil {
			t.Fatal(err)
		}
		if err := jsonWriteStruct(&b, c.code, c.data, c.msg); err != nil {
			t.Fatal(err)
		}
		var ma, mb map[string]any
		if err := json.Unmarshal(a.Bytes(), &ma); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b.Bytes(), &mb); err != nil {
			t.Fatal(err)
		}
		// 三种字段值必须逐字段一致（key 顺序无关）
		for _, k := range []string{"code", "data", "msg"} {
			if _, ok := ma[k]; !ok {
				t.Fatalf("map 版缺字段 %s", k)
			}
			if !jsonEqual(ma[k], mb[k]) {
				t.Fatalf("字段 %s 不一致: map=%v struct=%v", k, ma[k], mb[k])
			}
		}
	}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}

// BenchmarkJSONWriteMap 改造前基线。
func BenchmarkJSONWriteMap(b *testing.B) {
	var buf bytes.Buffer
	data := map[string]any{"token": "tok-1234567890", "courses": []int{1, 2, 3}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := jsonWriteMap(&buf, 0, data, "ok"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONWriteStruct 改造后。
func BenchmarkJSONWriteStruct(b *testing.B) {
	var buf bytes.Buffer
	data := map[string]any{"token": "tok-1234567890", "courses": []int{1, 2, 3}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := jsonWriteStruct(&buf, 0, data, "ok"); err != nil {
			b.Fatal(err)
		}
	}
}

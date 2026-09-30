package config

import (
	"reflect"
	"testing"
)

// 运行时配置中心映射的覆盖面守护。
//
// 背景：config.Config（env 侧）与 runtime.Config（进程内热配置中心）是两个独立
// 结构体，字段名系统性错位（ActivationCodesEnabled→ActivationEnabled、
// SF*→Vision*、Port→ListenPort），映射曾以字面量写在 server.go 的 runtime.New(...)
// 里。Go 编译器不校验跨结构体的手抄：新增 Config 字段忘了搬，env 初值静默丢失。
//
// 本测试用反射双向核对：Config 的每个字段要么被 RuntimeSettings 映射、要么在
// 「刻意不进配置中心」白名单里——新增字段漏搬必红。

// notInRuntimeCenter 刻意不进运行时配置中心的字段及理由。
// 不进热配置面是设计决定而非漏抄：改了它们要么需重启、要么由平台形态固定。
var notInRuntimeCenter = map[string]string{
	"DBPath":           "数据库路径属引擎初始化序（建库/主密钥），不属热配置面",
	"AdminToken":       "管理口令走 api.Options 鉴权面，不进配置中心（明文不落 settings）",
	"AdminName":        "管理员账号名走 api.Options 鉴权面",
	"PlatformEmbedded": "APK 形态由 Java 壳固定，管理员不可改（端口锁 3091）",
}

// configToRuntime 记录 config.Config 每个字段映射到 runtime.Config 的哪个字段。
// 反射测试据此逐字段核对，新增 Config 字段时若忘了在此登记与实现，测试必红。
var configToRuntime = map[string]string{
	"ActivationCodesEnabled": "ActivationEnabled",
	"SFBaseURL":              "VisionBaseURL",
	"SFAPIKey":               "VisionAPIKey",
	"SFModel":                "VisionModel",
	"ListenHost":             "ListenHost",
	"Port":                   "ListenPort",
	"PlatformID":             "PlatformID",
	"PlatformBaseURL":        "PlatformBaseURL",
}

// TestEveryConfigFieldIsMapped Config 每个字段都被映射或显式豁免。
func TestEveryConfigFieldIsMapped(t *testing.T) {
	typ := reflect.TypeOf(Config{})
	for i := range typ.NumField() {
		f := typ.Field(i).Name
		if _, skip := notInRuntimeCenter[f]; skip {
			continue
		}
		if _, mapped := configToRuntime[f]; !mapped {
			t.Errorf("Config 字段 %s 既未映射进运行时配置中心、也不在 notInRuntimeCenter "+
				"白名单中——env 初值会静默丢失", f)
		}
	}
	// 反向：白名单与映射表里不得有已不存在的字段（改名后残留会静默放过新字段）
	known := map[string]bool{}
	for i := range typ.NumField() {
		known[typ.Field(i).Name] = true
	}
	for f := range notInRuntimeCenter {
		if !known[f] {
			t.Errorf("notInRuntimeCenter 里的 %s 已不是 Config 字段（改名后残留）", f)
		}
	}
	for f, target := range configToRuntime {
		if !known[f] {
			t.Errorf("configToRuntime 里的源字段 %s 已不是 Config 字段（改名后残留）", f)
		}
		_ = target
	}
}

// TestRuntimeSettingsPropagatesAllMappedFields 逐条断言映射真的把值搬了过去。
// 此前激活码开关与视觉 API 密钥两条路径无任何测试覆盖（三个直调 runServer 的
// 根包测试都不设它们，恒走零值分支），漏搬不会有人发现。
func TestRuntimeSettingsPropagatesAllMappedFields(t *testing.T) {
	in := Config{
		ActivationCodesEnabled: true,
		SFAPIKey:               "sk-testkeyfortestonly",
		SFBaseURL:              "https://api.example.com/v1",
		SFModel:                "model-x",
		ListenHost:             "0.0.0.0",
		Port:                   "3091",
		PlatformID:             "zhidao-2026",
		PlatformBaseURL:        "https://mirror.example.com",
	}
	got := RuntimeSettings(in)

	type pair struct {
		src  string
		dst  string
		want any
		ok   bool
		str  string
	}
	pairs := []pair{
		{"ActivationCodesEnabled", "ActivationEnabled", nil, in.ActivationCodesEnabled, ""},
		{"SFBaseURL", "VisionBaseURL", nil, false, in.SFBaseURL},
		{"SFAPIKey", "VisionAPIKey", nil, false, in.SFAPIKey},
		{"SFModel", "VisionModel", nil, false, in.SFModel},
		{"ListenHost", "ListenHost", nil, false, in.ListenHost},
		{"Port", "ListenPort", nil, false, in.Port},
		{"PlatformID", "PlatformID", nil, false, in.PlatformID},
		{"PlatformBaseURL", "PlatformBaseURL", nil, false, in.PlatformBaseURL},
	}
	gotVal := reflect.ValueOf(got)
	for _, p := range pairs {
		fv := gotVal.FieldByName(p.dst)
		if !fv.IsValid() {
			t.Errorf("RuntimeSettings 结果没有字段 %s（源 %s）", p.dst, p.src)
			continue
		}
		if p.ok {
			if !fv.Bool() {
				t.Errorf("%s → %s 未传入（期望 true）", p.src, p.dst)
			}
			continue
		}
		if fv.String() != p.str {
			t.Errorf("%s → %s 未正确传入：期望 %q，实际 %q", p.src, p.dst, p.str, fv.String())
		}
	}
}

// TestRuntimeSettingsTakesLocalConfig 入参必须是本包 Config。
// 若改成 runtime.Config 说明依赖方向反了——会让 config 包 import runtime，
// 经 sites→upstream(native_ocr, //go:build cgo)→config 成环，这正是
// DefaultPlatformID 刻意用字面量而非 import 注册表要躲的坑（仅 CGO=1 暴露）。
func TestRuntimeSettingsTakesLocalConfig(t *testing.T) {
	if got := reflect.TypeOf(RuntimeSettings).In(0); got != reflect.TypeOf(Config{}) {
		t.Fatalf("RuntimeSettings 入参必须是本包 Config，实际 %v", got)
	}
}

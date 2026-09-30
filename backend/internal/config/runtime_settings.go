package config

// RuntimeSettings 是本包 Config 中「会被搬进运行时配置中心」的那部分投影。
//
// 为什么收在这里而不是 runtime 包：运行时配置中心（runtime.Store）不允许
// 反向依赖本包——runtime 经 sites → upstream(native_ocr, //go:build cgo) →
// config 成环，只在 CGO=1 构建时暴露。本包零内部依赖，投影放在这里是安全侧
// （runtime → config 单向新增亦可，但本包内聚更省一条依赖边）。
//
// 字段名系统性错位是刻意的平台中立化结果：SF* 是 SiliconFlow 的历史命名，
// Vision* 是跨平台中立名（与 ADR-0004 平台档案化同方向）；env 键名仍是 SF_*，
// 在 Load() 里硬编码，改字段名不破坏 .env 契约。
//
// 覆盖面由 runtime_mapping_test.go 的反射测试钉死：Config 新增字段若忘了在此
// 投影，测试必红（Go 编译器不校验跨结构体的手抄）。
type RuntimeSettingsView struct {
	ActivationEnabled bool
	VisionBaseURL     string
	VisionAPIKey      string
	VisionModel       string
	ListenHost        string
	ListenPort        string
	PlatformID        string
	PlatformBaseURL   string
}

// RuntimeSettings 把启动配置投影成运行时配置中心需要的字段集合。
// 纯函数、无副作用、无锁——装配期调用一次即可。
func RuntimeSettings(c Config) RuntimeSettingsView {
	return RuntimeSettingsView{
		ActivationEnabled: c.ActivationCodesEnabled,
		VisionBaseURL:     c.SFBaseURL,
		VisionAPIKey:      c.SFAPIKey,
		VisionModel:       c.SFModel,
		ListenHost:        c.ListenHost,
		ListenPort:        c.Port,
		PlatformID:        c.PlatformID,
		PlatformBaseURL:   c.PlatformBaseURL,
	}
}

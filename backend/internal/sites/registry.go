// Package sites 上游选课平台档案注册表。
//
// 每新增一个年份/站点的接口形态，就在 backend/internal/sites/<名字>/ 放一个适配器包
// （提供 upstream.SiteDescriptor），并在此表加一行——引擎、调度器、前端一行不改。
//
// 契约：**内置档案只增不删**。settings 表里持久化的 platform_id 必须始终可解析，
// 否则升级/降级后会静默回退到别的档案（= 把请求打到另一个站点）。历史档案即使退役
// 也要留在表内（可标注"仅回溯"），由 TestHistoricalIDsRetained 锁定。
package sites

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"xuanke-auto/backend/internal/sites/testsite"
	"xuanke-auto/backend/internal/sites/zhidao"
	"xuanke-auto/backend/internal/upstream"
)

// DefaultID 未配置平台时使用的档案（.env 的 XUANKE_PLATFORM 缺省值）。
const DefaultID = zhidao.ID

// builtins 内置档案表（键 = ID）。新增档案只在此加一行。
var builtins = map[string]upstream.SiteDescriptor{
	zhidao.ID:   zhidao.Descriptor(),
	testsite.ID: testsite.Descriptor(),
}

// Info 档案的管理员可见元数据（后台下拉展示用；不含客户端工厂与解码钩子）。
type Info struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Note           string `json:"note"`
	DefaultBaseURL string `json:"default_base_url"`
}

// List 返回全部内置档案（按 ID 排序，顺序稳定供前端渲染）。
func List() []Info {
	out := make([]Info, 0, len(builtins))
	for _, d := range builtins {
		out = append(out, Info{
			ID:             d.ID,
			Name:           d.Name,
			Note:           d.Note,
			DefaultBaseURL: d.DefaultBaseURL,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Resolve 按 ID 取档案；未知 ID 返回明确错误并列出全部可选档案——
// 装配面据此拒绝启动或拒绝保存，绝不静默落到别的站点。
func Resolve(id string) (upstream.SiteDescriptor, error) {
	d, ok := builtins[strings.TrimSpace(id)]
	if !ok {
		return upstream.SiteDescriptor{}, fmt.Errorf("未知选课平台 %q，可选：%s", id, strings.Join(IDs(), "、"))
	}
	return d, nil
}


// ResolveValidated 装配单点：解析档案并执行自检（Validate 13 条判据）。
// 启动与命令行工具（probe/logintest）统一走它——此前各调用方手写
// Resolve + Validate 或只 Resolve 不 Validate，工具与生产行为分叉。
func ResolveValidated(id string) (upstream.SiteDescriptor, error) {
	d, err := Resolve(id)
	if err != nil {
		return upstream.SiteDescriptor{}, err
	}
	if err := d.Validate(); err != nil {
		return upstream.SiteDescriptor{}, fmt.Errorf("平台档案自检未通过: %w", err)
	}
	return d, nil
}

// EffectiveBaseURL 计算生效的站点地址：覆盖值优先（非空且合法），否则回退档案默认地址。
// 与 effectiveBaseURL（server.go）同语义：非法覆盖（手改 DB）回退默认并留痕，绝不裸拼接。
func EffectiveBaseURL(desc upstream.SiteDescriptor, override string) string {
	override = strings.TrimSpace(override)
	if override == "" {
		return upstream.NormalizeBaseURL(desc.DefaultBaseURL)
	}
	if err := upstream.ValidateBaseURL(override); err != nil {
		log.Printf("[sites] 站点地址覆盖 %q 非法（%v），回退档案默认地址 %s", override, err, desc.DefaultBaseURL)
		return upstream.NormalizeBaseURL(desc.DefaultBaseURL)
	}
	return upstream.NormalizeBaseURL(override)
}

// IDs 返回全部档案 ID（稳定顺序，供错误提示与测试断言）。
func IDs() []string {
	out := make([]string, 0, len(builtins))
	for id := range builtins {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

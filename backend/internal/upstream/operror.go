package upstream

// OpErrorKind 平台报名/退选错误的结构化分类。站点文案各异（有的中文、有的英文码、
// 有的语义完全不在文案里），但**分流动作**只有四种：退避 / 停止重试 / 记满员 /
// 原文透传。分类判据刻意留在站点档案（各站点的码与文案约定不同），引擎与调度器
// 只消费这一个枚举位——这是「错误分流绝不匹配中文文案」契约的结构化载体。
//
// 为什么必须有这一层：调度器要区分「退避 30s」「按满员记 full 不再轰炸」与
// 「原文透传给用户」。若靠匹配平台文案判断，站点改一次文案就静默改错行为；且
// 平台原文含「已满员」的非满员失败（如「该课已满员，无法退选」）会被误判成满员。
type OpErrorKind int

const (
	// OpErrorUnknown 未分类的普通业务错误（课程不存在等），调用方原文透传。
	OpErrorUnknown OpErrorKind = iota
	// OpErrorRateLimited 平台风控/限流，需退避后重试。
	OpErrorRateLimited
	// OpErrorWindowClosed 选课窗口未开放或已关闭，不可再报（按满员记 full 不轰炸）。
	OpErrorWindowClosed
	// OpErrorClassFull 名额已满，可记 full 不再重试。
	OpErrorClassFull
)

// String 枚举的可读名（日志与调试用；刻意不与平台文案混用）。
func (k OpErrorKind) String() string {
	switch k {
	case OpErrorRateLimited:
		return "限流"
	case OpErrorWindowClosed:
		return "窗口关闭"
	case OpErrorClassFull:
		return "满员"
	}
	return "未分类"
}

// SiteError 平台操作失败的结构化错误。文案原样保留（供用户与日志阅读），
// 分类位由站点档案给出——调度器据此决定退避/记满员/透传，绝不解析文案。
//
// 用指针类型实现 error 接口，配合 errors.As 穿透包装链。
type SiteError struct {
	Kind OpErrorKind
	Msg  string
	Code int
}

func (e *SiteError) Error() string { return e.Msg }

// NewSiteError 构造平台错误（站点适配器包的统一出口）。
func NewSiteError(kind OpErrorKind, msg string, code int) *SiteError {
	return &SiteError{Kind: kind, Msg: msg, Code: code}
}

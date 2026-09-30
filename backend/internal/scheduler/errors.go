package scheduler

import (
	"errors"

	"xuanke-auto/backend/internal/upstream"
)

// platformErrorKind 平台报名错误归一枚举——手动/自动两条提交决策树共用同一分类
// （取代 handler 与 spawnChain 各自手推 upstream.ErrUnauthorized /
// upstream.IsReadErr / net.OpError 文案子串 的重复实现）。
type platformErrorKind int

const (
	errOther        platformErrorKind = iota
	errAuth                           // token 失效（upstream.ErrUnauthorized / 档案声明的未登录码）
	errRead                           // 请求已发出、响应读取中断（平台可能已处理，绝不能当"失败"上报）
	errWindowClosed                   // 选课窗口已关闭/未开启（不可再报，按满员记 full）
	errRateLimit                      // 平台风控退避（记 30s 退避不再轰炸）
	errFull                           // 名额已满（可记 full 不再重试）
)

// classifyPlatformError 把平台报名错误归一为枚举——纯函数：无锁、无副作用，可表驱动测试。
// 判定顺序：
//  1. upstream.ErrUnauthorized（errors.Is 穿透包装链）→ errAuth
//  2. upstream.IsReadErr（read 中断三形态 + 超时，已由 upstream 包收敛）→ errRead
//  3. 结构化平台错误 upstream.SiteError.Kind() → errRateLimit / errWindowClosed / errFull
//
// **绝不匹配中文文案**：文案随站点与版本变化，且平台原文含「已满员」的非满员失败会被
// 误判成满员。分类判据由站点档案给出（SiteDescriptor.OpErrorClassifier），调度器只
// 消费这一个枚举位。未命中 = errOther（普通业务错误，调用方透传原文案）。
func classifyPlatformError(err error) platformErrorKind {
	if err == nil {
		return errOther
	}
	if errors.Is(err, upstream.ErrUnauthorized) {
		return errAuth
	}
	if upstream.IsReadErr(err) {
		return errRead
	}
	var siteErr *upstream.SiteError
	if errors.As(err, &siteErr) {
		switch siteErr.Kind {
		case upstream.OpErrorRateLimited:
			return errRateLimit
		case upstream.OpErrorWindowClosed:
			return errWindowClosed
		case upstream.OpErrorClassFull:
			return errFull
		}
	}
	return errOther
}

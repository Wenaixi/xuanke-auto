package scheduler

import (
	"errors"
	"io"
	"log"
	"net"
	"strings"

	"xuanke-auto/backend/internal/zhidao"
)

// platformErrorKind 平台报名错误归一枚举——手动/自动两条提交决策树共用同一分类
// （C4 收权：取代 handler 与 spawnChain 各自手推 zhidao.ErrUnauthorized /
// zhidao.IsReadErr / net.OpError 文案子串 的重复实现）。
type platformErrorKind int

const (
	errOther platformErrorKind = iota
	errAuth                    // token 失效（zhidao.ErrUnauthorized / 平台 code=-1）
	errRead                    // 请求已发出、响应读取中断（平台可能已处理，绝不能当"失败"上报）
	errWindowClosed            // 选课窗口已关闭/未开启/报名时间已结束（不可再报，按满员记 full）
	errRateLimit               // 平台风控退避（频繁/429/稍后重试，记 30s 退避不再轰炸）
)

// classifyPlatformError 把平台报名错误归一为枚举——纯函数：无锁、无副作用，可表驱动测试。
// 判定顺序（与既有分支逐字一致，见旧 scheduler.go isRateLimitError/isWindowClosedError）：
//   1. zhidao.ErrUnauthorized（errors.Is 穿透包装链）
//   2. zhidao.IsReadErr（read 中断三形态 + 超时，已由 zhidao 包收敛）
//   3. 文案子串：风控（频繁/429/稍后重试）→ 窗口（关闭/未开启/报名时间/已结束）
// 未命中 = errOther（普通业务错误，调用方透传原文案）。
func classifyPlatformError(err error) platformErrorKind {
	if err == nil {
		return errOther
	}
	if errors.Is(err, zhidao.ErrUnauthorized) {
		return errAuth
	}
	if zhidao.IsReadErr(err) {
		return errRead
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "频繁") || strings.Contains(msg, "429") || strings.Contains(msg, "稍后重试"):
		return errRateLimit
	case strings.Contains(msg, "关闭") || strings.Contains(msg, "未开启") ||
		strings.Contains(msg, "报名时间") || strings.Contains(msg, "已结束"):
		return errWindowClosed
	}
	return errOther
}

// 保留旧的 isRateLimitError/isWindowClosedError 调用点迁移提示——
// 删除前 grep 确认零残留；spawnChain 分支已先切换为 classifyPlatformError（C4-1 Step 5）。
var _ = log.Printf
var _ = io.EOF // errors.go 保留 zhidao 依赖（classify 使用），防误删 import
var _ = net.OpError{}
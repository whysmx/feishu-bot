package utils

import "time"

// TimeoutConfig 超时配置（全局统一管理）
type TimeoutConfig struct {
	// HTTP 客户端超时
	HTTPClientTimeout time.Duration

	// CardKit 限流间隔（2 QPS = 500ms）
	CardKitRateLimitInterval time.Duration

	// 流式文本缓冲超时
	StreamIdleTimeout     time.Duration // 空闲超时：N毫秒无新数据则发送
	StreamMaxDuration     time.Duration // 最大持续时间：连续输出N秒后强制分段
	StreamMaxBufferSize   int           // 最大缓冲区大小：超过此大小强制分段

	// 进程管理超时
	ProcessWaitTimeout time.Duration // 等待进程退出的超时时间

	// 刷新定时器
	FlushTimerInterval time.Duration // 批量刷新定时器间隔

	// 工具执行
	ToolInputWaitDelay time.Duration // 等待工具输入完成
}

// DefaultTimeoutConfig 返回默认超时配置
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		// HTTP 客户端：10 秒
		HTTPClientTimeout: 10 * time.Second,

		// CardKit 限流：500ms（2 QPS）
		CardKitRateLimitInterval: 500 * time.Millisecond,

		// 流式缓冲优化配置
		StreamIdleTimeout:     8 * time.Second,  // 8秒无新数据则发送（减少API调用）
		StreamMaxDuration:     20 * time.Second, // 20秒连续输出后强制分段
		StreamMaxBufferSize:   30000,           // 最大30000字符（防止超过飞书150KB限制）

		// 进程管理：5秒
		ProcessWaitTimeout: 5 * time.Second,

		// 批量刷新：3秒（已弃用，保留用于兼容）
		FlushTimerInterval: 3 * time.Second,

		// 工具输入：100ms
		ToolInputWaitDelay: 100 * time.Millisecond,
	}
}

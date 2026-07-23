package types

// common.go 对应 Node src/types/common.ts：Logger 接口 + 错误类型 WsAuthFailureError/WsReconnectExhaustedError。

import "fmt"

// Logger 日志接口，对应 Node src/types/common.ts 的 Logger interface。
type Logger interface {
	Debug(message string, args ...any)
	Info(message string, args ...any)
	Warn(message string, args ...any)
	Error(message string, args ...any)
}

// 错误码常量（镜像 Node 错误类的 code 字段值）。
const (
	// WsAuthFailureCode 认证失败重试次数用尽错误码。
	WsAuthFailureCode = "WS_AUTH_FAILURE_EXHAUSTED"
	// WsReconnectExhaustedCode 连接断开重连次数用尽错误码。
	WsReconnectExhaustedCode = "WS_RECONNECT_EXHAUSTED"
)

// WsAuthFailureError 认证失败重试次数用尽错误。
//
// 当 WebSocket 认证连续失败次数达到 MaxAuthFailureAttempts 时返回。
// 通常表示 botId/secret 配置错误，重试无法恢复。对应 Node WSAuthFailureError。
type WsAuthFailureError struct {
	MaxAttempts int // 触发错误时的最大认证失败重试次数
}

// NewWsAuthFailureError 构造认证失败耗尽错误，对应 Node new WSAuthFailureError(maxAttempts)。
func NewWsAuthFailureError(maxAttempts int) *WsAuthFailureError {
	return &WsAuthFailureError{MaxAttempts: maxAttempts}
}

// Error 返回错误信息。
func (e *WsAuthFailureError) Error() string {
	return fmt.Sprintf("Max auth failure attempts exceeded (%d)", e.MaxAttempts)
}

// Code 返回稳定错误码 WsAuthFailureCode。
func (e *WsAuthFailureError) Code() string {
	return WsAuthFailureCode
}

// WsReconnectExhaustedError 连接断开重连次数用尽错误。
//
// 当 WebSocket 连接断开后重连次数达到 MaxReconnectAttempts 时返回。
// 通常表示网络或服务端持续不可用。对应 Node WSReconnectExhaustedError。
type WsReconnectExhaustedError struct {
	MaxAttempts int // 触发错误时的最大重连次数
}

// NewWsReconnectExhaustedError 构造重连耗尽错误，对应 Node new WSReconnectExhaustedError(maxAttempts)。
func NewWsReconnectExhaustedError(maxAttempts int) *WsReconnectExhaustedError {
	return &WsReconnectExhaustedError{MaxAttempts: maxAttempts}
}

// Error 返回错误信息。
func (e *WsReconnectExhaustedError) Error() string {
	return fmt.Sprintf("Max reconnect attempts exceeded (%d)", e.MaxAttempts)
}

// Code 返回稳定错误码 WsReconnectExhaustedCode。
func (e *WsReconnectExhaustedError) Code() string {
	return WsReconnectExhaustedCode
}

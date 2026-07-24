package types

// config.go 对应 Node src/types/config.ts：WsClientOptions 配置结构。

import (
	"crypto/tls"
	"net/http"
	"time"
)

// WsOptions 底层 WebSocket 连接选项，对应 Node wsOptions（ws 库的 ClientOptions）。
//
// 用于透传 TLS 证书、自定义 Header、拨号超时等底层连接配置。Go 化为字段式选项，
// 由 WsConnectionManager 在拨号时应用。
type WsOptions struct {
	TlsConfig        *tls.Config   // TLS 配置（如自定义 ca、客户端证书、跳过校验等）
	Header           http.Header   // 自定义握手 Header
	DialTimeout      time.Duration // 拨号超时；0 表示不额外限制（由 net/http 默认控制）
	HandshakeTimeout time.Duration // 握手超时；0 表示使用默认值
	Subprotocols     []string      // 子协议列表
}

// WsClientOptions WSClient 配置选项，对应 Node WSClientOptions。
//
// 仅定义结构，默认值由 client（NewWsClient）兜底填充。
type WsClientOptions struct {
	BotId                  string    // 机器人 ID（在企业微信后台获取）
	Secret                 string    // 机器人 Secret（在企业微信后台获取）
	Scene                  int       // 场景值（可选），由使用方传入
	PlugVersion            string    // 插件版本号（可选），由使用方传入
	ReconnectInterval      int       // WebSocket 重连基础延迟（毫秒），实际延迟按指数退避递增，默认 1000
	MaxReconnectAttempts   int       // 连接断开时的最大重连次数，默认 10，设为 -1 表示无限重连
	MaxAuthFailureAttempts int       // 认证失败时的最大重试次数，默认 5，设为 -1 表示无限重试
	HeartbeatInterval      int       // 心跳间隔（毫秒），默认 30000
	RequestTimeout         int       // 请求超时时间（毫秒），默认 10000
	WsUrl                  string    // 自定义 WebSocket 连接地址，默认 wss://openws.work.weixin.qq.com
	WsOptions              WsOptions // 传递给底层 WebSocket 的连接选项（如 TLS 证书配置等）
	MaxReplyQueueSize      int       // 单个 req_id 的回复队列最大长度，超过后新消息将被拒绝，默认 500
	Logger                 Logger    // 自定义日志实现
}

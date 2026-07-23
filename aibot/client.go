package aibot

// client.go 对应 Node src/client.ts：WsClient 核心客户端。
//
// 任务 15：WsClient 结构 + NewWsClient（兜底默认值）+ Connect/Disconnect/IsConnected + 回调桥接。
// 后续任务（17/22/25/26/27）补全 Reply/SendMessage/UploadMedia/DownloadFile 等。

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// WsClient 核心客户端，对应 Node WSClient。
//
// 持有 WsConnectionManager（连接管理）与 MessageHandler（消息分发），通过回调字段
// 暴露连接生命周期与消息/事件。回调签名镜像 Node WSClientEventMap。
type WsClient struct {
	options        types.WsClientOptions // 解析后的配置（含兜底默认值）
	logger         types.Logger          // 日志实现
	wsManager      *WsConnectionManager  // WebSocket 连接管理器
	messageHandler *MessageHandler       // 消息分发处理器

	mu      sync.Mutex // 保护 started
	started bool       // 是否已启动（防重复 Connect/Disconnect）

	// ========== 消息回调（任务 16 分发） ==========
	OnMessage func(frame *types.WsFrame[types.BaseMessage])  // 收到消息（所有类型），body 为 BaseMessage
	OnText    func(frame *types.WsFrame[types.TextMessage])  // 文本消息
	OnImage   func(frame *types.WsFrame[types.ImageMessage]) // 图片消息
	OnMixed   func(frame *types.WsFrame[types.MixedMessage]) // 图文混排消息
	OnVoice   func(frame *types.WsFrame[types.VoiceMessage]) // 语音消息
	OnFile    func(frame *types.WsFrame[types.FileMessage])  // 文件消息
	OnVideo   func(frame *types.WsFrame[types.VideoMessage]) // 视频消息

	// ========== 事件回调（任务 16/19 分发） ==========
	OnEvent             func(frame *types.WsFrame[types.EventMessage]) // 收到事件回调（所有事件类型）
	OnEnterChat         func(frame *types.WsFrame[types.EventMessage]) // 进入会话事件
	OnTemplateCardEvent func(frame *types.WsFrame[types.EventMessage]) // 模板卡片事件
	OnFeedbackEvent     func(frame *types.WsFrame[types.EventMessage]) // 用户反馈事件
	OnDisconnectedEvent func(frame *types.WsFrame[types.EventMessage]) // 连接断开事件（被新连接踢下线）

	// ========== 连接生命周期回调 ==========
	OnConnected     func()              // 连接建立（WebSocket open，认证尚未完成）
	OnAuthenticated func()              // 认证成功
	OnDisconnected  func(reason string) // 连接断开
	OnReconnecting  func(attempt int)   // 重连中
	OnError         func(err error)     // 发生错误
}

// NewWsClient 构造 WsClient，对应 Node WSClient constructor。
//
// 兜底默认值（零值字段应用默认；-1 表示无限重连/重试，保留不覆盖）：
//   - ReconnectInterval = 1000（毫秒）
//   - MaxReconnectAttempts = 10
//   - MaxAuthFailureAttempts = 5
//   - HeartbeatInterval = 30000（毫秒）
//   - RequestTimeout = 10000（毫秒）
//   - MaxReplyQueueSize = 500
//   - Logger = DefaultLogger{}
//
// scene/plug_version 仅在非零/非空时透传到认证帧 body。
func NewWsClient(opts types.WsClientOptions) *WsClient {
	// 兜底默认值
	if opts.ReconnectInterval == 0 {
		opts.ReconnectInterval = 1000
	}
	if opts.MaxReconnectAttempts == 0 {
		opts.MaxReconnectAttempts = 10
	}
	if opts.MaxAuthFailureAttempts == 0 {
		opts.MaxAuthFailureAttempts = 5
	}
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 30000
	}
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = 10000
	}
	if opts.MaxReplyQueueSize == 0 {
		opts.MaxReplyQueueSize = 500
	}
	if opts.Logger == nil {
		opts.Logger = &DefaultLogger{}
	}

	c := &WsClient{
		options: opts,
		logger:  opts.Logger,
	}

	// 初始化 WebSocket 管理器（参数顺序镜像 NewWsConnectionManager）
	c.wsManager = NewWsConnectionManager(
		c.logger,
		opts.HeartbeatInterval,
		opts.ReconnectInterval,
		opts.MaxReconnectAttempts,
		opts.WsUrl,
		opts.WsOptions,
		opts.MaxReplyQueueSize,
		opts.MaxAuthFailureAttempts,
	)

	// 设置认证凭证（scene/plug_version 仅在非零/非空时透传）
	extra := map[string]any{}
	if opts.Scene != 0 {
		extra["scene"] = opts.Scene
	}
	if opts.PlugVersion != "" {
		extra["plug_version"] = opts.PlugVersion
	}
	c.wsManager.SetCredentials(opts.BotId, opts.Secret, extra)

	// 初始化消息处理器
	c.messageHandler = NewMessageHandler(c.logger)

	// 绑定 WebSocket 事件
	c.setupWsEvents()

	return c
}

// setupWsEvents 绑定 WsConnectionManager 事件到 WsClient 回调，对应 Node setupWsEvents。
func (c *WsClient) setupWsEvents() {
	m := c.wsManager

	// 连接建立（WebSocket open，认证尚未完成）
	m.OnConnected = func() {
		if c.OnConnected != nil {
			c.OnConnected()
		}
	}

	// 认证成功
	m.OnAuthenticated = func() {
		c.logger.Info("Authenticated")
		if c.OnAuthenticated != nil {
			c.OnAuthenticated()
		}
	}

	// 连接断开
	m.OnDisconnected = func(reason string) {
		if c.OnDisconnected != nil {
			c.OnDisconnected(reason)
		}
	}

	// 服务端因新连接建立而主动断开旧连接：置 started=false 并触发 OnDisconnected
	m.OnServerDisconnect = func(reason string) {
		c.logger.Warn(fmt.Sprintf("Server disconnected this connection: %s", reason))
		c.mu.Lock()
		c.started = false
		c.mu.Unlock()
		if c.OnDisconnected != nil {
			c.OnDisconnected(reason)
		}
	}

	// 重连中
	m.OnReconnecting = func(attempt int) {
		if c.OnReconnecting != nil {
			c.OnReconnecting(attempt)
		}
	}

	// 错误
	m.OnError = func(err error) {
		if c.OnError != nil {
			c.OnError(err)
		}
	}

	// 收到消息：交由 MessageHandler 分发
	m.OnMessage = func(raw json.RawMessage) {
		c.messageHandler.HandleFrame(raw, c)
	}
}

// Connect 建立 WebSocket 长连接并阻塞至首次认证成功，对应 Node connect()（Go 化为阻塞）。
//
// 连接成功后自动发送认证帧（BotId + Secret）。重复调用时仅告警并返回 nil。
// 认证失败时返回错误，但 wsManager 会在后台按指数退避自动重连。
func (c *WsClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		c.logger.Warn("Client already connected")
		return nil
	}
	c.started = true
	c.mu.Unlock()

	c.logger.Info("Establishing WebSocket connection...")
	return c.wsManager.Connect(ctx)
}

// Disconnect 断开 WebSocket 连接，对应 Node disconnect()。
//
// 未启动时仅告警并返回。
func (c *WsClient) Disconnect() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		c.logger.Warn("Client not connected")
		return
	}
	c.started = false
	c.mu.Unlock()

	c.logger.Info("Disconnecting...")
	c.wsManager.Disconnect()
	c.logger.Info("Disconnected")
}

// IsConnected 获取当前连接状态，对应 Node get isConnected。
func (c *WsClient) IsConnected() bool {
	return c.wsManager.IsConnected()
}

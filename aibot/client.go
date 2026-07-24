package aibot

// client.go 对应 Node src/client.ts：WsClient 核心客户端。
//
// 任务 15：WsClient 结构 + NewWsClient（兜底默认值）+ Connect/Disconnect/IsConnected + 回调桥接。
// 后续任务（17/22/25/26/27）补全 Reply/SendMessage/UploadMedia/DownloadFile 等。

import (
	"context"
	"encoding/json"
	"errors"
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
	apiClient      *WeComApiClient       // HTTP API 客户端（仅文件下载），对应 Node apiClient

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

	// 初始化 API 客户端（仅用于文件下载），对应 Node new WeComApiClient(logger, requestTimeout)
	c.apiClient = NewWeComApiClient(c.logger, opts.RequestTimeout)

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

// ========== 回复消息 ==========

// ErrReplySkipped 非阻塞回复被跳过（上一条同 reqId 的回复尚未收到 ack）。
//
// ReplyStreamNonBlocking 在上一条回复未 ack 且当前帧非最终帧时返回此错误。
var ErrReplySkipped = errors.New("reply skipped: previous reply for this req_id has not been acknowledged yet")

// Reply 通过 WebSocket 通道发送回复消息（通用方法），对应 Node reply。
//
// 首参为 WsFrameHeaders（调用方传 frame.Headers），透传 headers.req_id。
// cmd 为空时由 WsConnectionManager.SendReply 兜底为 WsCmd.Response。
// 阻塞至收到服务端回执，返回回执帧（errcode 非 0 时同时返回帧与错误）。
func (c *WsClient) Reply(frame types.WsFrameHeaders, body any, cmd string) (*types.WsFrame[json.RawMessage], error) {
	return c.wsManager.SendReply(frame, body, cmd)
}

// ReplyStream 发送流式文本回复，对应 Node replyStream。
//
// 同一 streamId 的多次调用刷新内容；finish=true 结束流式消息。
// msgItem 仅在 finish=true 时附带（图文混排项）；feedback 仅在首次回复时设置。
// 阻塞至收到回执，返回回执帧。
func (c *WsClient) ReplyStream(frame types.WsFrameHeaders, streamId, content string, finish bool, msgItem any, feedback any) (*types.WsFrame[json.RawMessage], error) {
	stream := map[string]any{
		"id":      streamId,
		"finish":  finish,
		"content": content,
	}
	// msg_item 仅在 finish=true 时支持
	if finish && msgItem != nil {
		stream["msg_item"] = msgItem
	}
	// feedback 仅在首次回复时设置
	if feedback != nil {
		stream["feedback"] = feedback
	}
	body := map[string]any{
		"msgtype": "stream",
		"stream":  stream,
	}
	return c.Reply(frame, body, types.WsCmd.Response)
}

// ReplyStreamNonBlocking 非阻塞流式文本回复，对应 Node replyStreamNonBlocking。
//
// 若上一条同 reqId 的回复尚未收到 ack 且当前帧非最终帧（finish=false），则跳过本次发送，
// 返回 ErrReplySkipped，避免流式中间帧排队积压导致延迟。
// finish=true 的最终帧始终保证发送（走正常队列）。
func (c *WsClient) ReplyStreamNonBlocking(frame types.WsFrameHeaders, streamId, content string, finish bool, msgItem any, feedback any) (*types.WsFrame[json.RawMessage], error) {
	// finish=true 的最终帧必须发送，不做跳过判断
	if !finish && c.HasPendingReplyAck(frame) {
		return nil, ErrReplySkipped
	}
	return c.ReplyStream(frame, streamId, content, finish, msgItem, feedback)
}

// HasPendingReplyAck 检查指定帧是否有未完成的 ack（即上一条回复还未收到回执），
// 对应 Node hasPendingReplyAck。
//
// 用于流式场景：调用方可据此决定是否跳过当前中间帧，避免排队积压。
func (c *WsClient) HasPendingReplyAck(frame types.WsFrameHeaders) bool {
	return c.wsManager.HasPendingAck(frame.ReqId)
}

// ========== 文件下载 ==========

// DownloadFile 下载文件并使用 AES 密钥解密，对应 Node downloadFile。
//
// url 为文件下载地址；aesKey 取自消息体中的 image.aeskey / file.aeskey（Base64 编码）。
// aesKey 为空时直接返回未解密的原始数据（并告警）。返回解密后的字节与文件名。
func (c *WsClient) DownloadFile(url, aesKey string) ([]byte, string, error) {
	c.logger.Debug(fmt.Sprintf("[plugin] downloadFile: url=%s, hasAesKey=%t", url, aesKey != ""))
	c.logger.Info("Downloading and decrypting file...")

	// 下载加密的文件数据
	encrypted, filename, err := c.apiClient.DownloadFileRaw(url)
	if err != nil {
		c.logger.Error(fmt.Sprintf("File download/decrypt failed: %s", err.Error()))
		return nil, "", err
	}

	// 没有提供 aesKey，直接返回原始数据
	if aesKey == "" {
		c.logger.Warn("No aesKey provided, returning raw file data")
		return encrypted, filename, nil
	}

	// 使用独立的解密模块进行 AES-256-CBC 解密
	decrypted, err := DecryptFile(encrypted, aesKey)
	if err != nil {
		c.logger.Error(fmt.Sprintf("File download/decrypt failed: %s", err.Error()))
		return nil, "", err
	}

	c.logger.Info("File downloaded and decrypted successfully")
	return decrypted, filename, nil
}

// Api 返回内部 WeComApiClient 实例，对应 Node get api()（供文件下载等高级用途）。
func (c *WsClient) Api() *WeComApiClient {
	return c.apiClient
}

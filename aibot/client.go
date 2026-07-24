package aibot

// client.go 对应 Node src/client.ts：WsClient 核心客户端。
//
// 任务 15：WsClient 结构 + NewWsClient（兜底默认值）+ Connect/Disconnect/IsConnected + 回调桥接。
// 后续任务（17/22/25/26/27）补全 Reply/SendMessage/UploadMedia/DownloadFile 等。

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

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

// ReplyStreamWithCardOptions ReplyStreamWithCard 的可选项，对应 Node replyStreamWithCard 的 options 参数
// （Node 为内联匿名对象，Go 用命名结构体承载 4 个可选字段）。
type ReplyStreamWithCardOptions struct {
	MsgItem        []types.ReplyMsgItem // 图文混排项（仅 finish=true 时有效）
	StreamFeedback *types.ReplyFeedback // 流式消息反馈信息（首次回复时设置）
	TemplateCard   *types.TemplateCard  // 模板卡片内容（同一消息只能回复一次）
	CardFeedback   *types.ReplyFeedback // 模板卡片反馈信息
}

// ReplyWelcome 发送欢迎语回复，对应 Node replyWelcome。
//
// 需使用对应事件（如 enter_chat）的 req_id，frame 应来自触发欢迎语的事件帧；
// 收到事件回调后须在 5 秒内发送。body 为 WelcomeTextReplyBody 或 WelcomeTemplateCardReplyBody。
func (c *WsClient) ReplyWelcome(frame types.WsFrameHeaders, body any) (*types.WsFrame[json.RawMessage], error) {
	return c.Reply(frame, body, types.WsCmd.ResponseWelcome)
}

// ReplyTemplateCard 回复模板卡片消息，对应 Node replyTemplateCard。
//
// 收到消息回调或进入会话事件后可回复模板卡片；feedback 非空时合并到卡片（不修改调用方原始 card）。
func (c *WsClient) ReplyTemplateCard(frame types.WsFrameHeaders, card types.TemplateCard, feedback *types.ReplyFeedback) (*types.WsFrame[json.RawMessage], error) {
	// feedback 非空时合并到 card（card 为值拷贝，不影响调用方原始结构）
	if feedback != nil {
		card.Feedback = feedback
	}
	body := types.TemplateCardReplyBody{
		MsgType:      "template_card",
		TemplateCard: card,
	}
	return c.Reply(frame, body, types.WsCmd.Response)
}

// ReplyStreamWithCard 发送流式消息 + 模板卡片组合回复，对应 Node replyStreamWithCard。
//
// 首次回复必须返回 stream.id；template_card 可首次或后续回复，但同一消息只能回复一次。
// msg_item 仅 finish=true 时附带；streamFeedback 首次回复时设置；templateCard 非空时附带（cardFeedback 合并）。
func (c *WsClient) ReplyStreamWithCard(frame types.WsFrameHeaders, streamId, content string, finish bool, opts ReplyStreamWithCardOptions) (*types.WsFrame[json.RawMessage], error) {
	stream := types.StreamReply{
		Id:      streamId,
		Finish:  finish,
		Content: content,
	}
	// msg_item 仅在 finish=true 时支持
	if finish && len(opts.MsgItem) > 0 {
		stream.MsgItem = opts.MsgItem
	}
	// 流式消息反馈仅在首次回复时设置
	if opts.StreamFeedback != nil {
		stream.Feedback = opts.StreamFeedback
	}

	body := types.StreamWithTemplateCardReplyBody{
		MsgType: "stream_with_template_card",
		Stream:  stream,
	}

	// template_card 非空时附带（拷贝后合并 cardFeedback，不修改调用方原始 card）
	if opts.TemplateCard != nil {
		card := *opts.TemplateCard
		if opts.CardFeedback != nil {
			card.Feedback = opts.CardFeedback
		}
		body.TemplateCard = &card
	}

	return c.Reply(frame, body, types.WsCmd.Response)
}

// UpdateTemplateCard 更新模板卡片，对应 Node updateTemplateCard。
//
// 需使用对应事件（template_card_event）的 req_id，frame 应来自触发更新的事件帧；
// 收到事件回调后须在 5 秒内发送。card.TaskId 须与回调收到的 task_id 一致。
// userIds 非空时仅替换指定用户，否则替换所有用户。
func (c *WsClient) UpdateTemplateCard(frame types.WsFrameHeaders, card types.TemplateCard, userIds []string) (*types.WsFrame[json.RawMessage], error) {
	body := types.UpdateTemplateCardBody{
		ResponseType: "update_template_card",
		TemplateCard: card,
	}
	if len(userIds) > 0 {
		body.UserIds = userIds
	}
	return c.Reply(frame, body, types.WsCmd.ResponseUpdate)
}

// ========== 主动发送与媒体回复 ==========

// VideoOptions 视频消息可选参数（仅 mediaType=video 生效），对应 Node replyMedia/sendMediaMessage 的 videoOptions。
type VideoOptions struct {
	Title       string // 视频标题（≤128 字节，超出截断）
	Description string // 视频描述（≤512 字节，超出截断）
}

// SendMessage 向指定会话主动发送消息，对应 Node sendMessage。
//
// 无需依赖收到的回调帧；生成新 reqId，将 chatid 合并进 body 后通过 aibot_send_msg 通道发送。
// body 为 SendMarkdownMsgBody / SendTemplateCardMsgBody / SendMediaMsgBody。
func (c *WsClient) SendMessage(chatid string, body any) (*types.WsFrame[json.RawMessage], error) {
	reqId := GenerateReqId(types.WsCmd.SendMsg)
	fullBody, err := mergeChatId(chatid, body)
	if err != nil {
		return nil, fmt.Errorf("sendMessage: marshal body failed: %s", err.Error())
	}
	return c.wsManager.SendReply(types.WsFrameHeaders{ReqId: reqId}, fullBody, types.WsCmd.SendMsg)
}

// SendMediaMessage 主动发送媒体消息，对应 Node sendMediaMessage。
//
// 通过 aibot_send_msg 主动推送通道发送媒体消息（file/image/voice/video）。
// videoOpts 仅 mediaType=video 时生效（设置 title/description）。
func (c *WsClient) SendMediaMessage(chatid string, mediaType types.WeComMediaType, mediaId string, videoOpts *VideoOptions) (*types.WsFrame[json.RawMessage], error) {
	body := buildMediaBody(mediaType, mediaId, videoOpts)
	return c.SendMessage(chatid, body)
}

// ReplyMedia 被动回复媒体消息，对应 Node replyMedia。
//
// 通过 aibot_respond_msg 被动回复通道发送媒体消息（file/image/voice/video）。
// frame 为收到的原始帧（透传 req_id）；videoOpts 仅 mediaType=video 时生效。
func (c *WsClient) ReplyMedia(frame types.WsFrameHeaders, mediaType types.WeComMediaType, mediaId string, videoOpts *VideoOptions) (*types.WsFrame[json.RawMessage], error) {
	body := buildMediaBody(mediaType, mediaId, videoOpts)
	return c.Reply(frame, body, types.WsCmd.Response)
}

// buildMediaBody 构造媒体消息体（仅设置与 mediaType 匹配的字段），对应 Node replyMedia/sendMediaMessage 的 body 组装。
//
// Node 用动态键 `[mediaType]: mediaContent`，Go 用 switch 设置对应指针字段，产出等价 JSON。
func buildMediaBody(mediaType types.WeComMediaType, mediaId string, videoOpts *VideoOptions) types.SendMediaMsgBody {
	body := types.SendMediaMsgBody{MsgType: mediaType}
	switch mediaType {
	case types.WeComMediaFile:
		body.File = &types.SendMediaContent{MediaId: mediaId}
	case types.WeComMediaImage:
		body.Image = &types.SendMediaContent{MediaId: mediaId}
	case types.WeComMediaVoice:
		body.Voice = &types.SendMediaContent{MediaId: mediaId}
	case types.WeComMediaVideo:
		video := &types.SendVideoContent{MediaId: mediaId}
		if videoOpts != nil {
			video.Title = videoOpts.Title
			video.Description = videoOpts.Description
		}
		body.Video = video
	}
	return body
}

// mergeChatId 将 chatid 合并进 body（对应 Node `{ chatid, ...body }`）。
//
// body 序列化为 map 后注入 chatid 字段，避免修改原 body 类型。
func mergeChatId(chatid string, body any) (map[string]any, error) {
	full := map[string]any{}
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		if string(data) != "null" {
			if err := json.Unmarshal(data, &full); err != nil {
				return nil, err
			}
		}
	}
	full["chatid"] = chatid
	return full, nil
}

// ========== 上传临时素材 ==========

// uploadChunkSize 单个分片大小上限（Base64 编码前），对应 Node CHUNK_SIZE = 512KB。
const uploadChunkSize = 512 * 1024

// uploadMaxChunkRetries 单个分片最大重试次数（不含首次），对应 Node MAX_CHUNK_RETRIES = 2。
const uploadMaxChunkRetries = 2

// UploadMedia 上传临时素材（三步分片上传），对应 Node uploadMedia。
//
// 流程：init → chunk×N → finish。单分片 512KB（Base64 编码前），最多 100 个分片（超出拒绝）。
// 分片上传支持动态并发（≤4/3/2，按分片数）与单片重试（2 次，500ms×n 退避）。
// 返回 media_id（3 天内有效）。
func (c *WsClient) UploadMedia(fileBuffer []byte, opts types.UploadMediaOptions) (*types.UploadMediaFinishResult, error) {
	mediaType := opts.Type
	filename := opts.Filename
	totalSize := len(fileBuffer)
	totalChunks := totalSize / uploadChunkSize
	if totalSize%uploadChunkSize != 0 {
		totalChunks++
	}

	// 超大文件拒绝（最多 100 个分片，约 50MB）
	if totalChunks > 100 {
		return nil, fmt.Errorf("File too large: %d chunks exceeds maximum of 100 chunks (max ~50MB)", totalChunks)
	}

	// 计算文件 MD5
	md5Sum := md5.Sum(fileBuffer)
	md5Hex := hex.EncodeToString(md5Sum[:])

	c.logger.Info(fmt.Sprintf("Uploading media: type=%s, filename=%s, size=%d, chunks=%d", mediaType, filename, totalSize, totalChunks))

	// Step 1: 初始化上传
	initReqId := GenerateReqId(types.WsCmd.UploadMediaInit)
	initResult, err := c.wsManager.SendReply(types.WsFrameHeaders{ReqId: initReqId}, types.UploadMediaInitBody{
		Type:        mediaType,
		Filename:    filename,
		TotalSize:   totalSize,
		TotalChunks: totalChunks,
		Md5:         md5Hex,
	}, types.WsCmd.UploadMediaInit)
	if err != nil {
		return nil, err
	}
	var initBody types.UploadMediaInitResult
	if err := json.Unmarshal(initResult.Body, &initBody); err != nil {
		return nil, fmt.Errorf("Upload init failed: parse response: %s", err.Error())
	}
	if initBody.UploadId == "" {
		return nil, fmt.Errorf("Upload init failed: no upload_id returned. Response: %s", string(initResult.Body))
	}
	uploadId := initBody.UploadId
	c.logger.Info(fmt.Sprintf("Upload init success: upload_id=%s", uploadId))

	// 单分片上传（含重试）
	uploadChunk := func(chunkIndex int) error {
		start := chunkIndex * uploadChunkSize
		end := min(start+uploadChunkSize, totalSize)
		chunk := fileBuffer[start:end]
		base64Data := base64.StdEncoding.EncodeToString(chunk)

		var lastErr error
		for attempt := 0; attempt <= uploadMaxChunkRetries; attempt++ {
			chunkReqId := GenerateReqId(types.WsCmd.UploadMediaChunk)
			_, err := c.wsManager.SendReply(types.WsFrameHeaders{ReqId: chunkReqId}, types.UploadMediaChunkBody{
				UploadId:   uploadId,
				ChunkIndex: chunkIndex,
				Base64Data: base64Data,
			}, types.WsCmd.UploadMediaChunk)
			if err == nil {
				c.logger.Debug(fmt.Sprintf("Uploaded chunk %d/%d (%d bytes)", chunkIndex+1, totalChunks, len(chunk)))
				return nil
			}
			lastErr = err
			if attempt < uploadMaxChunkRetries {
				delay := time.Duration(500*(attempt+1)) * time.Millisecond
				c.logger.Warn(fmt.Sprintf("Chunk %d upload failed (attempt %d/%d), retrying in %dms... error: %s", chunkIndex, attempt+1, uploadMaxChunkRetries+1, 500*(attempt+1), err.Error()))
				time.Sleep(delay)
			}
		}
		return fmt.Errorf("Chunk %d upload failed after %d attempts: %s", chunkIndex, uploadMaxChunkRetries+1, lastErr.Error())
	}

	// Step 2: 分片上传
	if totalChunks <= 1 {
		// 单分片直接上传（totalChunks=0 即空文件时也上传一个空分片，对应 Node uploadChunk(0)）
		if err := uploadChunk(0); err != nil {
			return nil, err
		}
	} else {
		// 多分片并发上传：动态并发数（≤4 分片全并发；5~10 并发 3；>10 并发 2）
		maxConcurrency := 2
		switch {
		case totalChunks <= 4:
			maxConcurrency = totalChunks
		case totalChunks <= 10:
			maxConcurrency = 3
		}
		workerCount := min(maxConcurrency, totalChunks)
		c.logger.Debug(fmt.Sprintf("Upload concurrency: %d workers for %d chunks", workerCount, totalChunks))

		idxCh := make(chan int)
		go func() {
			for i := 0; i < totalChunks; i++ {
				idxCh <- i
			}
			close(idxCh)
		}()

		var (
			wg        sync.WaitGroup
			errMu     sync.Mutex
			failCount int
			firstErr  error
		)
		for range workerCount {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for idx := range idxCh {
					if err := uploadChunk(idx); err != nil {
						errMu.Lock()
						failCount++
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
					}
				}
			}()
		}
		wg.Wait()
		if failCount > 0 {
			return nil, fmt.Errorf("Upload failed: %d chunk(s) failed. First error: %s", failCount, firstErr.Error())
		}
	}

	c.logger.Info(fmt.Sprintf("All %d chunks uploaded, finishing...", totalChunks))

	// Step 3: 完成上传
	finishReqId := GenerateReqId(types.WsCmd.UploadMediaFinish)
	finishResult, err := c.wsManager.SendReply(types.WsFrameHeaders{ReqId: finishReqId}, types.UploadMediaFinishBody{
		UploadId: uploadId,
	}, types.WsCmd.UploadMediaFinish)
	if err != nil {
		return nil, err
	}
	var finishBody struct {
		Type      types.WeComMediaType `json:"type"`
		MediaId   string               `json:"media_id"`
		CreatedAt string               `json:"created_at"`
	}
	if err := json.Unmarshal(finishResult.Body, &finishBody); err != nil {
		return nil, fmt.Errorf("Upload finish failed: parse response: %s", err.Error())
	}
	if finishBody.MediaId == "" {
		return nil, fmt.Errorf("Upload finish failed: no media_id returned. Response: %s", string(finishResult.Body))
	}

	// type 缺省回退为入参 type；created_at 缺省回退为当前时间（ISO 8601）
	resultType := finishBody.Type
	if resultType == "" {
		resultType = mediaType
	}
	createdAt := finishBody.CreatedAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	c.logger.Info(fmt.Sprintf("Upload complete: media_id=%s, type=%s", finishBody.MediaId, resultType))

	return &types.UploadMediaFinishResult{
		Type:      resultType,
		MediaId:   finishBody.MediaId,
		CreatedAt: createdAt,
	}, nil
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

package types

// Package types 定义企业微信智能机器人 Go SDK 的公开数据类型，1:1 镜像 Node SDK 的 src/types/。
//
// 本文件对应 Node types/index.ts，集中声明本子包各文件的公开符号引用，
// 确保 re-export 完整且编译器可验证。Go 同包文件共享命名空间，
// 无需显式 re-export；此处仅作文档与校验用途。

// ========== common.go ==========

var _ Logger = (Logger)(nil) // 确认 Logger 接口存在

var (
	_ WsAuthFailureError        // 确认 WsAuthFailureError 存在
	_ WsReconnectExhaustedError // 确认 WsReconnectExhaustedError 存在
)

// ========== config.go ==========

var _ WsClientOptions // 确认 WsClientOptions 存在

// ========== api.go ==========

var (
	_ = WsCmd.Subscribe // WsCmd 常量
	_ = WsCmd.Heartbeat
	_ = WsCmd.Response
	_ = WsCmd.ResponseWelcome
	_ = WsCmd.ResponseUpdate
	_ = WsCmd.SendMsg
	_ = WsCmd.UploadMediaInit
	_ = WsCmd.UploadMediaChunk
	_ = WsCmd.UploadMediaFinish
	_ = WsCmd.Callback
	_ = WsCmd.EventCallback
)

var (
	_ WsFrameHeaders // 确认 WsFrameHeaders 存在
	_ WsFrame[any]   // 确认 WsFrame[T] 存在
)

// ========== api.go：模板卡片 + 回复体（任务 24）==========

var (
	_ = TemplateCardType.TextNotice // TemplateCardType 常量
	_ = TemplateCardType.NewsNotice
	_ = TemplateCardType.ButtonInteraction
	_ = TemplateCardType.VoteInteraction
	_ = TemplateCardType.MultipleInteraction
)

var (
	_ ReplyMsgItem // 回复通用子结构
	_ ReplyFeedback
	_ TemplateCard // 模板卡片及子结构
	_ TemplateCardSource
	_ TemplateCardActionMenu
	_ TemplateCardMainTitle
	_ TemplateCardEmphasisContent
	_ TemplateCardQuoteArea
	_ TemplateCardHorizontalContent
	_ TemplateCardJumpAction
	_ TemplateCardAction
	_ TemplateCardVerticalContent
	_ TemplateCardImage
	_ TemplateCardImageTextArea
	_ TemplateCardSubmitButton
	_ TemplateCardSelectionItem
	_ TemplateCardButton
	_ TemplateCardCheckbox
	_ StreamReply // 回复消息体
	_ StreamReplyBody
	_ WelcomeTextReplyBody
	_ WelcomeTemplateCardReplyBody
	_ TemplateCardReplyBody
	_ StreamWithTemplateCardReplyBody
	_ UpdateTemplateCardBody
)

// ========== api.go：媒体类型 + 主动发送体（任务 26）==========

var (
	_ = WeComMediaFile // WeComMediaType 常量
	_ = WeComMediaImage
	_ = WeComMediaVoice
	_ = WeComMediaVideo
)

var (
	_ WeComMediaType   // 媒体类型
	_ SendMediaContent // 媒体消息子结构
	_ SendVideoContent
	_ SendMediaMsgBody // 主动发送体
	_ SendMarkdownMsgBody
	_ SendTemplateCardMsgBody
)

// ========== api.go：上传临时素材（任务 27）==========

var (
	_ UploadMediaOptions  // 上传选项
	_ UploadMediaInitBody // 上传请求/响应 body
	_ UploadMediaInitResult
	_ UploadMediaChunkBody
	_ UploadMediaFinishBody
	_ UploadMediaFinishResult // 上传结果
)

// ========== message.go ==========

var (
	_ = MessageType.Text // MessageType 常量
	_ = MessageType.Image
	_ = MessageType.Mixed
	_ = MessageType.Voice
	_ = MessageType.File
	_ = MessageType.Video
)

var (
	_ MessageFrom // 消息子结构
	_ TextContent
	_ ImageContent
	_ VoiceContent
	_ FileContent
	_ VideoContent
	_ MixedMsgItem
	_ MixedContent
	_ QuoteContent
	_ BaseMessage // 基础消息
	_ TextMessage // 具体消息类型
	_ ImageMessage
	_ MixedMessage
	_ VoiceMessage
	_ FileMessage
	_ VideoMessage
)

// ========== event.go ==========

var (
	_ = EventType.EnterChat // EventType 常量
	_ = EventType.TemplateCardEvent
	_ = EventType.FeedbackEvent
	_ = EventType.Disconnected
)

var (
	_ EventFrom // 事件子结构
	_ EnterChatEvent
	_ TemplateCardEventData
	_ FeedbackEventData
	_ DisconnectedEventData
	_ EventMessage // 事件消息
)

var _ EventContent = EnterChatEvent{} // 确认 EventContent 接口实现

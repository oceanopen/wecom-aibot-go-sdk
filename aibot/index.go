package aibot

// Package aibot 是企业微信智能机器人 Go SDK，对应 Node SDK @wecom/aibot-node-sdk。
//
// 文件结构、命名、拆分 1:1 镜像 Node 的 src/。入口文件 index.go（对应 Node src/index.ts）
// 重新导出 aibot/types 子包的全部公开符号，使用户仅需
//
//	import "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
//
// 即可使用 aibot.WsClient、aibot.WsFrame[*aibot.TextMessage] 等。
//
// 详细任务计划与执行顺序见项目根目录 task.md。

import "github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"

// ========== types/common.go ==========

// Logger 日志接口，重新导出 types.Logger。
type Logger = types.Logger

// WsAuthFailureError 认证失败重试次数用尽错误，重新导出 types.WsAuthFailureError。
type WsAuthFailureError = types.WsAuthFailureError

// WsReconnectExhaustedError 连接断开重连次数用尽错误，重新导出 types.WsReconnectExhaustedError。
type WsReconnectExhaustedError = types.WsReconnectExhaustedError

// NewWsAuthFailureError 构造认证失败耗尽错误，重新导出 types.NewWsAuthFailureError。
var NewWsAuthFailureError = types.NewWsAuthFailureError

// NewWsReconnectExhaustedError 构造重连耗尽错误，重新导出 types.NewWsReconnectExhaustedError。
var NewWsReconnectExhaustedError = types.NewWsReconnectExhaustedError

// WsAuthFailureCode 认证失败错误码，重新导出 types.WsAuthFailureCode。
var WsAuthFailureCode = types.WsAuthFailureCode

// WsReconnectExhaustedCode 重连耗尽错误码，重新导出 types.WsReconnectExhaustedCode。
var WsReconnectExhaustedCode = types.WsReconnectExhaustedCode

// ========== types/config.go ==========

// WsClientOptions WSClient 配置选项，重新导出 types.WsClientOptions。
type WsClientOptions = types.WsClientOptions

// WsOptions 底层 WebSocket 连接选项，重新导出 types.WsOptions。
type WsOptions = types.WsOptions

// ========== types/api.go ==========

// WsCmd WebSocket 命令类型常量，重新导出 types.WsCmd。
var WsCmd = types.WsCmd

// WsFrameHeaders WebSocket 帧头信息，重新导出 types.WsFrameHeaders。
type WsFrameHeaders = types.WsFrameHeaders

// WsFrame WebSocket 帧结构（泛型类型别名，Go 1.24），重新导出 types.WsFrame[T]。
type WsFrame[T any] = types.WsFrame[T]

// ========== types/message.go ==========

// MessageType 消息类型常量，重新导出 types.MessageType。
var MessageType = types.MessageType

// MessageFrom 消息发送者信息，重新导出 types.MessageFrom。
type MessageFrom = types.MessageFrom

// TextContent 文本结构体，重新导出 types.TextContent。
type TextContent = types.TextContent

// ImageContent 图片结构体，重新导出 types.ImageContent。
type ImageContent = types.ImageContent

// VoiceContent 语音结构体，重新导出 types.VoiceContent。
type VoiceContent = types.VoiceContent

// FileContent 文件结构体，重新导出 types.FileContent。
type FileContent = types.FileContent

// VideoContent 视频结构体，重新导出 types.VideoContent。
type VideoContent = types.VideoContent

// MixedMsgItem 图文混排子项，重新导出 types.MixedMsgItem。
type MixedMsgItem = types.MixedMsgItem

// MixedContent 图文混排结构体，重新导出 types.MixedContent。
type MixedContent = types.MixedContent

// QuoteContent 引用结构体，重新导出 types.QuoteContent。
type QuoteContent = types.QuoteContent

// BaseMessage 基础消息结构，重新导出 types.BaseMessage。
type BaseMessage = types.BaseMessage

// TextMessage 文本消息，重新导出 types.TextMessage。
type TextMessage = types.TextMessage

// ImageMessage 图片消息，重新导出 types.ImageMessage。
type ImageMessage = types.ImageMessage

// MixedMessage 图文混排消息，重新导出 types.MixedMessage。
type MixedMessage = types.MixedMessage

// VoiceMessage 语音消息，重新导出 types.VoiceMessage。
type VoiceMessage = types.VoiceMessage

// FileMessage 文件消息，重新导出 types.FileMessage。
type FileMessage = types.FileMessage

// VideoMessage 视频消息，重新导出 types.VideoMessage。
type VideoMessage = types.VideoMessage

// ========== types/event.go ==========

// EventType 事件类型常量，重新导出 types.EventType。
var EventType = types.EventType

// EventFrom 事件发送者信息，重新导出 types.EventFrom。
type EventFrom = types.EventFrom

// EventContent 事件内容接口，重新导出 types.EventContent。
type EventContent = types.EventContent

// EnterChatEvent 进入会话事件，重新导出 types.EnterChatEvent。
type EnterChatEvent = types.EnterChatEvent

// TemplateCardEventData 模板卡片事件，重新导出 types.TemplateCardEventData。
type TemplateCardEventData = types.TemplateCardEventData

// FeedbackEventData 用户反馈事件，重新导出 types.FeedbackEventData。
type FeedbackEventData = types.FeedbackEventData

// DisconnectedEventData 连接断开事件，重新导出 types.DisconnectedEventData。
type DisconnectedEventData = types.DisconnectedEventData

// EventMessage 事件回调消息结构，重新导出 types.EventMessage。
type EventMessage = types.EventMessage

// ========== types/api.go：回复通用子结构（任务 24）==========

// ReplyMsgItem 回复图文混排子项，重新导出 types.ReplyMsgItem。
type ReplyMsgItem = types.ReplyMsgItem

// ReplyFeedback 回复反馈信息，重新导出 types.ReplyFeedback。
type ReplyFeedback = types.ReplyFeedback

// ========== types/api.go：模板卡片及子结构（任务 24）==========

// TemplateCardType 卡片类型常量，重新导出 types.TemplateCardType。
var TemplateCardType = types.TemplateCardType

// TemplateCard 模板卡片结构，重新导出 types.TemplateCard。
type TemplateCard = types.TemplateCard

// TemplateCardSource 卡片来源样式，重新导出 types.TemplateCardSource。
type TemplateCardSource = types.TemplateCardSource

// TemplateCardActionMenu 卡片右上角更多操作，重新导出 types.TemplateCardActionMenu。
type TemplateCardActionMenu = types.TemplateCardActionMenu

// TemplateCardMainTitle 模板卡片主标题，重新导出 types.TemplateCardMainTitle。
type TemplateCardMainTitle = types.TemplateCardMainTitle

// TemplateCardEmphasisContent 关键数据样式，重新导出 types.TemplateCardEmphasisContent。
type TemplateCardEmphasisContent = types.TemplateCardEmphasisContent

// TemplateCardQuoteArea 引用文献样式，重新导出 types.TemplateCardQuoteArea。
type TemplateCardQuoteArea = types.TemplateCardQuoteArea

// TemplateCardHorizontalContent 二级标题+文本列表项，重新导出 types.TemplateCardHorizontalContent。
type TemplateCardHorizontalContent = types.TemplateCardHorizontalContent

// TemplateCardJumpAction 跳转指引样式，重新导出 types.TemplateCardJumpAction。
type TemplateCardJumpAction = types.TemplateCardJumpAction

// TemplateCardAction 整体卡片点击跳转事件，重新导出 types.TemplateCardAction。
type TemplateCardAction = types.TemplateCardAction

// TemplateCardVerticalContent 卡片二级垂直内容，重新导出 types.TemplateCardVerticalContent。
type TemplateCardVerticalContent = types.TemplateCardVerticalContent

// TemplateCardImage 图片样式，重新导出 types.TemplateCardImage。
type TemplateCardImage = types.TemplateCardImage

// TemplateCardImageTextArea 左图右文样式，重新导出 types.TemplateCardImageTextArea。
type TemplateCardImageTextArea = types.TemplateCardImageTextArea

// TemplateCardSubmitButton 提交按钮样式，重新导出 types.TemplateCardSubmitButton。
type TemplateCardSubmitButton = types.TemplateCardSubmitButton

// TemplateCardSelectionItem 下拉式选择器，重新导出 types.TemplateCardSelectionItem。
type TemplateCardSelectionItem = types.TemplateCardSelectionItem

// TemplateCardButton 模板卡片按钮，重新导出 types.TemplateCardButton。
type TemplateCardButton = types.TemplateCardButton

// TemplateCardCheckbox 选择题样式，重新导出 types.TemplateCardCheckbox。
type TemplateCardCheckbox = types.TemplateCardCheckbox

// ========== types/api.go：回复消息体（任务 24）==========

// StreamReply 流式回复内容对象，重新导出 types.StreamReply。
type StreamReply = types.StreamReply

// StreamReplyBody 流式回复消息体，重新导出 types.StreamReplyBody。
type StreamReplyBody = types.StreamReplyBody

// WelcomeTextReplyBody 欢迎语文本回复体，重新导出 types.WelcomeTextReplyBody。
type WelcomeTextReplyBody = types.WelcomeTextReplyBody

// WelcomeTemplateCardReplyBody 欢迎语模板卡片回复体，重新导出 types.WelcomeTemplateCardReplyBody。
type WelcomeTemplateCardReplyBody = types.WelcomeTemplateCardReplyBody

// TemplateCardReplyBody 模板卡片回复体，重新导出 types.TemplateCardReplyBody。
type TemplateCardReplyBody = types.TemplateCardReplyBody

// StreamWithTemplateCardReplyBody 流式+模板卡片组合回复体，重新导出 types.StreamWithTemplateCardReplyBody。
type StreamWithTemplateCardReplyBody = types.StreamWithTemplateCardReplyBody

// UpdateTemplateCardBody 更新模板卡片消息体，重新导出 types.UpdateTemplateCardBody。
type UpdateTemplateCardBody = types.UpdateTemplateCardBody

// ========== types/api.go：媒体类型 + 主动发送体（任务 26）==========

// WeComMediaType 企业微信媒体类型，重新导出 types.WeComMediaType。
type WeComMediaType = types.WeComMediaType

// 媒体类型取值常量，重新导出 types.WeComMedia*。
const (
	WeComMediaFile  = types.WeComMediaFile
	WeComMediaImage = types.WeComMediaImage
	WeComMediaVoice = types.WeComMediaVoice
	WeComMediaVideo = types.WeComMediaVideo
)

// SendMediaContent 媒体消息内容，重新导出 types.SendMediaContent。
type SendMediaContent = types.SendMediaContent

// SendVideoContent 视频消息内容，重新导出 types.SendVideoContent。
type SendVideoContent = types.SendVideoContent

// SendMediaMsgBody 媒体消息发送体，重新导出 types.SendMediaMsgBody。
type SendMediaMsgBody = types.SendMediaMsgBody

// SendMarkdownMsgBody 主动发送 Markdown 消息体，重新导出 types.SendMarkdownMsgBody。
type SendMarkdownMsgBody = types.SendMarkdownMsgBody

// SendTemplateCardMsgBody 主动发送模板卡片消息体，重新导出 types.SendTemplateCardMsgBody。
type SendTemplateCardMsgBody = types.SendTemplateCardMsgBody

// ========== types/api.go：上传临时素材（任务 27）==========

// UploadMediaOptions uploadMedia 选项，重新导出 types.UploadMediaOptions。
type UploadMediaOptions = types.UploadMediaOptions

// UploadMediaInitBody 上传初始化请求体，重新导出 types.UploadMediaInitBody。
type UploadMediaInitBody = types.UploadMediaInitBody

// UploadMediaInitResult 上传初始化响应体，重新导出 types.UploadMediaInitResult。
type UploadMediaInitResult = types.UploadMediaInitResult

// UploadMediaChunkBody 上传分片请求体，重新导出 types.UploadMediaChunkBody。
type UploadMediaChunkBody = types.UploadMediaChunkBody

// UploadMediaFinishBody 完成上传请求体，重新导出 types.UploadMediaFinishBody。
type UploadMediaFinishBody = types.UploadMediaFinishBody

// UploadMediaFinishResult 上传结果，重新导出 types.UploadMediaFinishResult。
type UploadMediaFinishResult = types.UploadMediaFinishResult

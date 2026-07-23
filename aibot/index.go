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

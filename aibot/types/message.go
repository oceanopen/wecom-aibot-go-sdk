package types

// message.go 对应 Node src/types/message.ts：接收消息类型（BaseMessage 及 TextMessage 等）。

// ========== 消息类型常量 ==========

// MessageType 消息类型常量，对应 Node MessageType enum。
var MessageType = struct {
	Text  string // 文本消息
	Image string // 图片消息
	Mixed string // 图文混排消息
	Voice string // 语音消息
	File  string // 文件消息
	Video string // 视频消息
}{
	Text:  "text",
	Image: "image",
	Mixed: "mixed",
	Voice: "voice",
	File:  "file",
	Video: "video",
}

// ========== 消息子结构 ==========

// MessageFrom 消息发送者信息，对应 Node MessageFrom。
type MessageFrom struct {
	UserId string `json:"userid"` // 操作者的 userid
}

// TextContent 文本结构体，对应 Node TextContent。
type TextContent struct {
	Content string `json:"content"` // 文本消息内容
}

// ImageContent 图片结构体，对应 Node ImageContent。
type ImageContent struct {
	Url    string `json:"url"`              // 图片的下载 url（五分钟内有效，已加密）
	AesKey string `json:"aeskey,omitempty"` // 解密密钥，长连接模式下返回，每个下载链接的 aeskey 唯一
}

// VoiceContent 语音结构体，对应 Node VoiceContent。
type VoiceContent struct {
	Content string `json:"content"` // 语音转换成文本的内容
}

// FileContent 文件结构体，对应 Node FileContent。
type FileContent struct {
	Url    string `json:"url"`              // 文件的下载 url（五分钟内有效，已加密）
	AesKey string `json:"aeskey,omitempty"` // 解密密钥，长连接模式下返回，每个下载链接的 aeskey 唯一
}

// VideoContent 视频结构体，对应 Node VideoContent。
type VideoContent struct {
	Url    string `json:"url"`              // 视频的下载 url（五分钟内有效，已加密）
	AesKey string `json:"aeskey,omitempty"` // 解密密钥，长连接模式下返回，每个下载链接的 aeskey 唯一
}

// MixedMsgItem 图文混排子项，对应 Node MixedMsgItem。
type MixedMsgItem struct {
	MsgType string       `json:"msgtype"`         // 图文混排中的类型：text / image
	Text    TextContent  `json:"text,omitempty"`  // 文本内容（msgtype 为 text 时存在）
	Image   ImageContent `json:"image,omitempty"` // 图片内容（msgtype 为 image 时存在）
}

// MixedContent 图文混排结构体，对应 Node MixedContent。
type MixedContent struct {
	MsgItem []MixedMsgItem `json:"msg_item"` // 图文混排消息项列表
}

// QuoteContent 引用结构体，对应 Node QuoteContent。
type QuoteContent struct {
	MsgType string       `json:"msgtype"`         // 引用的类型：text / image / mixed / voice / file
	Text    TextContent  `json:"text,omitempty"`  // 引用的文本内容
	Image   ImageContent `json:"image,omitempty"` // 引用的图片内容
	Mixed   MixedContent `json:"mixed,omitempty"` // 引用的图文混排内容
	Voice   VoiceContent `json:"voice,omitempty"` // 引用的语音内容
	File    FileContent  `json:"file,omitempty"`  // 引用的文件内容
}

// ========== 基础消息与具体消息类型 ==========

// BaseMessage 基础消息结构，对应 Node BaseMessage。
//
// 所有具体消息类型嵌套此结构。Raw 保留服务端返回的原始 JSON 字段（Go 不支持索引签名，
// 用额外字段近似 Node 的 [key: string]: any）。
type BaseMessage struct {
	MsgId       string         `json:"msgid"`                  // 本次回调的唯一性标志，用于事件排重
	AibotId     string         `json:"aibotid"`                // 智能机器人 id
	ChatId      string         `json:"chatid,omitempty"`       // 会话 id，仅群聊类型时返回
	ChatType    string         `json:"chattype"`               // 会话类型：single 单聊, group 群聊
	From        MessageFrom    `json:"from"`                   // 事件触发者信息
	CreateTime  int            `json:"create_time,omitempty"`  // 事件产生的时间戳
	ResponseUrl string         `json:"response_url,omitempty"` // 支持主动回复消息的临时 url
	MsgType     string         `json:"msgtype"`                // 消息类型
	Quote       *QuoteContent  `json:"quote,omitempty"`        // 引用内容（若用户引用了其他消息则有该字段）
	Raw         map[string]any `json:"-"`                      // 原始数据（Node 的 [key: string]: any）
}

// TextMessage 文本消息，对应 Node TextMessage。
type TextMessage struct {
	BaseMessage
	Text TextContent `json:"text"` // 文本消息内容
}

// ImageMessage 图片消息，对应 Node ImageMessage。
type ImageMessage struct {
	BaseMessage
	Image ImageContent `json:"image"` // 图片内容
}

// MixedMessage 图文混排消息，对应 Node MixedMessage。
type MixedMessage struct {
	BaseMessage
	Mixed MixedContent `json:"mixed"` // 图文混排内容
}

// VoiceMessage 语音消息，对应 Node VoiceMessage。
type VoiceMessage struct {
	BaseMessage
	Voice VoiceContent `json:"voice"` // 语音内容
}

// FileMessage 文件消息，对应 Node FileMessage。
type FileMessage struct {
	BaseMessage
	File FileContent `json:"file"` // 文件内容
}

// VideoMessage 视频消息，对应 Node VideoMessage。
type VideoMessage struct {
	BaseMessage
	Video VideoContent `json:"video"` // 视频内容
}

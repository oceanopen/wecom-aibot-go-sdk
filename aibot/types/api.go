package types

// api.go 对应 Node src/types/api.ts：WsCmd 常量、WsFrame[T]、WsFrameHeaders、模板卡片及回复/发送/上传体类型（任务 6/24/26/27 填充）。

// ========== WebSocket 命令类型常量 ==========

// WsCmd WebSocket 命令类型常量，对应 Node WsCmd。
var WsCmd = struct {
	// ========== 开发者 → 企业微信 ==========

	Subscribe         string // 认证订阅
	Heartbeat         string // 心跳
	Response          string // 回复消息
	ResponseWelcome   string // 回复欢迎语
	ResponseUpdate    string // 更新模板卡片
	SendMsg           string // 主动发送消息
	UploadMediaInit   string // 上传临时素材 - 初始化
	UploadMediaChunk  string // 上传临时素材 - 分片上传
	UploadMediaFinish string // 上传临时素材 - 完成上传

	// ========== 企业微信 → 开发者 ==========

	Callback      string // 消息推送回调
	EventCallback string // 事件推送回调
}{
	Subscribe:         "aibot_subscribe",
	Heartbeat:         "ping",
	Response:          "aibot_respond_msg",
	ResponseWelcome:   "aibot_respond_welcome_msg",
	ResponseUpdate:    "aibot_respond_update_msg",
	SendMsg:           "aibot_send_msg",
	UploadMediaInit:   "aibot_upload_media_init",
	UploadMediaChunk:  "aibot_upload_media_chunk",
	UploadMediaFinish: "aibot_upload_media_finish",
	Callback:          "aibot_msg_callback",
	EventCallback:     "aibot_event_callback",
}

// ========== WebSocket 帧结构 ==========

// WsFrameHeaders WebSocket 帧头信息，对应 Node WsFrameHeaders（Pick<WsFrame, 'headers'>）。
//
// 仅包含 req_id，用于 reply / replyStream 等回复方法的参数类型，调用方传 frame.Headers。
type WsFrameHeaders struct {
	ReqId string `json:"req_id"` // 请求 ID，用于关联请求与响应
}

// WsFrame WebSocket 帧结构，对应 Node WsFrame<T>。
//
// 发送和接收统一使用 { cmd, headers, body } 格式：
//   - 认证发送：{ cmd: "aibot_subscribe", headers: { req_id }, body: { secret, bot_id } }
//   - 消息推送：{ cmd: "aibot_msg_callback", headers: { req_id }, body: { msgid, msgtype, ... } }
//   - 事件推送：{ cmd: "aibot_event_callback", headers: { req_id }, body: { event_type, ... } }
//   - 回复消息：{ cmd: "aibot_respond_msg", headers: { req_id }, body: { msgtype, stream: { ... } } }
//   - 回复欢迎语：{ cmd: "aibot_respond_welcome_msg", headers: { req_id }, body: { ... } }
//   - 更新模板卡片：{ cmd: "aibot_respond_update_msg", headers: { req_id }, body: { ... } }
//   - 心跳发送：{ cmd: "ping", headers: { req_id } }
//   - 认证/心跳响应：{ headers: { req_id }, errcode: 0, errmsg: "ok" }
type WsFrame[T any] struct {
	Cmd     string         `json:"cmd,omitempty"`     // 命令类型；认证/心跳响应时可能为空
	Headers WsFrameHeaders `json:"headers"`           // 请求头（含 req_id）
	Body    T              `json:"body,omitempty"`    // 消息体
	ErrCode int            `json:"errcode,omitempty"` // 响应错误码，0 表示成功
	ErrMsg  string         `json:"errmsg,omitempty"`  // 响应错误信息
}

// ========== 回复消息中的通用子结构 ==========

// ReplyMsgItem 回复消息中的图文混排子项，对应 Node ReplyMsgItem。
type ReplyMsgItem struct {
	MsgType string `json:"msgtype"` // 类型，固定值 image
	Image   struct {
		Base64 string `json:"base64"` // Base64 编码的图片数据（编码前 ≤10M，JPG/PNG）
		Md5    string `json:"md5"`    // 图片内容（base64 编码前）的 MD5 值
	} `json:"image"` // 图片内容
}

// ReplyFeedback 回复消息中的反馈信息，对应 Node ReplyFeedback。
type ReplyFeedback struct {
	Id string `json:"id"` // 反馈 ID（≤256 字节，utf-8 编码）
}

// ========== 模板卡片结构体及子结构体 ==========

// TemplateCardType 卡片类型枚举，对应 Node TemplateCardType enum。
var TemplateCardType = struct {
	TextNotice          string // 文本通知模版卡片
	NewsNotice          string // 图文展示模版卡片
	ButtonInteraction   string // 按钮交互模版卡片
	VoteInteraction     string // 投票选择模版卡片
	MultipleInteraction string // 多项选择模版卡片
}{
	TextNotice:          "text_notice",
	NewsNotice:          "news_notice",
	ButtonInteraction:   "button_interaction",
	VoteInteraction:     "vote_interaction",
	MultipleInteraction: "multiple_interaction",
}

// TemplateCardSource 卡片来源样式信息，对应 Node TemplateCardSource。
type TemplateCardSource struct {
	IconUrl   string `json:"icon_url,omitempty"`   // 来源图片的 url
	Desc      string `json:"desc,omitempty"`       // 来源图片描述（≤13 字）
	DescColor int    `json:"desc_color,omitempty"` // 来源文字颜色：0(默认)灰/1 黑/2 红/3 绿
}

// TemplateCardActionMenu 卡片右上角更多操作按钮，对应 Node TemplateCardActionMenu。
type TemplateCardActionMenu struct {
	Desc       string `json:"desc"` // 更多操作界面的描述
	ActionList []struct {
		Text string `json:"text"` // 操作的描述文案
		Key  string `json:"key"`  // 操作 key 值（≤1024 字节，不可重复）
	} `json:"action_list"` // 操作列表，长度 [1, 3]
}

// TemplateCardMainTitle 模板卡片主标题，对应 Node TemplateCardMainTitle。
type TemplateCardMainTitle struct {
	Title string `json:"title,omitempty"` // 一级标题（≤26 字）
	Desc  string `json:"desc,omitempty"`  // 标题辅助信息（≤30 字）
}

// TemplateCardEmphasisContent 关键数据样式，对应 Node TemplateCardEmphasisContent。
type TemplateCardEmphasisContent struct {
	Title string `json:"title,omitempty"` // 关键数据内容（≤10 字）
	Desc  string `json:"desc,omitempty"`  // 关键数据描述（≤15 字）
}

// TemplateCardQuoteArea 引用文献样式，对应 Node TemplateCardQuoteArea。
type TemplateCardQuoteArea struct {
	Type      int    `json:"type,omitempty"`       // 点击事件：0/不填无，1 跳 url，2 跳小程序
	Url       string `json:"url,omitempty"`        // 跳转 url（type=1 必填）
	AppId     string `json:"appid,omitempty"`      // 小程序 appid（type=2 必填）
	PagePath  string `json:"pagepath,omitempty"`   // 小程序 pagepath（type=2 选填）
	Title     string `json:"title,omitempty"`      // 引用文献样式标题
	QuoteText string `json:"quote_text,omitempty"` // 引用文献引用文案
}

// TemplateCardHorizontalContent 二级标题+文本列表项，对应 Node TemplateCardHorizontalContent。
type TemplateCardHorizontalContent struct {
	Type    int    `json:"type,omitempty"`   // 链接类型：0/不填普通文本，1 跳 url，3 跳成员详情
	KeyName string `json:"keyname"`          // 二级标题（≤5 字）
	Value   string `json:"value,omitempty"`  // 二级文本（≤26 字）
	Url     string `json:"url,omitempty"`    // 跳转 url（type=1 必填）
	UserId  string `json:"userid,omitempty"` // 成员详情 userid（type=3 必填）
}

// TemplateCardJumpAction 跳转指引样式，对应 Node TemplateCardJumpAction。
type TemplateCardJumpAction struct {
	Type     int    `json:"type,omitempty"`     // 跳转类型：0/不填非链接，1 跳 url，2 跳小程序，3 智能回复
	Title    string `json:"title"`              // 跳转文案（≤13 字）
	Url      string `json:"url,omitempty"`      // 跳转 url（type=1 必填）
	AppId    string `json:"appid,omitempty"`    // 小程序 appid（type=2 必填）
	PagePath string `json:"pagepath,omitempty"` // 小程序 pagepath（type=2 选填）
	Question string `json:"question,omitempty"` // 智能问答问题（type=3 必填，≤200 字节）
}

// TemplateCardAction 整体卡片的点击跳转事件，对应 Node TemplateCardAction。
//
// Type 为必填字段（无 omitempty），即便为 0 也保留序列化。
type TemplateCardAction struct {
	Type     int    `json:"type"`               // 卡片跳转类型：0 非链接，1 跳 url，2 打开小程序
	Url      string `json:"url,omitempty"`      // 跳转 url（type=1 必填）
	AppId    string `json:"appid,omitempty"`    // 小程序 appid（type=2 必填）
	PagePath string `json:"pagepath,omitempty"` // 小程序 pagepath（type=2 选填）
}

// TemplateCardVerticalContent 卡片二级垂直内容，对应 Node TemplateCardVerticalContent。
type TemplateCardVerticalContent struct {
	Title string `json:"title"`          // 卡片二级标题（≤26 字）
	Desc  string `json:"desc,omitempty"` // 二级普通文本（≤112 字）
}

// TemplateCardImage 图片样式，对应 Node TemplateCardImage。
type TemplateCardImage struct {
	Url         string  `json:"url"`                    // 图片的 url
	AspectRatio float64 `json:"aspect_ratio,omitempty"` // 宽高比（1.3~2.25，不填默认 1.3）
}

// TemplateCardImageTextArea 左图右文样式，对应 Node TemplateCardImageTextArea。
type TemplateCardImageTextArea struct {
	Type     int    `json:"type,omitempty"`     // 点击事件：0/不填无，1 跳 url，2 跳小程序
	Url      string `json:"url,omitempty"`      // 跳转 url（type=1 必填）
	AppId    string `json:"appid,omitempty"`    // 小程序 appid（type=2 必填）
	PagePath string `json:"pagepath,omitempty"` // 小程序 pagepath（type=2 选填）
	Title    string `json:"title,omitempty"`    // 左图右文样式标题
	Desc     string `json:"desc,omitempty"`     // 左图右文样式描述
	ImageUrl string `json:"image_url"`          // 左图右文样式的图片 url
}

// TemplateCardSubmitButton 提交按钮样式，对应 Node TemplateCardSubmitButton。
type TemplateCardSubmitButton struct {
	Text string `json:"text"` // 按钮文案（≤10 字）
	Key  string `json:"key"`  // 提交按钮 key（≤1024 字节）
}

// TemplateCardSelectionItem 下拉式选择器，对应 Node TemplateCardSelectionItem。
type TemplateCardSelectionItem struct {
	QuestionKey string `json:"question_key"`          // 题目 key（≤1024 字节，不可重复）
	Title       string `json:"title,omitempty"`       // 选择器标题（≤13 字）
	Disable     bool   `json:"disable,omitempty"`     // 是否不可选（仅更新模版卡片时有效）
	SelectedId  string `json:"selected_id,omitempty"` // 默认选定的 id（不填或错填默认第一个）
	OptionList  []struct {
		Id   string `json:"id"`   // 选项 id（≤128 字节，不可重复）
		Text string `json:"text"` // 选项文案（≤10 字）
	} `json:"option_list"` // 选项列表，[1, 10]
}

// TemplateCardButton 模板卡片按钮，对应 Node TemplateCardButton。
type TemplateCardButton struct {
	Text  string `json:"text"`            // 按钮文案（≤10 字）
	Style int    `json:"style,omitempty"` // 按钮样式 1~4，不填或错填默认 1
	Key   string `json:"key"`             // 按钮 key 值（≤1024 字节，不可重复）
}

// TemplateCardCheckbox 选择题样式（投票选择），对应 Node TemplateCardCheckbox。
type TemplateCardCheckbox struct {
	QuestionKey string `json:"question_key"`      // 选择题 key 值（≤1024 字节）
	Disable     bool   `json:"disable,omitempty"` // 是否不可选（仅更新模版卡片时有效）
	Mode        int    `json:"mode,omitempty"`    // 选择题模式：0 单选，1 多选，默认 0
	OptionList  []struct {
		Id        string `json:"id"`                   // 选项 id（≤128 字节，不可重复）
		Text      string `json:"text"`                 // 选项文案描述（≤11 字）
		IsChecked bool   `json:"is_checked,omitempty"` // 该选项是否默认选中
	} `json:"option_list"` // 选项列表，[1, 20]
}

// TemplateCard 模板卡片结构（通用类型，包含所有可能的字段），对应 Node TemplateCard。
//
// 可选子结构字段使用指针，确保未设置时 JSON 真正省略（omitempty 对非指针 struct 无效）。
type TemplateCard struct {
	CardType              string                          `json:"card_type"`                         // 卡片类型
	Source                *TemplateCardSource             `json:"source,omitempty"`                  // 卡片来源样式信息
	ActionMenu            *TemplateCardActionMenu         `json:"action_menu,omitempty"`             // 卡片右上角更多操作按钮
	MainTitle             *TemplateCardMainTitle          `json:"main_title,omitempty"`              // 模版卡片的主要内容
	EmphasisContent       *TemplateCardEmphasisContent    `json:"emphasis_content,omitempty"`        // 关键数据样式（建议不与引用样式共用）
	QuoteArea             *TemplateCardQuoteArea          `json:"quote_area,omitempty"`              // 引用文献样式（建议不与关键数据共用）
	SubTitleText          string                          `json:"sub_title_text,omitempty"`          // 二级普通文本（≤112 字）
	HorizontalContentList []TemplateCardHorizontalContent `json:"horizontal_content_list,omitempty"` // 二级标题+文本列表（≤6）
	JumpList              []TemplateCardJumpAction        `json:"jump_list,omitempty"`               // 跳转指引样式列表（≤3）
	CardAction            *TemplateCardAction             `json:"card_action,omitempty"`             // 整体卡片的点击跳转事件
	CardImage             *TemplateCardImage              `json:"card_image,omitempty"`              // 图片样式（news_notice 使用）
	ImageTextArea         *TemplateCardImageTextArea      `json:"image_text_area,omitempty"`         // 左图右文样式（news_notice 使用）
	VerticalContentList   []TemplateCardVerticalContent   `json:"vertical_content_list,omitempty"`   // 卡片二级垂直内容（≤4，news_notice 使用）
	ButtonSelection       *TemplateCardSelectionItem      `json:"button_selection,omitempty"`        // 下拉式选择器（button_interaction 使用）
	ButtonList            []TemplateCardButton            `json:"button_list,omitempty"`             // 按钮列表（≤6，button_interaction 使用）
	Checkbox              *TemplateCardCheckbox           `json:"checkbox,omitempty"`                // 选择题样式（vote_interaction 使用）
	SelectList            []TemplateCardSelectionItem     `json:"select_list,omitempty"`             // 下拉式选择器列表（≤3，multiple_interaction 使用）
	SubmitButton          *TemplateCardSubmitButton       `json:"submit_button,omitempty"`           // 提交按钮样式（vote/multiple_interaction 使用）
	TaskId                string                          `json:"task_id,omitempty"`                 // 任务 ID（≤128 字节，仅数字/字母/_-@）
	Feedback              *ReplyFeedback                  `json:"feedback,omitempty"`                // 反馈信息
}

// ========== 流式回复消息体 ==========

// StreamReply 流式回复内容对象，对应 Node StreamReplyBody 的 stream 字段（被 StreamWithTemplateCardReplyBody 复用）。
//
// Node 的 replyStream/replyStreamWithCard 用对象字面量 { id, finish, content } 总是序列化这三项
// （即便 finish=false / content=""），故 Finish/Content 不加 omitempty；仅 msg_item/feedback 条件序列化。
type StreamReply struct {
	Id       string         `json:"id"`                 // 流式消息 ID（首次回复设置，后续刷新用相同 ID）
	Finish   bool           `json:"finish"`             // 是否结束流式消息（总是序列化，对齐 Node）
	Content  string         `json:"content"`            // 回复内容（支持 Markdown，≤20480 字节，utf8）
	MsgItem  []ReplyMsgItem `json:"msg_item,omitempty"` // 图文混排列表（仅 finish=true 支持，≤10）
	Feedback *ReplyFeedback `json:"feedback,omitempty"` // 反馈信息（首次回复时设置）
}

// StreamReplyBody 流式回复消息体，对应 Node StreamReplyBody。
type StreamReplyBody struct {
	MsgType string      `json:"msgtype"` // 消息类型，固定值 stream
	Stream  StreamReply `json:"stream"`  // 流式内容
}

// ========== 欢迎语回复消息体 ==========

// WelcomeTextReplyBody 欢迎语回复消息体（文本类型），对应 Node WelcomeTextReplyBody。
type WelcomeTextReplyBody struct {
	MsgType string `json:"msgtype"` // 消息类型，固定值 text
	Text    struct {
		Content string `json:"content"` // 欢迎语文本内容
	} `json:"text"` // 文本内容
}

// WelcomeTemplateCardReplyBody 欢迎语回复消息体（模板卡片类型），对应 Node WelcomeTemplateCardReplyBody。
type WelcomeTemplateCardReplyBody struct {
	MsgType      string       `json:"msgtype"`       // 消息类型，固定值 template_card
	TemplateCard TemplateCard `json:"template_card"` // 模板卡片内容
}

// ========== 模板卡片回复消息体 ==========

// TemplateCardReplyBody 模板卡片回复消息体，对应 Node TemplateCardReplyBody。
type TemplateCardReplyBody struct {
	MsgType      string       `json:"msgtype"`       // 消息类型，固定值 template_card
	TemplateCard TemplateCard `json:"template_card"` // 模板卡片内容
}

// ========== 流式消息 + 模板卡片组合回复消息体 ==========

// StreamWithTemplateCardReplyBody 流式消息 + 模板卡片组合回复消息体，对应 Node StreamWithTemplateCardReplyBody。
type StreamWithTemplateCardReplyBody struct {
	MsgType      string        `json:"msgtype"`                 // 消息类型，固定值 stream_with_template_card
	Stream       StreamReply   `json:"stream"`                  // 流式内容
	TemplateCard *TemplateCard `json:"template_card,omitempty"` // 模板卡片内容（同一消息只能回复一次）
}

// ========== 更新模板卡片消息体 ==========

// UpdateTemplateCardBody 更新模板卡片消息体，对应 Node UpdateTemplateCardBody。
type UpdateTemplateCardBody struct {
	ResponseType string       `json:"response_type"`     // 响应类型，固定值 update_template_card
	UserIds      []string     `json:"userids,omitempty"` // 要替换的 userid 列表（不填则替换所有用户）
	TemplateCard TemplateCard `json:"template_card"`     // 要替换的模版卡片内容
}

// ========== 媒体消息类型 ==========

// WeComMediaType 企业微信媒体类型，对应 Node WeComMediaType（'file' | 'image' | 'voice' | 'video'）。
type WeComMediaType string

// 媒体类型取值常量。
const (
	WeComMediaFile  WeComMediaType = "file"  // 文件
	WeComMediaImage WeComMediaType = "image" // 图片
	WeComMediaVoice WeComMediaType = "voice" // 语音
	WeComMediaVideo WeComMediaType = "video" // 视频
)

// ========== 主动发送消息体 ==========

// SendMediaContent 媒体消息内容（file/image/voice 共用，仅 media_id），对应 Node SendMediaMsgBody 的内联 { media_id }。
type SendMediaContent struct {
	MediaId string `json:"media_id"` // 临时素材 media_id
}

// SendVideoContent 视频消息内容，对应 Node SendMediaMsgBody.video 内联结构。
type SendVideoContent struct {
	MediaId     string `json:"media_id"`              // 临时素材 media_id
	Title       string `json:"title,omitempty"`       // 视频标题（≤128 字节，超出截断）
	Description string `json:"description,omitempty"` // 视频描述（≤512 字节，超出截断）
}

// SendMediaMsgBody 媒体消息发送体（主动发送 + 被动回复共用），对应 Node SendMediaMsgBody。
//
// file/image/voice/video 四选一：仅设置与 msgtype 匹配的字段（指针 + omitempty 保证其余省略）。
type SendMediaMsgBody struct {
	MsgType WeComMediaType    `json:"msgtype"`         // 消息类型（file/image/voice/video）
	File    *SendMediaContent `json:"file,omitempty"`  // 文件消息
	Image   *SendMediaContent `json:"image,omitempty"` // 图片消息
	Voice   *SendMediaContent `json:"voice,omitempty"` // 语音消息
	Video   *SendVideoContent `json:"video,omitempty"` // 视频消息
}

// SendMarkdownMsgBody 主动发送 Markdown 消息体，对应 Node SendMarkdownMsgBody。
type SendMarkdownMsgBody struct {
	MsgType  string `json:"msgtype"` // 消息类型，固定值 markdown
	Markdown struct {
		Content string `json:"content"` // markdown 文本内容
	} `json:"markdown"` // markdown 消息内容
}

// SendTemplateCardMsgBody 主动发送模板卡片消息体，对应 Node SendTemplateCardMsgBody。
type SendTemplateCardMsgBody struct {
	MsgType      string       `json:"msgtype"`       // 消息类型，固定值 template_card
	TemplateCard TemplateCard `json:"template_card"` // 模板卡片内容
}

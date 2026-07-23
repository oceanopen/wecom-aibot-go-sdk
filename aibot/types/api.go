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

package aibot

import (
	"encoding/json"
	"testing"
)

// TestReExportLogger 验证 Logger 接口可从 aibot 包引用。
func TestReExportLogger(t *testing.T) {
	var _ Logger = (*stubLogger)(nil)
}

type stubLogger struct{}

func (stubLogger) Debug(string, ...any) {}
func (stubLogger) Info(string, ...any)  {}
func (stubLogger) Warn(string, ...any)  {}
func (stubLogger) Error(string, ...any) {}

// TestReExportErrors 验证错误类型可从 aibot 包引用。
func TestReExportErrors(t *testing.T) {
	err := NewWsAuthFailureError(5)
	if err.Code() != WsAuthFailureCode {
		t.Errorf("WsAuthFailureError.Code() = %q, want %q", err.Code(), WsAuthFailureCode)
	}
	err2 := NewWsReconnectExhaustedError(10)
	if err2.Code() != WsReconnectExhaustedCode {
		t.Errorf("WsReconnectExhaustedError.Code() = %q, want %q", err2.Code(), WsReconnectExhaustedCode)
	}
}

// TestReExportConfig 验证 WsClientOptions 可从 aibot 包引用。
func TestReExportConfig(t *testing.T) {
	opts := WsClientOptions{BotId: "bot_001", Secret: "secret"}
	if opts.BotId != "bot_001" {
		t.Errorf("WsClientOptions.BotId = %q, want %q", opts.BotId, "bot_001")
	}
}

// TestReExportWsCmd 验证 WsCmd 可从 aibot 包引用。
func TestReExportWsCmd(t *testing.T) {
	if WsCmd.Subscribe != "aibot_subscribe" {
		t.Errorf("WsCmd.Subscribe = %q, want %q", WsCmd.Subscribe, "aibot_subscribe")
	}
}

// TestReExportWsFrame 验证泛型别名 WsFrame[T] 可从 aibot 包使用。
func TestReExportWsFrame(t *testing.T) {
	// WsFrame[*TextMessage] — 这是最核心的用法
	frame := WsFrame[*TextMessage]{
		Cmd:     WsCmd.Callback,
		Headers: WsFrameHeaders{ReqId: "req_001"},
		Body: &TextMessage{
			BaseMessage: BaseMessage{MsgId: "msg_001", MsgType: MessageType.Text},
			Text:        TextContent{Content: "hello"},
		},
		ErrCode: 0,
		ErrMsg:  "ok",
	}
	if frame.Cmd != WsCmd.Callback {
		t.Errorf("WsFrame.Cmd = %q, want %q", frame.Cmd, WsCmd.Callback)
	}
	if frame.Body.Text.Content != "hello" {
		t.Errorf("WsFrame.Body.Text.Content = %q, want %q", frame.Body.Text.Content, "hello")
	}

	// JSON 往返
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal WsFrame[*TextMessage]: %v", err)
	}
	var decoded WsFrame[*TextMessage]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal WsFrame[*TextMessage]: %v", err)
	}
	if decoded.Body.Text.Content != "hello" {
		t.Errorf("decoded Body.Text.Content = %q, want %q", decoded.Body.Text.Content, "hello")
	}
}

// TestReExportWsFrameWithAny 验证 WsFrame[any] 泛型别名可用。
func TestReExportWsFrameWithAny(t *testing.T) {
	frame := WsFrame[any]{
		Cmd:     WsCmd.Heartbeat,
		Headers: WsFrameHeaders{ReqId: "req_002"},
	}
	if frame.Cmd != WsCmd.Heartbeat {
		t.Errorf("WsFrame.Cmd = %q, want %q", frame.Cmd, WsCmd.Heartbeat)
	}
}

// TestReExportMessageTypes 验证消息类型可从 aibot 包引用。
func TestReExportMessageTypes(t *testing.T) {
	if MessageType.Text != "text" {
		t.Errorf("MessageType.Text = %q, want %q", MessageType.Text, "text")
	}
	msg := TextMessage{
		BaseMessage: BaseMessage{MsgId: "msg_001", MsgType: MessageType.Text},
		Text:        TextContent{Content: "hi"},
	}
	if msg.Text.Content != "hi" {
		t.Errorf("TextMessage.Text.Content = %q, want %q", msg.Text.Content, "hi")
	}
	// 其他消息类型
	_ = ImageMessage{}
	_ = MixedMessage{}
	_ = VoiceMessage{}
	_ = FileMessage{}
	_ = VideoMessage{}
}

// TestReExportEventTypes 验证事件类型可从 aibot 包引用。
func TestReExportEventTypes(t *testing.T) {
	if EventType.EnterChat != "enter_chat" {
		t.Errorf("EventType.EnterChat = %q, want %q", EventType.EnterChat, "enter_chat")
	}
	// EventMessage + DecodeEvent
	jsonStr := `{"msgid":"evt_001","create_time":0,"aibotid":"bot_001","from":{"userid":"u1"},"msgtype":"event","event":{"eventtype":"enter_chat"}}`
	var msg EventMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal EventMessage: %v", err)
	}
	ev := msg.DecodeEvent()
	if ev == nil {
		t.Fatal("DecodeEvent returned nil, want non-nil")
	}
	if _, ok := ev.(EnterChatEvent); !ok {
		t.Fatalf("DecodeEvent returned %T, want EnterChatEvent", ev)
	}
	// 其他事件类型
	_ = TemplateCardEventData{}
	_ = FeedbackEventData{}
	_ = DisconnectedEventData{}
}

// TestReExportTemplateCard 验证模板卡片及子结构、TemplateCardType 可从 aibot 包引用（任务 24）。
func TestReExportTemplateCard(t *testing.T) {
	if TemplateCardType.TextNotice != "text_notice" {
		t.Errorf("TemplateCardType.TextNotice = %q, want text_notice", TemplateCardType.TextNotice)
	}
	card := TemplateCard{
		CardType:   TemplateCardType.ButtonInteraction,
		MainTitle:  &TemplateCardMainTitle{Title: "标题"},
		CardAction: &TemplateCardAction{Type: 1, Url: "https://example.com"},
		ButtonList: []TemplateCardButton{{Text: "确认", Key: "ok"}},
		TaskId:     "task_001",
		Feedback:   &ReplyFeedback{Id: "fb_001"},
	}
	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal TemplateCard: %v", err)
	}
	var decoded TemplateCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal TemplateCard: %v", err)
	}
	if decoded.TaskId != "task_001" || decoded.CardAction.Type != 1 {
		t.Errorf("TemplateCard round-trip mismatch: %+v", decoded)
	}
	// 回复体别名可用
	_ = TemplateCardReplyBody{MsgType: "template_card", TemplateCard: card}
	_ = StreamReplyBody{MsgType: "stream", Stream: StreamReply{Id: "s1"}}
	_ = WelcomeTextReplyBody{MsgType: "text"}
	_ = StreamWithTemplateCardReplyBody{MsgType: "stream_with_template_card"}
	_ = UpdateTemplateCardBody{ResponseType: "update_template_card"}
}

// TestReExportMediaAndSend 验证媒体类型常量与主动发送体可从 aibot 包引用（任务 26）。
func TestReExportMediaAndSend(t *testing.T) {
	if WeComMediaImage != "image" {
		t.Errorf("WeComMediaImage = %q, want image", WeComMediaImage)
	}
	var mt WeComMediaType = WeComMediaVideo
	if mt != "video" {
		t.Errorf("WeComMediaType = %q, want video", mt)
	}
	// 媒体发送体：仅设置匹配字段
	body := SendMediaMsgBody{MsgType: WeComMediaImage, Image: &SendMediaContent{MediaId: "mid"}}
	data, _ := json.Marshal(body)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	if m["msgtype"] != "image" || m["image"] == nil {
		t.Errorf("SendMediaMsgBody json = %s", data)
	}
	for _, k := range []string{"file", "voice", "video"} {
		if _, ok := m[k]; ok {
			t.Errorf("SendMediaMsgBody.%s should be absent for image", k)
		}
	}
	_ = SendMarkdownMsgBody{}
	_ = SendVideoContent{}
}

// TestReExportUploadTypes 验证上传相关类型可从 aibot 包引用（任务 27）。
func TestReExportUploadTypes(t *testing.T) {
	opts := UploadMediaOptions{Type: WeComMediaFile, Filename: "f.bin"}
	if opts.Type != WeComMediaFile || opts.Filename != "f.bin" {
		t.Errorf("UploadMediaOptions = %+v", opts)
	}
	result := UploadMediaFinishResult{Type: WeComMediaImage, MediaId: "mid", CreatedAt: "2026-01-01T00:00:00Z"}
	if result.MediaId != "mid" {
		t.Errorf("UploadMediaFinishResult.MediaId = %q", result.MediaId)
	}
	// 上传请求体别名可用
	_ = UploadMediaInitBody{Type: WeComMediaFile, Filename: "f", TotalSize: 1, TotalChunks: 1}
	_ = UploadMediaChunkBody{UploadId: "uid", ChunkIndex: 0, Base64Data: "AA=="}
	_ = UploadMediaFinishBody{UploadId: "uid"}
}

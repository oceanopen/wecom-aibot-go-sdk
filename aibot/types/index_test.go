package types

import (
	"encoding/json"
	"testing"
)

// TestReExportCommon 验证 common.go 的公开符号可从 types 包引用。
func TestReExportCommon(t *testing.T) {
	// Logger 接口
	var _ Logger = (*testLogger)(nil)
	// WsAuthFailureError
	err := NewWsAuthFailureError(5)
	if err.Code() != WsAuthFailureCode {
		t.Errorf("WsAuthFailureError.Code() = %q, want %q", err.Code(), WsAuthFailureCode)
	}
	// WsReconnectExhaustedError
	err2 := NewWsReconnectExhaustedError(10)
	if err2.Code() != WsReconnectExhaustedCode {
		t.Errorf("WsReconnectExhaustedError.Code() = %q, want %q", err2.Code(), WsReconnectExhaustedCode)
	}
}

// testLogger 用于测试 Logger 接口的 stub。
type testLogger struct{}

func (testLogger) Debug(string, ...any) {}
func (testLogger) Info(string, ...any)  {}
func (testLogger) Warn(string, ...any)  {}
func (testLogger) Error(string, ...any) {}

// TestReExportConfig 验证 config.go 的公开符号可从 types 包引用。
func TestReExportConfig(t *testing.T) {
	opts := WsClientOptions{
		BotId:  "bot_001",
		Secret: "secret_001",
	}
	if opts.BotId != "bot_001" {
		t.Errorf("WsClientOptions.BotId = %q, want %q", opts.BotId, "bot_001")
	}
}

// TestReExportApi 验证 api.go 的公开符号可从 types 包引用。
func TestReExportApi(t *testing.T) {
	// WsCmd 常量
	if WsCmd.Subscribe != "aibot_subscribe" {
		t.Errorf("WsCmd.Subscribe = %q, want %q", WsCmd.Subscribe, "aibot_subscribe")
	}
	// WsFrameHeaders
	h := WsFrameHeaders{ReqId: "req_001"}
	if h.ReqId != "req_001" {
		t.Errorf("WsFrameHeaders.ReqId = %q, want %q", h.ReqId, "req_001")
	}
	// WsFrame[T]
	frame := WsFrame[string]{Cmd: WsCmd.Heartbeat, Headers: h}
	if frame.Cmd != WsCmd.Heartbeat {
		t.Errorf("WsFrame.Cmd = %q, want %q", frame.Cmd, WsCmd.Heartbeat)
	}
}

// TestReExportMessage 验证 message.go 的公开符号可从 types 包引用。
func TestReExportMessage(t *testing.T) {
	// MessageType 常量
	if MessageType.Text != "text" {
		t.Errorf("MessageType.Text = %q, want %q", MessageType.Text, "text")
	}
	// BaseMessage
	msg := BaseMessage{MsgId: "msg_001", MsgType: MessageType.Text}
	if msg.MsgId != "msg_001" {
		t.Errorf("BaseMessage.MsgId = %q, want %q", msg.MsgId, "msg_001")
	}
	// TextMessage
	txt := TextMessage{BaseMessage: BaseMessage{MsgType: MessageType.Text}, Text: TextContent{Content: "hi"}}
	if txt.Text.Content != "hi" {
		t.Errorf("TextMessage.Text.Content = %q, want %q", txt.Text.Content, "hi")
	}
	// ImageMessage / MixedMessage / VoiceMessage / FileMessage / VideoMessage
	_ = ImageMessage{}
	_ = MixedMessage{}
	_ = VoiceMessage{}
	_ = FileMessage{}
	_ = VideoMessage{}
}

// TestReExportEvent 验证 event.go 的公开符号可从 types 包引用。
func TestReExportEvent(t *testing.T) {
	// EventType 常量
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
	// 其他事件数据类型
	_ = TemplateCardEventData{}
	_ = FeedbackEventData{}
	_ = DisconnectedEventData{}
}

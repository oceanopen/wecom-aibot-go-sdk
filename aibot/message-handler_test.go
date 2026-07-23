package aibot

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// newHandlerClient 构造一个仅用于分发测试的 WsClient（不连接）。
func newHandlerClient() *WsClient {
	return NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s"})
}

// TestHandleTextMessage 验证文本帧触发 OnMessage + OnText，且 body 正确。
func TestHandleTextMessage(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req_text_1"},
		"body": {
			"msgid": "m1", "aibotid": "bot1", "msgtype": "text", "chattype": "single",
			"text": {"content": "hello world"}
		}
	}`)
	client := newHandlerClient()

	var gotMsgId, gotMsgType, gotContent string
	var onMessageCalled bool
	client.OnMessage = func(f *types.WsFrame[types.BaseMessage]) {
		onMessageCalled = true
		gotMsgId = f.Body.MsgId
		gotMsgType = f.Body.MsgType
	}
	client.OnText = func(f *types.WsFrame[types.TextMessage]) {
		gotContent = f.Body.Text.Content
	}

	client.messageHandler.HandleFrame(raw, client)

	if !onMessageCalled {
		t.Error("OnMessage was not called for text frame")
	}
	if gotMsgId != "m1" {
		t.Errorf("OnMessage body.msgid = %q, want m1", gotMsgId)
	}
	if gotMsgType != "text" {
		t.Errorf("OnMessage body.msgtype = %q, want text", gotMsgType)
	}
	if gotContent != "hello world" {
		t.Errorf("OnText body.text.content = %q, want 'hello world'", gotContent)
	}
}

// TestHandleEventMessage 验证事件帧触发 OnEvent，且 body 正确（含 DecodeEvent）。
func TestHandleEventMessage(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_event_callback",
		"headers": {"req_id": "req_evt_1"},
		"body": {
			"msgid": "e1", "aibotid": "bot1", "msgtype": "event",
			"event": {"eventtype": "enter_chat"}
		}
	}`)
	client := newHandlerClient()

	var gotMsgId string
	var decoded types.EventContent
	var onEventCalled bool
	client.OnEvent = func(f *types.WsFrame[types.EventMessage]) {
		onEventCalled = true
		gotMsgId = f.Body.MsgId
		decoded = f.Body.DecodeEvent()
	}

	client.messageHandler.HandleFrame(raw, client)

	if !onEventCalled {
		t.Fatal("OnEvent was not called for event frame")
	}
	if gotMsgId != "e1" {
		t.Errorf("OnEvent body.msgid = %q, want e1", gotMsgId)
	}
	enterChat, ok := decoded.(types.EnterChatEvent)
	if !ok {
		t.Fatalf("DecodeEvent = %T, want EnterChatEvent", decoded)
	}
	if enterChat.EventType != types.EventType.EnterChat {
		t.Errorf("eventtype = %q, want %q", enterChat.EventType, types.EventType.EnterChat)
	}
}

// TestHandleEventDoesNotTriggerOnMessage 验证事件帧不触发 OnMessage（仅 OnEvent）。
func TestHandleEventDoesNotTriggerOnMessage(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_event_callback",
		"headers": {"req_id": "req_evt_2"},
		"body": {"msgid": "e2", "msgtype": "event", "event": {"eventtype": "enter_chat"}}
	}`)
	client := newHandlerClient()

	var onMessageCalled bool
	client.OnMessage = func(f *types.WsFrame[types.BaseMessage]) {
		onMessageCalled = true
	}

	client.messageHandler.HandleFrame(raw, client)

	if onMessageCalled {
		t.Error("OnMessage should not be called for event frame")
	}
}

// TestHandleUnhandledMessageType 验证未处理的 msgtype 仅触发 OnMessage，不 panic。
func TestHandleUnhandledMessageType(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req_img_1"},
		"body": {"msgid": "m2", "msgtype": "image", "image": {"media_id": "mid"}}
	}`)
	client := newHandlerClient()

	var onMessageCalled bool
	client.OnMessage = func(f *types.WsFrame[types.BaseMessage]) {
		onMessageCalled = true
	}

	// 不应 panic（OnImage 尚未在任务 16 接线）
	client.messageHandler.HandleFrame(raw, client)

	if !onMessageCalled {
		t.Error("OnMessage should be called for image frame (generic)")
	}
}

// TestHandleInvalidMessageFormat 验证缺失 msgtype 的帧不触发任何回调。
func TestHandleInvalidMessageFormat(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req_bad_1"},
		"body": {"msgid": "m3"}
	}`)
	client := newHandlerClient()

	var anyCalled bool
	client.OnMessage = func(f *types.WsFrame[types.BaseMessage]) { anyCalled = true }
	client.OnText = func(f *types.WsFrame[types.TextMessage]) { anyCalled = true }
	client.OnEvent = func(f *types.WsFrame[types.EventMessage]) { anyCalled = true }

	client.messageHandler.HandleFrame(raw, client)

	if anyCalled {
		t.Error("no callback should fire for frame missing msgtype")
	}
}

// TestHandleNilCallbacks 验证回调未设置时不 panic。
func TestHandleNilCallbacks(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req_nil_1"},
		"body": {"msgid": "m4", "msgtype": "text", "text": {"content": "hi"}}
	}`)
	client := newHandlerClient()                   // 所有回调为 nil
	client.messageHandler.HandleFrame(raw, client) // 不应 panic
}

// ========== 任务 19：其余消息/事件类型分发 ==========

// TestHandleMessageTypesDispatch 验证各 msgtype 触发对应类型化回调。
func TestHandleMessageTypesDispatch(t *testing.T) {
	client := newHandlerClient()
	var mu sync.Mutex
	fired := ""
	set := func(name string) { mu.Lock(); fired = name; mu.Unlock() }

	client.OnText = func(*types.WsFrame[types.TextMessage]) { set("text") }
	client.OnImage = func(*types.WsFrame[types.ImageMessage]) { set("image") }
	client.OnMixed = func(*types.WsFrame[types.MixedMessage]) { set("mixed") }
	client.OnVoice = func(*types.WsFrame[types.VoiceMessage]) { set("voice") }
	client.OnFile = func(*types.WsFrame[types.FileMessage]) { set("file") }
	client.OnVideo = func(*types.WsFrame[types.VideoMessage]) { set("video") }

	cases := []struct{ msgtype, want string }{
		{"text", "text"},
		{"image", "image"},
		{"mixed", "mixed"},
		{"voice", "voice"},
		{"file", "file"},
		{"video", "video"},
	}
	for _, c := range cases {
		mu.Lock()
		fired = ""
		mu.Unlock()
		raw := json.RawMessage(fmt.Sprintf(
			`{"cmd":"aibot_msg_callback","headers":{"req_id":"r"},"body":{"msgid":"m","msgtype":%q}}`,
			c.msgtype,
		))
		client.messageHandler.HandleFrame(raw, client)
		mu.Lock()
		got := fired
		mu.Unlock()
		if got != c.want {
			t.Errorf("msgtype=%s fired %q, want %q", c.msgtype, got, c.want)
		}
	}
}

// TestHandleTypedMessageBody 验证类型化回调 body 正确（以 image 为例）。
func TestHandleTypedMessageBody(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_msg_callback",
		"headers": {"req_id": "req_img_body"},
		"body": {"msgid": "img1", "aibotid": "bot1", "msgtype": "image", "image": {"media_id": "mid123"}}
	}`)
	client := newHandlerClient()

	var gotMsgId string
	client.OnImage = func(f *types.WsFrame[types.ImageMessage]) {
		gotMsgId = f.Body.MsgId
	}
	client.messageHandler.HandleFrame(raw, client)
	if gotMsgId != "img1" {
		t.Errorf("OnImage body.msgid = %q, want img1", gotMsgId)
	}
}

// TestHandleEventTypesDispatch 验证各 eventtype 触发对应类型化事件回调。
func TestHandleEventTypesDispatch(t *testing.T) {
	client := newHandlerClient()
	var mu sync.Mutex
	fired := ""
	set := func(name string) { mu.Lock(); fired = name; mu.Unlock() }

	client.OnEvent = func(*types.WsFrame[types.EventMessage]) { set("event") }
	client.OnEnterChat = func(*types.WsFrame[types.EventMessage]) { set("enter_chat") }
	client.OnTemplateCardEvent = func(*types.WsFrame[types.EventMessage]) { set("template_card_event") }
	client.OnFeedbackEvent = func(*types.WsFrame[types.EventMessage]) { set("feedback_event") }
	client.OnDisconnectedEvent = func(*types.WsFrame[types.EventMessage]) { set("disconnected_event") }

	cases := []struct{ eventtype, want string }{
		{"enter_chat", "enter_chat"},
		{"template_card_event", "template_card_event"},
		{"feedback_event", "feedback_event"},
		{"disconnected_event", "disconnected_event"},
	}
	for _, c := range cases {
		mu.Lock()
		fired = ""
		mu.Unlock()
		raw := json.RawMessage(fmt.Sprintf(
			`{"cmd":"aibot_event_callback","headers":{"req_id":"r"},"body":{"msgid":"e","msgtype":"event","event":{"eventtype":%q}}}`,
			c.eventtype,
		))
		client.messageHandler.HandleFrame(raw, client)
		mu.Lock()
		got := fired
		mu.Unlock()
		// OnEvent 先触发（set "event"），随后类型化回调覆盖
		if got != c.want {
			t.Errorf("eventtype=%s fired %q, want %q", c.eventtype, got, c.want)
		}
	}
}

// TestHandleTemplateCardEventBody 验证类型化事件回调 body 正确（以 template_card_event 为例）。
func TestHandleTemplateCardEventBody(t *testing.T) {
	raw := json.RawMessage(`{
		"cmd": "aibot_event_callback",
		"headers": {"req_id": "req_tc"},
		"body": {"msgid": "tc1", "msgtype": "event", "event": {"eventtype": "template_card_event", "event_key": "btn1", "task_id": "t1"}}
	}`)
	client := newHandlerClient()

	var gotKey string
	client.OnTemplateCardEvent = func(f *types.WsFrame[types.EventMessage]) {
		tc, ok := f.Body.Event.(types.TemplateCardEventData)
		if ok {
			gotKey = tc.EventKey
		}
	}
	client.messageHandler.HandleFrame(raw, client)
	if gotKey != "btn1" {
		t.Errorf("template_card_event event_key = %q, want btn1", gotKey)
	}
}

package types

import (
	"encoding/json"
	"testing"
)

func TestEventTypeConstants(t *testing.T) {
	expected := map[string]string{
		"EnterChat":         "enter_chat",
		"TemplateCardEvent": "template_card_event",
		"FeedbackEvent":     "feedback_event",
		"Disconnected":      "disconnected_event",
	}
	got := map[string]string{
		"EnterChat":         EventType.EnterChat,
		"TemplateCardEvent": EventType.TemplateCardEvent,
		"FeedbackEvent":     EventType.FeedbackEvent,
		"Disconnected":      EventType.Disconnected,
	}
	for name, want := range expected {
		if got[name] != want {
			t.Errorf("EventType.%s = %q, want %q", name, got[name], want)
		}
	}
}

func TestEventMessageDecodeEnterChat(t *testing.T) {
	jsonStr := `{
		"msgid": "evt_001",
		"create_time": 1700000000,
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_001"},
		"msgtype": "event",
		"event": {"eventtype": "enter_chat"}
	}`
	var msg EventMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal EventMessage: %v", err)
	}
	ev := msg.DecodeEvent()
	if ev == nil {
		t.Fatal("DecodeEvent returned nil, want non-nil")
	}
	enter, ok := ev.(EnterChatEvent)
	if !ok {
		t.Fatalf("DecodeEvent returned %T, want EnterChatEvent", ev)
	}
	if enter.EventType != EventType.EnterChat {
		t.Errorf("EnterChatEvent.EventType = %q, want %q", enter.EventType, EventType.EnterChat)
	}
	// Event 字段也应被填充
	if msg.Event == nil {
		t.Fatal("EventMessage.Event is nil after DecodeEvent")
	}
}

func TestEventMessageDecodeTemplateCardEvent(t *testing.T) {
	jsonStr := `{
		"msgid": "evt_002",
		"create_time": 1700000000,
		"aibotid": "bot_001",
		"from": {"userid": "user_002", "corpid": "corp_001"},
		"msgtype": "event",
		"event": {"eventtype": "template_card_event", "event_key": "btn_ok", "task_id": "task_001"}
	}`
	var msg EventMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal EventMessage: %v", err)
	}
	ev := msg.DecodeEvent()
	if ev == nil {
		t.Fatal("DecodeEvent returned nil, want non-nil")
	}
	card, ok := ev.(TemplateCardEventData)
	if !ok {
		t.Fatalf("DecodeEvent returned %T, want TemplateCardEventData", ev)
	}
	if card.EventKey != "btn_ok" {
		t.Errorf("EventKey = %q, want %q", card.EventKey, "btn_ok")
	}
	if card.TaskId != "task_001" {
		t.Errorf("TaskId = %q, want %q", card.TaskId, "task_001")
	}
	// 验证 EventFrom.CorpId
	if msg.From.CorpId != "corp_001" {
		t.Errorf("From.CorpId = %q, want %q", msg.From.CorpId, "corp_001")
	}
}

func TestEventMessageDecodeFeedbackEvent(t *testing.T) {
	jsonStr := `{
		"msgid": "evt_003",
		"create_time": 1700000000,
		"aibotid": "bot_001",
		"from": {"userid": "user_003"},
		"msgtype": "event",
		"event": {"eventtype": "feedback_event"}
	}`
	var msg EventMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal EventMessage: %v", err)
	}
	ev := msg.DecodeEvent()
	if ev == nil {
		t.Fatal("DecodeEvent returned nil, want non-nil")
	}
	if _, ok := ev.(FeedbackEventData); !ok {
		t.Fatalf("DecodeEvent returned %T, want FeedbackEventData", ev)
	}
}

func TestEventMessageDecodeDisconnectedEvent(t *testing.T) {
	jsonStr := `{
		"msgid": "evt_004",
		"create_time": 1700000000,
		"aibotid": "bot_001",
		"from": {"userid": "user_004"},
		"msgtype": "event",
		"event": {"eventtype": "disconnected_event"}
	}`
	var msg EventMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal EventMessage: %v", err)
	}
	ev := msg.DecodeEvent()
	if ev == nil {
		t.Fatal("DecodeEvent returned nil, want non-nil")
	}
	if _, ok := ev.(DisconnectedEventData); !ok {
		t.Fatalf("DecodeEvent returned %T, want DisconnectedEventData", ev)
	}
}

func TestEventMessageDecodeUnknownEvent(t *testing.T) {
	jsonStr := `{
		"msgid": "evt_005",
		"create_time": 1700000000,
		"aibotid": "bot_001",
		"from": {"userid": "user_005"},
		"msgtype": "event",
		"event": {"eventtype": "unknown_event"}
	}`
	var msg EventMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal EventMessage: %v", err)
	}
	ev := msg.DecodeEvent()
	if ev != nil {
		t.Errorf("DecodeEvent returned %T for unknown eventtype, want nil", ev)
	}
}

func TestEventMessageOmitEmpty(t *testing.T) {
	msg := EventMessage{
		MsgId:   "evt_006",
		AibotId: "bot_001",
		From:    EventFrom{UserId: "user_006"},
		MsgType: "event",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal EventMessage: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	for _, key := range []string{"chatid", "chattype"} {
		if _, ok := m[key]; ok {
			t.Errorf("expected %q to be omitted when zero, but it was present", key)
		}
	}
}

func TestEventContentInterface(t *testing.T) {
	// 验证所有事件类型均实现 EventContent 接口。
	var _ EventContent = EnterChatEvent{}
	var _ EventContent = TemplateCardEventData{}
	var _ EventContent = FeedbackEventData{}
	var _ EventContent = DisconnectedEventData{}
}

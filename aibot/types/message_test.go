package types

import (
	"encoding/json"
	"testing"
)

func TestMessageTypeConstants(t *testing.T) {
	expected := map[string]string{
		"Text":  "text",
		"Image": "image",
		"Mixed": "mixed",
		"Voice": "voice",
		"File":  "file",
		"Video": "video",
	}
	got := map[string]string{
		"Text":  MessageType.Text,
		"Image": MessageType.Image,
		"Mixed": MessageType.Mixed,
		"Voice": MessageType.Voice,
		"File":  MessageType.File,
		"Video": MessageType.Video,
	}
	for name, want := range expected {
		if got[name] != want {
			t.Errorf("MessageType.%s = %q, want %q", name, got[name], want)
		}
	}
}

func TestTextMessageJSON(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_001",
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_001"},
		"create_time": 1700000000,
		"msgtype": "text",
		"text": {"content": "hello world"}
	}`
	var msg TextMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal TextMessage: %v", err)
	}
	if msg.MsgId != "msg_001" {
		t.Errorf("MsgId = %q, want %q", msg.MsgId, "msg_001")
	}
	if msg.MsgType != MessageType.Text {
		t.Errorf("MsgType = %q, want %q", msg.MsgType, MessageType.Text)
	}
	if msg.Text.Content != "hello world" {
		t.Errorf("Text.Content = %q, want %q", msg.Text.Content, "hello world")
	}
	if msg.From.UserId != "user_001" {
		t.Errorf("From.UserId = %q, want %q", msg.From.UserId, "user_001")
	}
	if msg.ChatType != "single" {
		t.Errorf("ChatType = %q, want %q", msg.ChatType, "single")
	}
}

func TestImageMessageJSON(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_002",
		"aibotid": "bot_001",
		"chattype": "group",
		"chatid": "chat_001",
		"from": {"userid": "user_002"},
		"msgtype": "image",
		"image": {"url": "https://example.com/img", "aeskey": "abc123"}
	}`
	var msg ImageMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal ImageMessage: %v", err)
	}
	if msg.MsgType != MessageType.Image {
		t.Errorf("MsgType = %q, want %q", msg.MsgType, MessageType.Image)
	}
	if msg.Image.Url != "https://example.com/img" {
		t.Errorf("Image.Url = %q, want %q", msg.Image.Url, "https://example.com/img")
	}
	if msg.Image.AesKey != "abc123" {
		t.Errorf("Image.AesKey = %q, want %q", msg.Image.AesKey, "abc123")
	}
	if msg.ChatId != "chat_001" {
		t.Errorf("ChatId = %q, want %q", msg.ChatId, "chat_001")
	}
}

func TestMixedMessageJSON(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_003",
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_003"},
		"msgtype": "mixed",
		"mixed": {
			"msg_item": [
				{"msgtype": "text", "text": {"content": "part1"}},
				{"msgtype": "image", "image": {"url": "https://example.com/img"}}
			]
		}
	}`
	var msg MixedMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal MixedMessage: %v", err)
	}
	if msg.MsgType != MessageType.Mixed {
		t.Errorf("MsgType = %q, want %q", msg.MsgType, MessageType.Mixed)
	}
	if len(msg.Mixed.MsgItem) != 2 {
		t.Fatalf("len(MsgItem) = %d, want 2", len(msg.Mixed.MsgItem))
	}
	if msg.Mixed.MsgItem[0].Text.Content != "part1" {
		t.Errorf("MsgItem[0].Text.Content = %q, want %q", msg.Mixed.MsgItem[0].Text.Content, "part1")
	}
	if msg.Mixed.MsgItem[1].Image.Url != "https://example.com/img" {
		t.Errorf("MsgItem[1].Image.Url = %q, want %q", msg.Mixed.MsgItem[1].Image.Url, "https://example.com/img")
	}
}

func TestVoiceMessageJSON(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_004",
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_004"},
		"msgtype": "voice",
		"voice": {"content": "transcribed text"}
	}`
	var msg VoiceMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal VoiceMessage: %v", err)
	}
	if msg.Voice.Content != "transcribed text" {
		t.Errorf("Voice.Content = %q, want %q", msg.Voice.Content, "transcribed text")
	}
}

func TestFileMessageJSON(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_005",
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_005"},
		"msgtype": "file",
		"file": {"url": "https://example.com/file", "aeskey": "filekey"}
	}`
	var msg FileMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal FileMessage: %v", err)
	}
	if msg.File.Url != "https://example.com/file" {
		t.Errorf("File.Url = %q, want %q", msg.File.Url, "https://example.com/file")
	}
	if msg.File.AesKey != "filekey" {
		t.Errorf("File.AesKey = %q, want %q", msg.File.AesKey, "filekey")
	}
}

func TestVideoMessageJSON(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_006",
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_006"},
		"msgtype": "video",
		"video": {"url": "https://example.com/video"}
	}`
	var msg VideoMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal VideoMessage: %v", err)
	}
	if msg.Video.Url != "https://example.com/video" {
		t.Errorf("Video.Url = %q, want %q", msg.Video.Url, "https://example.com/video")
	}
	if msg.Video.AesKey != "" {
		t.Errorf("Video.AesKey = %q, want empty (omitempty)", msg.Video.AesKey)
	}
}

func TestMessageWithQuote(t *testing.T) {
	jsonStr := `{
		"msgid": "msg_007",
		"aibotid": "bot_001",
		"chattype": "single",
		"from": {"userid": "user_007"},
		"msgtype": "text",
		"text": {"content": "reply with quote"},
		"quote": {
			"msgtype": "text",
			"text": {"content": "original message"}
		}
	}`
	var msg TextMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		t.Fatalf("unmarshal TextMessage with quote: %v", err)
	}
	if msg.Quote == nil {
		t.Fatal("Quote is nil, want non-nil")
	}
	if msg.Quote.MsgType != "text" {
		t.Errorf("Quote.MsgType = %q, want %q", msg.Quote.MsgType, "text")
	}
	if msg.Quote.Text.Content != "original message" {
		t.Errorf("Quote.Text.Content = %q, want %q", msg.Quote.Text.Content, "original message")
	}
}

func TestBaseMessageOmitEmpty(t *testing.T) {
	// 验证 ChatId/CreateTime/ResponseUrl/Quote 为零值时 JSON 中省略。
	msg := BaseMessage{
		MsgId:    "msg_008",
		AibotId:  "bot_001",
		ChatType: "single",
		From:     MessageFrom{UserId: "user_008"},
		MsgType:  "text",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal BaseMessage: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	for _, key := range []string{"chatid", "create_time", "response_url", "quote"} {
		if _, ok := m[key]; ok {
			t.Errorf("expected %q to be omitted when zero, but it was present", key)
		}
	}
}

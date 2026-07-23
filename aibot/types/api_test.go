package types

import (
	"encoding/json"
	"testing"
)

func TestWsCmdConstants(t *testing.T) {
	// 验证所有 WsCmd 常量值与 Node SDK 一致。
	expected := map[string]string{
		"Subscribe":         "aibot_subscribe",
		"Heartbeat":         "ping",
		"Response":          "aibot_respond_msg",
		"ResponseWelcome":   "aibot_respond_welcome_msg",
		"ResponseUpdate":    "aibot_respond_update_msg",
		"SendMsg":           "aibot_send_msg",
		"UploadMediaInit":   "aibot_upload_media_init",
		"UploadMediaChunk":  "aibot_upload_media_chunk",
		"UploadMediaFinish": "aibot_upload_media_finish",
		"Callback":          "aibot_msg_callback",
		"EventCallback":     "aibot_event_callback",
	}
	got := map[string]string{
		"Subscribe":         WsCmd.Subscribe,
		"Heartbeat":         WsCmd.Heartbeat,
		"Response":          WsCmd.Response,
		"ResponseWelcome":   WsCmd.ResponseWelcome,
		"ResponseUpdate":    WsCmd.ResponseUpdate,
		"SendMsg":           WsCmd.SendMsg,
		"UploadMediaInit":   WsCmd.UploadMediaInit,
		"UploadMediaChunk":  WsCmd.UploadMediaChunk,
		"UploadMediaFinish": WsCmd.UploadMediaFinish,
		"Callback":          WsCmd.Callback,
		"EventCallback":     WsCmd.EventCallback,
	}
	for name, want := range expected {
		if got[name] != want {
			t.Errorf("WsCmd.%s = %q, want %q", name, got[name], want)
		}
	}
}

func TestWsFrameHeadersReqId(t *testing.T) {
	h := WsFrameHeaders{ReqId: "req_123"}
	if h.ReqId != "req_123" {
		t.Errorf("WsFrameHeaders.ReqId = %q, want %q", h.ReqId, "req_123")
	}
}

func TestWsFrameHeadersJSONRoundTrip(t *testing.T) {
	h := WsFrameHeaders{ReqId: "req_abc"}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("marshal WsFrameHeaders: %v", err)
	}
	var h2 WsFrameHeaders
	if err := json.Unmarshal(data, &h2); err != nil {
		t.Fatalf("unmarshal WsFrameHeaders: %v", err)
	}
	if h2.ReqId != h.ReqId {
		t.Errorf("WsFrameHeaders round-trip ReqId = %q, want %q", h2.ReqId, h.ReqId)
	}
}

func TestWsFrameJSONRoundTrip(t *testing.T) {
	// 使用 map body 类型测试 WsFrame JSON 往返。
	type bodyType = map[string]any

	original := WsFrame[bodyType]{
		Cmd: WsCmd.Subscribe,
		Headers: WsFrameHeaders{
			ReqId: "req_001",
		},
		Body: map[string]any{
			"secret": "my_secret",
			"bot_id": "bot_123",
		},
		ErrCode: 0,
		ErrMsg:  "ok",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal WsFrame: %v", err)
	}

	var decoded WsFrame[bodyType]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal WsFrame: %v", err)
	}

	if decoded.Cmd != original.Cmd {
		t.Errorf("WsFrame.Cmd = %q, want %q", decoded.Cmd, original.Cmd)
	}
	if decoded.Headers.ReqId != original.Headers.ReqId {
		t.Errorf("WsFrame.Headers.ReqId = %q, want %q", decoded.Headers.ReqId, original.Headers.ReqId)
	}
	if decoded.ErrCode != original.ErrCode {
		t.Errorf("WsFrame.ErrCode = %d, want %d", decoded.ErrCode, original.ErrCode)
	}
	if decoded.ErrMsg != original.ErrMsg {
		t.Errorf("WsFrame.ErrMsg = %q, want %q", decoded.ErrMsg, original.ErrMsg)
	}
	if decoded.Body["secret"] != original.Body["secret"] {
		t.Errorf("WsFrame.Body[\"secret\"] = %v, want %v", decoded.Body["secret"], original.Body["secret"])
	}
	if decoded.Body["bot_id"] != original.Body["bot_id"] {
		t.Errorf("WsFrame.Body[\"bot_id\"] = %v, want %v", decoded.Body["bot_id"], original.Body["bot_id"])
	}
}

func TestWsFrameOmitEmpty(t *testing.T) {
	// 验证 cmd/errcode/errmsg 为零值时 JSON 中省略（omitempty）。
	frame := WsFrame[string]{
		Headers: WsFrameHeaders{ReqId: "req_002"},
		Body:    "",
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if _, ok := m["cmd"]; ok {
		t.Error("expected cmd to be omitted when empty, but it was present")
	}
	if _, ok := m["errcode"]; ok {
		t.Error("expected errcode to be omitted when zero, but it was present")
	}
	if _, ok := m["errmsg"]; ok {
		t.Error("expected errmsg to be omitted when empty, but it was present")
	}
}

func TestWsFrameAuthResponse(t *testing.T) {
	// 验证认证/心跳响应帧：cmd 为空，errcode=0，errmsg="ok"。
	jsonStr := `{"headers":{"req_id":"req_003"},"errcode":0,"errmsg":"ok"}`
	var frame WsFrame[any]
	if err := json.Unmarshal([]byte(jsonStr), &frame); err != nil {
		t.Fatalf("unmarshal auth response: %v", err)
	}
	if frame.Cmd != "" {
		t.Errorf("WsFrame.Cmd = %q, want empty", frame.Cmd)
	}
	if frame.Headers.ReqId != "req_003" {
		t.Errorf("WsFrame.Headers.ReqId = %q, want %q", frame.Headers.ReqId, "req_003")
	}
	if frame.ErrCode != 0 {
		t.Errorf("WsFrame.ErrCode = %d, want 0", frame.ErrCode)
	}
	if frame.ErrMsg != "ok" {
		t.Errorf("WsFrame.ErrMsg = %q, want %q", frame.ErrMsg, "ok")
	}
}

func TestWsFrameCallbackFrame(t *testing.T) {
	// 验证消息回调帧：cmd=aibot_msg_callback，body 含消息字段。
	type msgBody = map[string]any
	jsonStr := `{"cmd":"aibot_msg_callback","headers":{"req_id":"req_004"},"body":{"msgid":"msg_001","msgtype":"text"}}`
	var frame WsFrame[msgBody]
	if err := json.Unmarshal([]byte(jsonStr), &frame); err != nil {
		t.Fatalf("unmarshal callback frame: %v", err)
	}
	if frame.Cmd != WsCmd.Callback {
		t.Errorf("WsFrame.Cmd = %q, want %q", frame.Cmd, WsCmd.Callback)
	}
	if frame.Body["msgid"] != "msg_001" {
		t.Errorf("WsFrame.Body[\"msgid\"] = %v, want %q", frame.Body["msgid"], "msg_001")
	}
	if frame.Body["msgtype"] != "text" {
		t.Errorf("WsFrame.Body[\"msgtype\"] = %v, want %q", frame.Body["msgtype"], "text")
	}
}

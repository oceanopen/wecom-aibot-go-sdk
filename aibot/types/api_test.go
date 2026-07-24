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

// ========== 模板卡片类型常量 ==========

func TestTemplateCardTypeConstants(t *testing.T) {
	expected := map[string]string{
		"TextNotice":          "text_notice",
		"NewsNotice":          "news_notice",
		"ButtonInteraction":   "button_interaction",
		"VoteInteraction":     "vote_interaction",
		"MultipleInteraction": "multiple_interaction",
	}
	got := map[string]string{
		"TextNotice":          TemplateCardType.TextNotice,
		"NewsNotice":          TemplateCardType.NewsNotice,
		"ButtonInteraction":   TemplateCardType.ButtonInteraction,
		"VoteInteraction":     TemplateCardType.VoteInteraction,
		"MultipleInteraction": TemplateCardType.MultipleInteraction,
	}
	for name, want := range expected {
		if got[name] != want {
			t.Errorf("TemplateCardType.%s = %q, want %q", name, got[name], want)
		}
	}
}

// ========== TemplateCard omitempty（可选省略 / 必填保留）==========

func TestTemplateCardOmitEmpty(t *testing.T) {
	// 仅 card_type：所有可选字段（含指针子结构、切片、字符串）应省略
	minimal := TemplateCard{CardType: TemplateCardType.TextNotice}
	data, err := json.Marshal(minimal)
	if err != nil {
		t.Fatalf("marshal minimal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	if len(m) != 1 {
		t.Errorf("minimal card should have only card_type, got keys: %v (json=%s)", keys(m), data)
	}
	if m["card_type"] != TemplateCardType.TextNotice {
		t.Errorf("card_type = %v, want %q", m["card_type"], TemplateCardType.TextNotice)
	}

	// 设置一个可选指针子结构 → 应出现
	card := TemplateCard{
		CardType:   TemplateCardType.TextNotice,
		Source:     &TemplateCardSource{Desc: "来源"},
		CardAction: &TemplateCardAction{Type: 0},
	}
	data2, _ := json.Marshal(card)
	var m2 map[string]any
	_ = json.Unmarshal(data2, &m2)
	if _, ok := m2["source"]; !ok {
		t.Errorf("source should be present when set (json=%s)", data2)
	}
	if _, ok := m2["card_action"]; !ok {
		t.Errorf("card_action should be present when set (json=%s)", data2)
	}
}

// TestTemplateCardActionTypeRequired 验证 TemplateCardAction.Type 为必填：即便为 0 也保留。
func TestTemplateCardActionTypeRequired(t *testing.T) {
	data, err := json.Marshal(TemplateCardAction{Type: 0})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v, ok := m["type"]
	if !ok {
		t.Errorf("type must be present even when 0 (json=%s)", data)
	}
	if num, _ := v.(float64); num != 0 {
		t.Errorf("type = %v, want 0", v)
	}
	// 其余可选字段（url/appid/pagepath）为空应省略
	for _, k := range []string{"url", "appid", "pagepath"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s should be omitted when empty (json=%s)", k, data)
		}
	}
}

// TestTemplateCardRoundTrip 验证填充各字段的卡片 JSON 往返（含匿名结构切片）。
func TestTemplateCardRoundTrip(t *testing.T) {
	card := TemplateCard{
		CardType:     TemplateCardType.ButtonInteraction,
		SubTitleText: "副标题",
		MainTitle:    &TemplateCardMainTitle{Title: "主标题", Desc: "描述"},
		CardAction:   &TemplateCardAction{Type: 1, Url: "https://example.com"},
		ActionMenu: &TemplateCardActionMenu{
			Desc: "更多",
			ActionList: []struct {
				Text string `json:"text"`
				Key  string `json:"key"`
			}{{Text: "操作1", Key: "k1"}},
		},
		ButtonList: []TemplateCardButton{
			{Text: "确认", Key: "confirm", Style: 1},
		},
		TaskId:   "task_001",
		Feedback: &ReplyFeedback{Id: "fb_001"},
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TemplateCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// 重新序列化应稳定（结构等价）
	data2, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(data) != string(data2) {
		t.Errorf("round-trip not stable:\n got=%s\nwant=%s", data2, data)
	}
	if decoded.MainTitle == nil || decoded.MainTitle.Title != "主标题" {
		t.Errorf("MainTitle lost in round-trip: %+v", decoded.MainTitle)
	}
	if decoded.CardAction == nil || decoded.CardAction.Type != 1 {
		t.Errorf("CardAction.Type lost in round-trip: %+v", decoded.CardAction)
	}
	if decoded.TaskId != "task_001" {
		t.Errorf("TaskId = %q", decoded.TaskId)
	}
}

// ========== 回复消息体序列化 ==========

func TestReplyMsgItemSerialize(t *testing.T) {
	item := ReplyMsgItem{
		MsgType: "image",
	}
	item.Image.Base64 = "BASE64DATA"
	item.Image.Md5 = "MD5HASH"
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"msgtype":"image","image":{"base64":"BASE64DATA","md5":"MD5HASH"}}`
	if string(data) != want {
		t.Errorf("ReplyMsgItem json = %s, want %s", data, want)
	}
}

func TestStreamReplyBodySerialize(t *testing.T) {
	body := StreamReplyBody{
		MsgType: "stream",
		Stream: StreamReply{
			Id:       "stream_1",
			Finish:   true,
			Content:  "hello",
			Feedback: &ReplyFeedback{Id: "fb_1"},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["msgtype"] != "stream" {
		t.Errorf("msgtype = %v, want stream", m["msgtype"])
	}
	stream, _ := m["stream"].(map[string]any)
	if stream == nil || stream["id"] != "stream_1" {
		t.Errorf("stream.id mismatch: %v (json=%s)", stream, data)
	}
	if _, ok := stream["msg_item"]; ok {
		t.Errorf("msg_item should be omitted when empty (json=%s)", data)
	}

	// 仅 id（finish/content/feedback 省略）
	min, _ := json.Marshal(StreamReplyBody{MsgType: "stream", Stream: StreamReply{Id: "only_id"}})
	var mm map[string]any
	_ = json.Unmarshal(min, &mm)
	s2, _ := mm["stream"].(map[string]any)
	for _, k := range []string{"finish", "content", "feedback", "msg_item"} {
		if _, ok := s2[k]; ok {
			t.Errorf("stream.%s should be omitted when zero (json=%s)", k, min)
		}
	}
}

func TestReplyBodiesMsgType(t *testing.T) {
	cases := []struct {
		name string
		body any
		want string
	}{
		{"TemplateCardReplyBody", TemplateCardReplyBody{MsgType: "template_card"}, "template_card"},
		{"WelcomeTextReplyBody", WelcomeTextReplyBody{MsgType: "text"}, "text"},
		{"WelcomeTemplateCardReplyBody", WelcomeTemplateCardReplyBody{MsgType: "template_card"}, "template_card"},
		{"StreamWithTemplateCardReplyBody", StreamWithTemplateCardReplyBody{MsgType: "stream_with_template_card"}, "stream_with_template_card"},
		{"UpdateTemplateCardBody", UpdateTemplateCardBody{ResponseType: "update_template_card"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			// UpdateTemplateCardBody 用 response_type 而非 msgtype
			key := "msgtype"
			want := tc.want
			if tc.name == "UpdateTemplateCardBody" {
				key = "response_type"
				want = "update_template_card"
			}
			if m[key] != want {
				t.Errorf("%s %s = %v, want %q (json=%s)", tc.name, key, m[key], want, data)
			}
		})
	}
}

func TestStreamWithTemplateCardOptionalCard(t *testing.T) {
	// template_card 未设置（nil）→ 省略
	without := StreamWithTemplateCardReplyBody{
		MsgType: "stream_with_template_card",
		Stream:  StreamReply{Id: "s1"},
	}
	data, _ := json.Marshal(without)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	if _, ok := m["template_card"]; ok {
		t.Errorf("template_card should be omitted when nil (json=%s)", data)
	}

	// 设置 → 出现
	withCard := StreamWithTemplateCardReplyBody{
		MsgType:      "stream_with_template_card",
		Stream:       StreamReply{Id: "s1"},
		TemplateCard: &TemplateCard{CardType: TemplateCardType.TextNotice},
	}
	data2, _ := json.Marshal(withCard)
	var m2 map[string]any
	_ = json.Unmarshal(data2, &m2)
	if _, ok := m2["template_card"]; !ok {
		t.Errorf("template_card should be present when set (json=%s)", data2)
	}
}

func TestUpdateTemplateCardBodyUserIds(t *testing.T) {
	// userids 为空 → 省略；非空 → 出现
	empty, _ := json.Marshal(UpdateTemplateCardBody{ResponseType: "update_template_card"})
	var em map[string]any
	_ = json.Unmarshal(empty, &em)
	if _, ok := em["userids"]; ok {
		t.Errorf("userids should be omitted when empty (json=%s)", empty)
	}

	set, _ := json.Marshal(UpdateTemplateCardBody{
		ResponseType: "update_template_card",
		UserIds:      []string{"u1", "u2"},
	})
	var sm map[string]any
	_ = json.Unmarshal(set, &sm)
	v, _ := sm["userids"].([]any)
	if len(v) != 2 {
		t.Errorf("userids len = %d, want 2 (json=%s)", len(v), set)
	}
}

// keys 返回 map 的键集合（测试辅助）。
func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

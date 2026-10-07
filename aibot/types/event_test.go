package types

import (
	"encoding/json"
	"testing"
)

// eventFrameFixture 构造完整事件回调帧 JSON（body.event 为原始事件对象；body 层带模板卡
// 事件携带的 response_url）。
func eventFrameFixture(event string) []byte {
	return []byte(`{"cmd":"aibot_event_callback","headers":{"req_id":"r1"},"body":{"msgid":"m1","create_time":1,"aibotid":"a1","chattype":"single","from":{"userid":"u1"},"msgtype":"event","response_url":"https://qyapi.weixin.qq.com/cgi-bin/aibot/response?response_code=CODE","event":` + event + `}}`)
}

func TestDecodeTemplateCardEventSelectedItems(t *testing.T) {
	raw := eventFrameFixture(`{"eventtype":"template_card_event","event_key":"task_2026@01:submit","task_id":"task_2026@01","selected_items":{"selected_item":[{"question_key":"q0","option_ids":{"option_id":["task_2026@01:0","task_2026@01:2"]}},{"question_key":"q1","option_ids":{"option_id":["task_2026@01:5"]}}]}}`)
	var frame WsFrame[EventMessage]
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("解析事件帧失败: %v", err)
	}
	event, ok := frame.Body.DecodeEvent().(TemplateCardEventData)
	if !ok {
		t.Fatalf("DecodeEvent 未返回 TemplateCardEventData")
	}
	if event.EventKey != "task_2026@01:submit" || event.TaskId != "task_2026@01" {
		t.Fatalf("基础字段解码不符: %+v", event)
	}
	items := event.SelectedItems
	if items == nil || len(items.SelectedItem) != 2 {
		t.Fatalf("selected_items 解码不符: %+v", items)
	}
	first := items.SelectedItem[0]
	if first.QuestionKey != "q0" {
		t.Fatalf("首题 question_key 不符: %q", first.QuestionKey)
	}
	if first.OptionIds == nil || len(first.OptionIds.OptionId) != 2 || first.OptionIds.OptionId[0] != "task_2026@01:0" || first.OptionIds.OptionId[1] != "task_2026@01:2" {
		t.Fatalf("首题选中项不符: %+v", first.OptionIds)
	}
	second := items.SelectedItem[1]
	if second.QuestionKey != "q1" || second.OptionIds == nil || len(second.OptionIds.OptionId) != 1 || second.OptionIds.OptionId[0] != "task_2026@01:5" {
		t.Fatalf("次题解码不符: %+v", second)
	}
}

func TestWsFrameRawResponsePreserved(t *testing.T) {
	// RawResponse 保全企微原始帧：结构化解码有损（字段形态与文档不符时静默丢空），原文是
	// 排障第一手资料——任何入站帧反序列化后都应按帧携带原文。
	raw := eventFrameFixture(`{"eventtype":"template_card_event","event_key":"k:1","task_id":"k"}`)
	var frame WsFrame[EventMessage]
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("解析事件帧失败: %v", err)
	}
	if string(frame.RawResponse) != string(raw) {
		t.Fatalf("RawResponse 应与原始帧逐字节一致: %s", frame.RawResponse)
	}
}

func TestDecodeTemplateCardEventNestedForm(t *testing.T) {
	// 官方报文形态（智能机器人「接收事件」文档示例实证）：卡片数据嵌套在与 eventtype
	// 同名的二级键 template_card_event 下，二级对象不含 eventtype。
	raw := eventFrameFixture(`{"eventtype":"template_card_event","template_card_event":{"card_type":"vote_interaction","event_key":"wsbind--1a2b3c4d-9f8e7d6c:submit","task_id":"wsbind--1a2b3c4d-9f8e7d6c","selected_items":{"selected_item":[{"question_key":"wsbind--1a2b3c4d-9f8e7d6c","option_ids":{"option_id":["wsbind--1a2b3c4d-9f8e7d6c:0"]}}]}}}`)
	var frame WsFrame[EventMessage]
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("解析事件帧失败: %v", err)
	}
	event, ok := frame.Body.DecodeEvent().(TemplateCardEventData)
	if !ok {
		t.Fatalf("DecodeEvent 未返回 TemplateCardEventData")
	}
	if frame.Body.ResponseUrl != "https://qyapi.weixin.qq.com/cgi-bin/aibot/response?response_code=CODE" {
		t.Fatalf("body 层 response_url 应解码: %q", frame.Body.ResponseUrl)
	}
	if event.EventType != EventType.TemplateCardEvent {
		t.Fatalf("嵌套形态应回填 eventtype: %q", event.EventType)
	}
	if event.EventKey != "wsbind--1a2b3c4d-9f8e7d6c:submit" || event.TaskId != "wsbind--1a2b3c4d-9f8e7d6c" {
		t.Fatalf("嵌套字段解码不符: %+v", event)
	}
	items := event.SelectedItems
	if items == nil || len(items.SelectedItem) != 1 {
		t.Fatalf("嵌套 selected_items 解码不符: %+v", items)
	}
	if item := items.SelectedItem[0]; item.QuestionKey != "wsbind--1a2b3c4d-9f8e7d6c" ||
		item.OptionIds == nil || len(item.OptionIds.OptionId) != 1 || item.OptionIds.OptionId[0] != "wsbind--1a2b3c4d-9f8e7d6c:0" {
		t.Fatalf("嵌套选中项不符: %+v", item)
	}
}

func TestDecodeTemplateCardEventWithoutSelectedItems(t *testing.T) {
	// 纯点击型事件（无提交按钮的卡）无 selected_items 字段：不报错、字段为零值。
	raw := eventFrameFixture(`{"eventtype":"template_card_event","event_key":"task_2026@01:1","task_id":"task_2026@01"}`)
	var frame WsFrame[EventMessage]
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("解析事件帧失败: %v", err)
	}
	event, ok := frame.Body.DecodeEvent().(TemplateCardEventData)
	if !ok {
		t.Fatalf("DecodeEvent 未返回 TemplateCardEventData")
	}
	if event.EventKey != "task_2026@01:1" {
		t.Fatalf("event_key 不符: %q", event.EventKey)
	}
	if event.SelectedItems != nil {
		t.Fatalf("无 selected_items 的事件应解码为零值: %+v", event.SelectedItems)
	}
}

func TestDecodeTemplateCardEventSelectedItemsPartial(t *testing.T) {
	// 缺字段容错：选中项缺 option_ids、selected_item 为空数组均不报错。
	raw := eventFrameFixture(`{"eventtype":"template_card_event","event_key":"t:submit","task_id":"t","selected_items":{"selected_item":[{"question_key":"q0"}]}}`)
	var frame WsFrame[EventMessage]
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("解析事件帧失败: %v", err)
	}
	event := frame.Body.DecodeEvent().(TemplateCardEventData)
	if event.SelectedItems == nil || len(event.SelectedItems.SelectedItem) != 1 {
		t.Fatalf("selected_items 解码不符: %+v", event.SelectedItems)
	}
	item := event.SelectedItems.SelectedItem[0]
	if item.QuestionKey != "q0" || item.OptionIds != nil {
		t.Fatalf("缺 option_ids 的选中项应为零值: %+v", item)
	}
}

package types

// event.go 对应 Node src/types/event.ts：事件类型（EventMessage 及各类 EventData）。

import "encoding/json"

// ========== 事件类型常量 ==========

// EventType 事件类型常量，对应 Node EventType enum。
var EventType = struct {
	EnterChat         string // 进入会话事件：用户当天首次进入机器人单聊会话
	TemplateCardEvent string // 模板卡片事件：用户点击模板卡片按钮
	FeedbackEvent     string // 用户反馈事件：用户对机器人回复进行反馈
	Disconnected      string // 连接断开事件：有新连接建立时，服务端向旧连接发送此事件并主动断开
}{
	EnterChat:         "enter_chat",
	TemplateCardEvent: "template_card_event",
	FeedbackEvent:     "feedback_event",
	Disconnected:      "disconnected_event",
}

// ========== 事件子结构 ==========

// EventFrom 事件发送者信息，对应 Node EventFrom（比 MessageFrom 多了 CorpId 字段）。
type EventFrom struct {
	UserId string `json:"userid"`           // 事件触发者的 userid
	CorpId string `json:"corpid,omitempty"` // 事件触发者的 corpid，企业内部机器人不返回
}

// EventContent 事件内容接口，对应 Node EventContent 联合类型。
//
// 各事件数据类型实现此接口。Go 中用接口近似 Node 联合类型，
// JSON 反序列化时需通过 EventMessage.DecodeEvent() 获取具体类型。
type EventContent interface {
	GetEventType() string
}

// EnterChatEvent 进入会话事件，对应 Node EnterChatEvent。
type EnterChatEvent struct {
	EventType string `json:"eventtype"` // 事件类型，固定值 enter_chat
}

// GetEventType 返回事件类型，实现 EventContent 接口。
func (e EnterChatEvent) GetEventType() string { return e.EventType }

// TemplateCardEventData 模板卡片事件，对应 Node TemplateCardEventData。
type TemplateCardEventData struct {
	EventType string `json:"eventtype"`           // 事件类型，固定值 template_card_event
	EventKey  string `json:"event_key,omitempty"` // 用户点击的按钮 key
	TaskId    string `json:"task_id,omitempty"`   // 任务 ID
}

// GetEventType 返回事件类型，实现 EventContent 接口。
func (e TemplateCardEventData) GetEventType() string { return e.EventType }

// FeedbackEventData 用户反馈事件，对应 Node FeedbackEventData。
type FeedbackEventData struct {
	EventType string `json:"eventtype"` // 事件类型，固定值 feedback_event
}

// GetEventType 返回事件类型，实现 EventContent 接口。
func (e FeedbackEventData) GetEventType() string { return e.EventType }

// DisconnectedEventData 连接断开事件，对应 Node DisconnectedEventData。
//
// 有新连接建立时，服务端向旧连接推送此事件并主动断开旧连接。
type DisconnectedEventData struct {
	EventType string `json:"eventtype"` // 事件类型，固定值 disconnected_event
}

// GetEventType 返回事件类型，实现 EventContent 接口。
func (e DisconnectedEventData) GetEventType() string { return e.EventType }

// ========== 事件消息 ==========

// EventMessage 事件回调消息结构，对应 Node EventMessage。
//
// Event 字段为接口类型，JSON 反序列化后需调用 DecodeEvent() 获取具体事件类型。
type EventMessage struct {
	MsgId      string          `json:"msgid"`              // 本次回调的唯一性标志，用于事件排重
	CreateTime int             `json:"create_time"`        // 事件产生的时间戳
	AibotId    string          `json:"aibotid"`            // 智能机器人 id
	ChatId     string          `json:"chatid,omitempty"`   // 会话 id，仅群聊类型时返回
	ChatType   string          `json:"chattype,omitempty"` // 会话类型：single 单聊, group 群聊
	From       EventFrom       `json:"from"`               // 事件触发者信息
	MsgType    string          `json:"msgtype"`            // 消息类型，事件回调固定为 event
	Event      EventContent    `json:"-"`                  // 事件内容（DecodeEvent 后填充）
	rawEvent   json.RawMessage // 原始 event JSON（供 DecodeEvent 使用）
}

// UnmarshalJSON 自定义反序列化，将 event 字段暂存为 RawMessage 供 DecodeEvent 使用。
func (m *EventMessage) UnmarshalJSON(data []byte) error {
	// 用别名避免递归调用 UnmarshalJSON。
	type alias EventMessage
	var tmp struct {
		alias
		RawEvent json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*m = EventMessage(tmp.alias)
	m.rawEvent = tmp.RawEvent
	return nil
}

// DecodeEvent 按 eventtype 将原始 event JSON 解码为具体事件类型并填充 Event 字段，
// 对应 Node 中 EventContent 联合类型的自动推断。
//
// 返回解码后的事件内容，若 eventtype 未知则返回 nil。
func (m *EventMessage) DecodeEvent() EventContent {
	if len(m.rawEvent) == 0 {
		return nil
	}
	// 先提取 eventtype 决定具体类型。
	var probe struct {
		EventType string `json:"eventtype"`
	}
	if err := json.Unmarshal(m.rawEvent, &probe); err != nil {
		return nil
	}
	switch probe.EventType {
	case EventType.EnterChat:
		var e EnterChatEvent
		if json.Unmarshal(m.rawEvent, &e) == nil {
			m.Event = e
			return e
		}
	case EventType.TemplateCardEvent:
		var e TemplateCardEventData
		if json.Unmarshal(m.rawEvent, &e) == nil {
			m.Event = e
			return e
		}
	case EventType.FeedbackEvent:
		var e FeedbackEventData
		if json.Unmarshal(m.rawEvent, &e) == nil {
			m.Event = e
			return e
		}
	case EventType.Disconnected:
		var e DisconnectedEventData
		if json.Unmarshal(m.rawEvent, &e) == nil {
			m.Event = e
			return e
		}
	}
	return nil
}

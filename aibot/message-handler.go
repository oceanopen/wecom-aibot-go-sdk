package aibot

// message-handler.go 对应 Node src/message-handler.ts：MessageHandler 消息/事件分发处理器。
//
// 任务 15：MessageHandler 结构 + 构造。
// 任务 16：按 cmd+msgtype 二次解码分发（文本 OnText + 事件 OnEvent）。
// 任务 19：补全其余消息类型（OnImage/OnMixed/OnVoice/OnFile/OnVideo）与类型化事件分发。

import (
	"encoding/json"
	"fmt"

	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// MessageHandler 消息处理器，对应 Node MessageHandler。
//
// 负责解析 WebSocket 帧并分发为具体的消息事件和事件回调。
type MessageHandler struct {
	logger types.Logger
}

// NewMessageHandler 构造 MessageHandler，对应 Node MessageHandler constructor。
func NewMessageHandler(logger types.Logger) *MessageHandler {
	return &MessageHandler{logger: logger}
}

// HandleFrame 处理收到的 WebSocket 帧，对应 Node MessageHandler.handleFrame。
//
// 接收帧结构：
//   - 消息推送：{ cmd: "aibot_msg_callback", headers: { req_id }, body: { msgid, msgtype, ... } }
//   - 事件推送：{ cmd: "aibot_event_callback", headers: { req_id }, body: { msgid, msgtype: "event", event: { ... } } }
//
// 先用探针解析 cmd + body.msgtype 路由：事件推送 → handleEventCallback；消息推送 → handleMessageCallback。
// 任务 16 实现文本（OnText）与事件（OnEvent）分发，任务 19 补全其余类型。
func (h *MessageHandler) HandleFrame(raw json.RawMessage, client *WsClient) {
	// 探针：解析 cmd 与 body.msgtype（避免对未知帧整体解码）
	var probe struct {
		Cmd  string `json:"cmd,omitempty"`
		Body struct {
			MsgType string `json:"msgtype"`
		} `json:"body"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse message frame: %s", err.Error()))
		return
	}

	// body 或 msgtype 缺失视为非法格式，忽略（不触发回调）
	if probe.Body.MsgType == "" {
		h.logger.Warn(fmt.Sprintf("Received invalid message format: %s", truncateFrame(raw, 200)))
		return
	}

	// 事件推送
	if probe.Cmd == types.WsCmd.EventCallback {
		h.handleEventCallback(raw, client)
		return
	}

	// 消息推送
	h.handleMessageCallback(raw, client, probe.Body.MsgType)
}

// handleMessageCallback 处理消息推送（aibot_msg_callback），对应 Node handleMessageCallback。
//
// 先触发通用 OnMessage（body 解析为 BaseMessage），再按 msgtype 触发类型化回调。
// 任务 16 仅实现文本（OnText）；其余类型（OnImage/OnMixed/...）任务 19 补全。
func (h *MessageHandler) handleMessageCallback(raw json.RawMessage, client *WsClient, msgtype string) {
	// 通用消息事件：以 BaseMessage 解析
	var baseFrame types.WsFrame[types.BaseMessage]
	if err := json.Unmarshal(raw, &baseFrame); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse message body: %s", err.Error()))
		return
	}
	if client.OnMessage != nil {
		client.OnMessage(&baseFrame)
	}

	// 按 msgtype 触发特定事件
	switch msgtype {
	case types.MessageType.Text:
		if client.OnText != nil {
			var textFrame types.WsFrame[types.TextMessage]
			if err := json.Unmarshal(raw, &textFrame); err != nil {
				h.logger.Error(fmt.Sprintf("Failed to parse text message: %s", err.Error()))
				return
			}
			client.OnText(&textFrame)
		}
	default:
		h.logger.Debug(fmt.Sprintf("Received unhandled message type: %s", msgtype))
	}
}

// handleEventCallback 处理事件推送（aibot_event_callback），对应 Node handleEventCallback。
//
// 触发通用 OnEvent（body 解析为 EventMessage）。任务 19 补全类型化事件分发（OnEnterChat 等）。
func (h *MessageHandler) handleEventCallback(raw json.RawMessage, client *WsClient) {
	var eventFrame types.WsFrame[types.EventMessage]
	if err := json.Unmarshal(raw, &eventFrame); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse event body: %s", err.Error()))
		return
	}
	if client.OnEvent != nil {
		client.OnEvent(&eventFrame)
	}
}

// truncateFrame 截断帧 JSON 用于日志，对应 Node JSON.stringify(frame).substring(0, 200)。
func truncateFrame(raw json.RawMessage, max int) string {
	s := string(raw)
	if len(s) > max {
		return s[:max]
	}
	return s
}

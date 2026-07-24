package aibot

// message-handler.go 对应 Node src/message-handler.ts：MessageHandler 消息/事件分发处理器。
//
// 持有 WsClient 引用，按 cmd + msgtype/eventtype 二次解码后路由到类型化回调：
// 文本/图片/图文/语音/文件/视频消息与进入会话/模板卡片/反馈/断开事件。

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
		dispatchTyped(client.OnText, raw, h.logger, "text message")
	case types.MessageType.Image:
		dispatchTyped(client.OnImage, raw, h.logger, "image message")
	case types.MessageType.Mixed:
		dispatchTyped(client.OnMixed, raw, h.logger, "mixed message")
	case types.MessageType.Voice:
		dispatchTyped(client.OnVoice, raw, h.logger, "voice message")
	case types.MessageType.File:
		dispatchTyped(client.OnFile, raw, h.logger, "file message")
	case types.MessageType.Video:
		dispatchTyped(client.OnVideo, raw, h.logger, "video message")
	default:
		h.logger.Debug(fmt.Sprintf("Received unhandled message type: %s", msgtype))
	}
}

// handleEventCallback 处理事件推送（aibot_event_callback），对应 Node handleEventCallback。
//
// 先触发通用 OnEvent（body 解析为 EventMessage），再按 eventtype 触发类型化事件回调。
// DecodeEvent 同时填充 eventFrame.Body.Event，回调内可直接访问。
func (h *MessageHandler) handleEventCallback(raw json.RawMessage, client *WsClient) {
	var eventFrame types.WsFrame[types.EventMessage]
	if err := json.Unmarshal(raw, &eventFrame); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse event body: %s", err.Error()))
		return
	}

	// 通用事件回调
	if client.OnEvent != nil {
		client.OnEvent(&eventFrame)
	}

	// 按 eventtype 触发类型化事件（DecodeEvent 返回具体事件类型并填充 Body.Event）
	event := eventFrame.Body.DecodeEvent()
	eventType := ""
	if event != nil {
		eventType = event.GetEventType()
	}
	switch eventType {
	case types.EventType.EnterChat:
		if client.OnEnterChat != nil {
			client.OnEnterChat(&eventFrame)
		}
	case types.EventType.TemplateCardEvent:
		if client.OnTemplateCardEvent != nil {
			client.OnTemplateCardEvent(&eventFrame)
		}
	case types.EventType.FeedbackEvent:
		if client.OnFeedbackEvent != nil {
			client.OnFeedbackEvent(&eventFrame)
		}
	case types.EventType.Disconnected:
		if client.OnDisconnectedEvent != nil {
			client.OnDisconnectedEvent(&eventFrame)
		}
	case "":
		h.logger.Debug(fmt.Sprintf("Received event callback without eventtype: %s", truncateFrame(raw, 200)))
	default:
		h.logger.Debug(fmt.Sprintf("Received unhandled event type: %s", eventType))
	}
}

// dispatchTyped 将原始帧解码为 WsFrame[T] 并触发回调（回调为 nil 时跳过），减少各 msgtype 分支的重复代码。
func dispatchTyped[T any](cb func(*types.WsFrame[T]), raw json.RawMessage, logger types.Logger, label string) {
	if cb == nil {
		return
	}
	var frame types.WsFrame[T]
	if err := json.Unmarshal(raw, &frame); err != nil {
		logger.Error(fmt.Sprintf("Failed to parse %s: %s", label, err.Error()))
		return
	}
	cb(&frame)
}

// truncateFrame 截断帧 JSON 用于日志，对应 Node JSON.stringify(frame).substring(0, 200)。
func truncateFrame(raw json.RawMessage, max int) string {
	s := string(raw)
	if len(s) > max {
		return s[:max]
	}
	return s
}

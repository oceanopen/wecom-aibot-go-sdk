package aibot

// message-handler.go 对应 Node src/message-handler.ts：MessageHandler 消息/事件分发处理器。
//
// 任务 15：MessageHandler 结构 + 构造 + 最小 HandleFrame（通用 OnMessage 透传）。
// 任务 16：按 cmd+msgtype 二次解码分发类型化回调（OnText/OnImage/OnEvent 等）。
// 任务 19：补全其余消息/事件回调分发。

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
// 任务 15：仅做通用消息透传（触发 OnMessage，body 解析为 BaseMessage）。
// 任务 16 实现 cmd+msgtype 二次解码与类型化分发（OnText/OnImage/OnEvent 等）。
func (h *MessageHandler) HandleFrame(raw json.RawMessage, client *WsClient) {
	var frame types.WsFrame[types.BaseMessage]
	if err := json.Unmarshal(raw, &frame); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse message frame: %s", err.Error()))
		return
	}
	if client.OnMessage != nil {
		client.OnMessage(&frame)
	}
}

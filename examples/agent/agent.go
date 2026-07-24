// agent.go 实现 agent 主循环：调用 Claude（带工具）→ 执行工具 → 流式推送结果到企业微信。
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
)

// agentStreamState agent 主循环与推送 ticker 共享的可变状态。
// 仅展示当前状态文本或流式文本，不保留历史。
type agentStreamState struct {
	mu          sync.Mutex
	statusText  string // 当前工具状态（覆盖式，非累加）
	currentText string // 最终回合的流式文本
}

func (s *agentStreamState) fullContent() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentText != "" {
		return s.currentText
	}
	return s.statusText
}

func (s *agentStreamState) setStatus(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusText = text
}

func (s *agentStreamState) appendText(delta string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentText += delta
}

func (s *agentStreamState) clearCurrentText() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentText = ""
}

func (s *agentStreamState) setErrorText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentText = text
}

// RunAgent 执行 agent 循环：带工具调用 Claude，执行工具调用，
// 将结果以单条不断刷新的流式消息推送到企业微信。
func RunAgent(
	ctx context.Context,
	ai *anthropic.Client,
	wsClient *aibot.WsClient,
	frame *aibot.WsFrame[aibot.TextMessage],
	cfg *Config,
	security *SecurityPolicy,
	userMessage string,
) {
	streamId := aibot.GenerateReqId("stream")
	state := &agentStreamState{}
	executor := NewToolExecutor(security)
	tools := ToolDefinitions()

	done := make(chan struct{})
	finishDone := make(chan struct{})

	// ticker goroutine：周期性把内容推送到企业微信，是唯一调用 ReplyStream 的 goroutine。
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		var lastSent string

		for {
			select {
			case <-ticker.C:
				current := state.fullContent()
				if current != "" && current != lastSent {
					_, _ = wsClient.ReplyStream(frame.Headers, streamId, current, false, nil, nil)
					lastSent = current
				}
			case <-done:
				finalContent := state.fullContent()
				if finalContent == "" {
					finalContent = "处理完成"
				}
				_, _ = wsClient.ReplyStream(frame.Headers, streamId, finalContent, true, nil, nil)
				close(finishDone)
				return
			}
		}
	}()

	// 初始消息
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	}

	// 系统提示
	var system []anthropic.TextBlockParam
	if cfg.SystemPrompt != "" {
		system = []anthropic.TextBlockParam{{Text: cfg.SystemPrompt}}
	}

	// agent 循环
	for turn := 0; turn < cfg.MaxTurns; turn++ {
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(cfg.Model),
			MaxTokens: cfg.MaxTokens,
			Messages:  messages,
			Tools:     tools,
		}
		if len(system) > 0 {
			params.System = system
		}

		stream := ai.Messages.NewStreaming(ctx, params)

		var accMessage anthropic.Message
		turnHasToolUse := false

		for stream.Next() {
			event := stream.Current()
			_ = accMessage.Accumulate(event)

			switch evt := event.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				if evt.ContentBlock.Type == "tool_use" {
					turnHasToolUse = true
				}
			case anthropic.ContentBlockDeltaEvent:
				if delta, ok := evt.Delta.AsAny().(anthropic.TextDelta); ok {
					if !turnHasToolUse {
						state.appendText(delta.Text)
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			state.setErrorText(fmt.Sprintf("AI 服务错误: %v", err))
			break
		}

		// 检查停止原因
		if accMessage.StopReason == anthropic.StopReasonToolUse {
			// 丢弃 tool_use 前泄漏的思考文本
			state.clearCurrentText()

			// assistant 回合入历史
			messages = append(messages, accMessage.ToParam())

			// 执行每个工具调用
			var toolResults []anthropic.ContentBlockParamUnion
			for _, block := range accMessage.Content {
				toolUse, ok := block.AsAny().(anthropic.ToolUseBlock)
				if !ok {
					continue
				}

				state.setStatus(fmt.Sprintf("🔧 正在执行 %s...", toolUse.Name))

				result, isError := executor.Execute(ctx, toolUse.Name, toolUse.Input)

				toolResults = append(toolResults,
					anthropic.NewToolResultBlock(toolUse.ID, result, isError))
			}

			// 工具结果作为 user 消息回灌
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
			continue
		}

		// end_turn / max_tokens / 其它：文本已在 currentText
		break
	}

	close(done)
	<-finishDone
}

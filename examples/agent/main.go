// examples/agent 演示基于企业微信智能机器人 SDK 的 AI Agent：
// 接收文本消息 → 调用 Claude（带工具：读写文件、受限 bash）→ 流式回复。
//
// 配置见 config.example.json。运行：go run . [path/to/config.json]
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
)

func main() {
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Println(err)
		fmt.Println("请创建 config.json，参考 config.example.json")
		return
	}

	security, err := NewSecurityPolicy(cfg.WorkingDir, cfg.BashPatterns)
	if err != nil {
		fmt.Println("安全策略初始化失败:", err)
		return
	}

	fmt.Println("企业微信 AI Agent")
	fmt.Println("Bot ID:", cfg.BotId)
	fmt.Println("Model:", cfg.Model)
	fmt.Println("Working Dir:", cfg.WorkingDir)

	// Anthropic client（可选自定义 BaseURL，用于代理或兼容网关）
	aiOpts := []option.RequestOption{
		option.WithAPIKey(cfg.AnthropicApiKey),
	}
	if cfg.AnthropicBaseUrl != "" {
		aiOpts = append(aiOpts, option.WithBaseURL(cfg.AnthropicBaseUrl))
	}
	ai := anthropic.NewClient(aiOpts...)

	// 企业微信 WebSocket 客户端
	client := aibot.NewWsClient(aibot.WsClientOptions{
		BotId:  cfg.BotId,
		Secret: cfg.BotSecret,
	})

	// 连接生命周期回调
	client.OnConnected = func() {
		fmt.Println("连接已建立")
	}
	client.OnAuthenticated = func() {
		fmt.Println("认证成功")
	}
	client.OnDisconnected = func(reason string) {
		fmt.Println("连接断开:", reason)
	}
	client.OnReconnecting = func(attempt int) {
		fmt.Printf("正在重连（第 %d 次）...\n", attempt)
	}
	client.OnError = func(err error) {
		fmt.Println("错误:", err.Error())
	}

	// 文本消息 → agent 循环（每条消息独立 goroutine）
	client.OnText = func(frame *aibot.WsFrame[aibot.TextMessage]) {
		fmt.Printf("收到文本: %s\n", frame.Body.Text.Content)
		go RunAgent(
			context.Background(),
			&ai,
			client,
			frame,
			cfg,
			security,
			frame.Body.Text.Content,
		)
	}

	// 进入会话 → 欢迎语
	client.OnEnterChat = func(frame *aibot.WsFrame[aibot.EventMessage]) {
		fmt.Printf("用户进入会话: %s\n", frame.Body.From.UserId)
		var welcomeBody aibot.WelcomeTextReplyBody
		welcomeBody.MsgType = "text"
		welcomeBody.Text.Content = "你好！我是 AI 助手，可以帮你读写文件和执行命令。"
		_, _ = client.ReplyWelcome(frame.Headers, welcomeBody)
	}

	// SIGINT/SIGTERM 触发优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Connect 阻塞至首次认证成功，放在 goroutine 中以便主线程等待退出信号
	go func() {
		if err := client.Connect(ctx); err != nil {
			fmt.Printf("连接结束: %v\n", err)
		}
	}()

	fmt.Println("\n按 Ctrl+C 退出")
	<-ctx.Done()
	fmt.Println("\n正在断开连接...")
	client.Disconnect()
	fmt.Println("已退出")
}

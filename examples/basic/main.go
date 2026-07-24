// examples/basic 对应 Node examples/basic.ts：企业微信智能机器人 SDK 最小闭环示例。
//
// 覆盖：连接 → 认证 → 收文本 → 流式回复 → 优雅退出（SIGINT）。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
)

func main() {
	// 凭证优先取环境变量，便于不提交密钥；缺省占位需替换为真实值
	botId := envOr("WECOM_BOT_ID", "your-bot-id")
	secret := envOr("WECOM_BOT_SECRET", "your-bot-secret")

	// 创建 WsClient 实例
	client := aibot.NewWsClient(aibot.WsClientOptions{
		BotId:  botId,
		Secret: secret,
	})

	// 连接生命周期回调
	client.OnConnected = func() {
		fmt.Println("✅ WebSocket 已连接")
	}
	client.OnAuthenticated = func() {
		fmt.Println("🔐 认证成功")
	}
	client.OnDisconnected = func(reason string) {
		fmt.Printf("❌ 连接断开: %s\n", reason)
	}
	client.OnReconnecting = func(attempt int) {
		fmt.Printf("🔄 正在进行第 %d 次重连...\n", attempt)
	}
	client.OnError = func(err error) {
		fmt.Printf("⚠️ 发生错误: %v\n", err)
	}

	// 收到任意消息（所有类型）
	client.OnMessage = func(frame *aibot.WsFrame[aibot.BaseMessage]) {
		fmt.Printf("📨 收到消息: msgtype=%s, msgid=%s\n", frame.Body.MsgType, frame.Body.MsgId)
	}

	// 收到文本消息：使用流式回复
	client.OnText = func(frame *aibot.WsFrame[aibot.TextMessage]) {
		content := frame.Body.Text.Content
		fmt.Printf("📝 收到文本消息: %s\n", content)

		// 生成流式消息 ID（同一会话内多次刷新使用相同 ID）
		streamId := aibot.GenerateReqId("stream")

		// 发送流式中间帧（finish=false）
		if _, err := client.ReplyStream(frame.Headers, streamId, "正在思考...", false, nil, nil); err != nil {
			fmt.Printf("流式中间帧失败: %v\n", err)
			return
		}

		// 模拟异步处理后发送最终结果
		time.Sleep(500 * time.Millisecond)
		reply := fmt.Sprintf("你好！你说的是：%s", content)
		if _, err := client.ReplyStream(frame.Headers, streamId, reply, true, nil, nil); err != nil {
			fmt.Printf("流式最终帧失败: %v\n", err)
			return
		}
		fmt.Println("✅ 流式回复完成")
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

	// 等待退出信号
	<-ctx.Done()
	fmt.Println("\n正在停止机器人...")
	client.Disconnect()
}

// envOr 读取环境变量，缺省返回 fallback。
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

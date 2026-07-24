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
	botId := envOr("WECOM_BOT_ID", "")
	secret := envOr("WECOM_BOT_SECRET", "")

	// TODO: 增加 botId 和 secret 的非空校验
	if botId == "" || secret == "" {
		fmt.Println("❌ 请设置 WECOM_BOT_ID 和 WECOM_BOT_SECRET 环境变量")
		return
	}

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

		// 拷贝 headers（值类型）供 goroutine 安全使用，避免持有回调帧指针
		headers := frame.Headers
		// 生成流式消息 ID（同一会话内多次刷新使用相同 ID）
		streamId := aibot.GenerateReqId("stream")

		// 流式回复在后台发送，避免阻塞消息回调。
		// 服务端对流式中间帧（finish=false）的 ack 固有延迟约 5s，紧贴 replyAckTimeout(5000ms)，
		// 不应视为致命错误：「正在思考...」一经发出即展示给用户，中间帧 ack 超时仅记录，不影响最终帧。
		// （镜像 Node examples/basic.ts：流式帧 fire-and-forget，不 await 中间帧 ack）
		go func() {
			// 中间帧：ack 超时可忽略
			if _, err := client.ReplyStream(headers, streamId, "正在思考...", false, nil, nil); err != nil {
				fmt.Printf("（中间帧 ack 未及时确认，可忽略）: %v\n", err)
			}

			// 模拟异步处理后发送最终结果
			time.Sleep(500 * time.Millisecond)
			reply := fmt.Sprintf("你好！你说的是：%s", content)
			if _, err := client.ReplyStream(headers, streamId, reply, true, nil, nil); err != nil {
				fmt.Printf("流式最终帧失败: %v\n", err)
				return
			}
			fmt.Println("✅ 流式回复完成")
		}()
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

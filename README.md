# wecom-aibot-go-sdk

企业微信智能机器人 **Go SDK**，代码层面 1:1 镜像 [Node SDK](https://github.com/WecomTeam/aibot-node-sdk)（文件结构、命名、拆分与逻辑保持一致）。

> 文档：https://developer.work.weixin.qq.com/document/path/101463

## 状态

当前已打通**最小闭环**：连接 → 认证 → 收文本 → 流式回复 → 优雅退出。文件下载、模板卡片、主动发送、分片上传等能力在后续版本逐步补全（见 `task.md`）。

## 安装

要求 **Go 1.24+**（重新导出泛型 `WsFrame[T]` 依赖 1.24 泛型类型别名）。

```bash
go get github.com/oceanopen/wecom-aibot-go-sdk
```

## 快速开始

完整可运行示例见 [`examples/basic`](./examples/basic/main.go)。填入凭证后即可连接真实服务器：

```bash
# 设置凭证（避免提交密钥）
export WECOM_BOT_ID=your-bot-id
export WECOM_BOT_SECRET=your-bot-secret

go run ./examples/basic
```

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
)

func main() {
	client := aibot.NewWsClient(aibot.WsClientOptions{
		BotId:  os.Getenv("WECOM_BOT_ID"),
		Secret: os.Getenv("WECOM_BOT_SECRET"),
	})

	// 连接生命周期回调
	client.OnAuthenticated = func() { fmt.Println("🔐 认证成功") }
	client.OnDisconnected = func(reason string) { fmt.Printf("❌ 断开: %s\n", reason) }

	// 收到文本消息：使用流式回复
	client.OnText = func(frame *aibot.WsFrame[aibot.TextMessage]) {
		content := frame.Body.Text.Content
		streamId := aibot.GenerateReqId("stream")
		_, _ = client.ReplyStream(frame.Headers, streamId, "正在思考...", false, nil, nil)
		_, _ = client.ReplyStream(frame.Headers, streamId, fmt.Sprintf("你好：%s", content), true, nil, nil)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := client.Connect(ctx); err != nil {
			fmt.Printf("连接结束: %v\n", err)
		}
	}()

	<-ctx.Done() // 等待 SIGINT/SIGTERM
	client.Disconnect()
}
```

## 配置

`NewWsClient(opts aibot.WsClientOptions)` 接收结构体配置（**不使用**函数式 `With*` 选项）。零值字段由客户端兜底默认：

| 字段 | 说明 | 默认值 |
|---|---|---|
| `BotId` / `Secret` | 机器人 ID / Secret（企业微信后台获取） | 必填 |
| `Scene` / `PlugVersion` | 场景值 / 插件版本（可选，非零/非空时透传认证帧） | — |
| `HeartbeatInterval` | 心跳间隔（毫秒） | `30000` |
| `ReconnectInterval` | 重连基础延迟（毫秒，指数退避） | `1000` |
| `MaxReconnectAttempts` | 连接断开最大重连次数，`-1` 表示无限 | `10` |
| `MaxAuthFailureAttempts` | 认证失败最大重试次数，`-1` 表示无限 | `5` |
| `MaxReplyQueueSize` | 单 `req_id` 回复队列最大长度 | `500` |
| `RequestTimeout` | 请求超时（毫秒） | `10000` |
| `WsUrl` | 自定义 WebSocket 地址 | `wss://openws.work.weixin.qq.com` |
| `WsOptions` | 底层 WebSocket 选项（TLS/Header 等） | — |
| `Logger` | 自定义日志实现（`aibot.Logger`） | `DefaultLogger` |

## 核心 API（最小闭环相关）

| 方法 | 说明 |
|---|---|
| `NewWsClient(opts) *WsClient` | 构造客户端 |
| `Connect(ctx) error` | 建立连接并阻塞至首次认证成功 |
| `Disconnect()` | 主动断开 |
| `IsConnected() bool` | 当前连接状态 |
| `ReplyStream(frame.Headers, streamId, content, finish, msgItem, feedback)` | 流式回复（首参为 `WsFrameHeaders`） |
| `ReplyStreamNonBlocking(...)` | 非阻塞流式回复（上一条未 ack 且非最终帧返回 `ErrReplySkipped`） |
| `Reply(frame.Headers, body, cmd)` | 通用回复 |
| `OnText` / `OnMessage` / `OnEvent` / ... | 消息与事件回调 |
| `OnConnected` / `OnAuthenticated` / `OnDisconnected` / `OnReconnecting` / `OnError` | 生命周期回调 |

## 构建/测试

```bash
go build ./...        # 构建
go test ./...         # 测试（含本地 mock WebSocket 服务端集成测试，无需真实凭证）
go run ./examples/basic
```

## License

MIT

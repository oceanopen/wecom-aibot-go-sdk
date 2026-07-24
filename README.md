# wecom-aibot-go-sdk

企业微信智能机器人 **Go SDK**，代码层面 1:1 镜像 [Node SDK](https://github.com/WecomTeam/aibot-node-sdk)（文件结构、命名、拆分与逻辑保持一致）。

> 文档：https://developer.work.weixin.qq.com/document/path/101463

## 特性

- **WebSocket 长连接**：拨号 → 认证 → 心跳 → 指数退避重连（认证/断开两套计数器，`-1` 无限）
- **消息/事件分发**：文本/图片/图文/语音/文件/视频 + 进入会话/模板卡片/反馈/被踢下线事件
- **回复通道**：流式回复（阻塞/非阻塞）、欢迎语、模板卡片、流式+卡片组合、更新卡片
- **主动发送**：Markdown / 模板卡片 / 媒体（file/image/voice/video）
- **分片上传**：`UploadMedia` 三步上传（512KB/片、动态并发、单片重试、超大拒绝）
- **文件下载解密**：`DownloadFile` 下载 + AES-256-CBC 解密（PKCS#7 块 32）
- **Webhook 加解密**：`WecomCrypto`（EncodingAesKey / SHA1 签名 / AES-256-CBC）
- **可注入日志**：实现 `aibot.Logger` 接口即可

## 安装

要求 **Go 1.24+**（重新导出泛型 `WsFrame[T]` 依赖 1.24 泛型类型别名）。

```bash
go get github.com/oceanopen/wecom-aibot-go-sdk
```

```go
import (
	aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
)
// 使用：aibot.WsClient / aibot.NewWsClient / aibot.WsFrame[*aibot.TextMessage] ...
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

	// 收到文本消息：流式回复（中间帧 + 最终帧）
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
| `ReconnectInterval` | 重连基础延迟（毫秒，指数退避 1s→30s） | `1000` |
| `MaxReconnectAttempts` | 连接断开最大重连次数，`-1` 表示无限 | `10` |
| `MaxAuthFailureAttempts` | 认证失败最大重试次数，`-1` 表示无限 | `5` |
| `MaxReplyQueueSize` | 单 `req_id` 回复队列最大长度 | `500` |
| `RequestTimeout` | HTTP 请求超时（毫秒） | `10000` |
| `WsUrl` | 自定义 WebSocket 地址 | 内置默认地址 |
| `WsOptions` | 底层 WebSocket 选项（TLS/Header 等） | — |
| `Logger` | 自定义日志实现（`aibot.Logger`） | `DefaultLogger` |

## 核心 API

回复类方法首参均为 `WsFrameHeaders`，调用方传 `frame.Headers`（镜像 Node `reply(frame)`）。

### 连接与生命周期

| 方法 | 说明 |
|---|---|
| `NewWsClient(opts) *WsClient` | 构造客户端 |
| `Connect(ctx) error` | 建立连接并阻塞至首次认证成功 |
| `Disconnect()` | 主动断开 |
| `IsConnected() bool` | 当前连接状态 |

### 回复消息

| 方法 | 说明 |
|---|---|
| `Reply(frame.Headers, body, cmd)` | 通用回复（cmd 空兜底 `Response`） |
| `ReplyStream(frame.Headers, streamId, content, finish, msgItem, feedback)` | 流式回复（msg_item 仅 finish 时附带） |
| `ReplyStreamNonBlocking(...)` | 非阻塞流式回复（上一条未 ack 且非最终帧返回 `ErrReplySkipped`） |
| `ReplyWelcome(frame.Headers, body)` | 欢迎语（cmd=`ResponseWelcome`，body 为 `WelcomeTextReplyBody`/`WelcomeTemplateCardReplyBody`） |
| `ReplyTemplateCard(frame.Headers, card, feedback)` | 模板卡片回复（feedback 非空合并进卡片） |
| `ReplyStreamWithCard(frame.Headers, streamId, content, finish, opts)` | 流式 + 模板卡片组合回复 |
| `UpdateTemplateCard(frame.Headers, card, userIds)` | 更新模板卡片（cmd=`ResponseUpdate`） |
| `HasPendingReplyAck(frame.Headers) bool` | 是否有待完成 ack |

### 主动发送与媒体

| 方法 | 说明 |
|---|---|
| `SendMessage(chatid, body)` | 主动发送（生成新 reqId，chatid 合并进 body，cmd=`SendMsg`） |
| `SendMediaMessage(chatid, mediaType, mediaId, videoOpts)` | 主动发送媒体 |
| `ReplyMedia(frame.Headers, mediaType, mediaId, videoOpts)` | 被动回复媒体（不合并 chatid） |
| `UploadMedia(fileBuffer, opts) (*UploadMediaFinishResult, error)` | 分片上传临时素材（512KB/片，动态并发，单片重试 2 次） |
| `DownloadFile(url, aesKey) ([]byte, string, error)` | 下载并 AES 解密文件附件 |
| `Api() *WeComApiClient` | 内部 HTTP 客户端（高级用途） |

### 消息/事件回调

| 回调字段 | 触发 |
|---|---|
| `OnMessage` | 收到任意消息（body 为 `BaseMessage`） |
| `OnText` / `OnImage` / `OnMixed` / `OnVoice` / `OnFile` / `OnVideo` | 对应 `MessageType` 的类型化消息 |
| `OnEvent` | 收到任意事件 |
| `OnEnterChat` / `OnTemplateCardEvent` / `OnFeedbackEvent` / `OnDisconnectedEvent` | 对应 `EventType` 的类型化事件 |
| `OnConnected` / `OnAuthenticated` / `OnDisconnected` / `OnReconnecting` / `OnError` | 连接生命周期 |

## 消息类型（`MessageType`）

| 常量 | 值 | 回调 |
|---|---|---|
| `MessageType.Text` | `text` | `OnText` |
| `MessageType.Image` | `image` | `OnImage` |
| `MessageType.Mixed` | `mixed` | `OnMixed` |
| `MessageType.Voice` | `voice` | `OnVoice` |
| `MessageType.File` | `file` | `OnFile` |
| `MessageType.Video` | `video` | `OnVideo` |

## 事件类型（`EventType`）

| 常量 | 值 | 回调 |
|---|---|---|
| `EventType.EnterChat` | `enter_chat` | `OnEnterChat` |
| `EventType.TemplateCardEvent` | `template_card_event` | `OnTemplateCardEvent` |
| `EventType.FeedbackEvent` | `feedback_event` | `OnFeedbackEvent` |
| `EventType.Disconnected` | `disconnected_event` | `OnDisconnectedEvent`（被踢下线，不重连） |

## 协议常量（`WsCmd`）

| 常量 | 值 | 方向 |
|---|---|---|
| `WsCmd.Subscribe` | `aibot_subscribe` | 认证订阅 |
| `WsCmd.Heartbeat` | `ping` | 心跳 |
| `WsCmd.Response` | `aibot_respond_msg` | 被动回复 |
| `WsCmd.ResponseWelcome` | `aibot_respond_welcome_msg` | 欢迎语回复 |
| `WsCmd.ResponseUpdate` | `aibot_respond_update_msg` | 更新模板卡片 |
| `WsCmd.SendMsg` | `aibot_send_msg` | 主动发送 |
| `WsCmd.UploadMediaInit` / `UploadMediaChunk` / `UploadMediaFinish` | `aibot_upload_media_*` | 分片上传三步 |
| `WsCmd.Callback` | `aibot_msg_callback` | 消息推送（服务端→开发者） |
| `WsCmd.EventCallback` | `aibot_event_callback` | 事件推送（服务端→开发者） |

## 加解密

### 文件附件（消息自带 aeskey）

```go
// aesKey 取自 body.Image.AesKey / body.File.AesKey 等
data, filename, err := client.DownloadFile(imageUrl, body.Image.AesKey)
// 或仅解密已有密文：
plain, err := aibot.DecryptFile(encrypted, aesKey)
```

AES-256-CBC、IV = 密钥前 16 字节、**PKCS#7 块大小 = 32**（企微特殊约定）。

### Webhook 回调（EncodingAesKey）

```go
wc, err := aibot.NewWecomCrypto(token, encodingAesKey, receiveId)
// 验签
if !wc.VerifySignature(signature, timestamp, nonce, encrypt) { ... }
// 解密
plain, err := wc.Decrypt(encrypt)
// 加密（返回 base64 密文 + 签名）
encrypt, signature, err := wc.Encrypt(plainText, timestamp, nonce)
```

签名 = `SHA1(sort([token,timestamp,nonce,encrypt]) join)`；明文结构 `[16随机][4大端len][msg][receiveId]`。`DecodeEncodingAesKey` 将 43 位 key 补 `=` 后 base64 解码为 32 字节。

## 构建/测试

```bash
go build ./...        # 构建
go vet ./...          # 静态检查
go run ./examples/basic
```

## 项目结构

```
aibot/                       # 包 aibot
├── index.go                 # 跨包 re-export（Go 1.24 泛型别名）
├── client.go                # WsClient（连接/回复/发送/上传/下载）
├── ws.go                    # WsConnectionManager（拨号/认证/心跳/重连/回复队列）
├── message-handler.go       # MessageHandler（消息/事件分发）
├── api.go                   # WeComApiClient（HTTP 下载 + Content-Disposition）
├── crypto.go                # DecryptFile（AES-256-CBC）
├── wecom-crypto.go          # WecomCrypto（Webhook 加解密）
├── logger.go                # DefaultLogger
├── utils.go                 # GenerateReqId / GenerateRandomString
└── types/                   # 包 types（WsCmd/WsFrame/消息/事件/卡片/回复体/上传体）
```

## License

MIT

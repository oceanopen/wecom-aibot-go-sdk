# wecom-aibot-go-sdk 重构任务计划（Node 1:1 镜像版）

> 目标：与企业微信智能机器人 Node SDK（`https://github.com/WecomTeam/aibot-node-sdk`）实现**代码层面 1:1 还原**——
> 文件结构、文件命名、文件拆分、变量与类型命名全部与 Node 参考项目保持一致；逻辑亦 1:1。
> 文档：https://developer.work.weixin.qq.com/document/path/101463
> 执行方式：**先搭全量文件骨架，再逐个文件填充内容**，每个任务独立可验证。

---

## 一、全局约定

### 1.1 module / 安装 / import

- module：`github.com/oceanopen/wecom-aibot-go-sdk`
- Go 版本：**`go 1.24`**（重新导出泛型 `WsFrame[T]` 需要 1.24 泛型类型别名；本地为 go1.24.1）

```bash
go get github.com/oceanopen/wecom-aibot-go-sdk
```

```go
import (
	aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
)
// 使用：aibot.WsClient / aibot.NewWsClient / aibot.WsFrame[*aibot.TextMessage] ...
```

### 1.2 目录与文件结构（1:1 镜像 Node `src/`，无 `internal/`）

```
wecom-aibot-go-sdk/
├── go.mod
├── task.md
├── README.md
├── CLAUDE.md
├── aibot/                       # 包 aibot（镜像 src/）
│   ├── index.go                 # ← src/index.ts        重新导出全部公开符号
│   ├── client.go                # ← src/client.ts        WsClient（含 UploadMedia）
│   ├── ws.go                    # ← src/ws.ts            WsConnectionManager
│   ├── message-handler.go       # ← src/message-handler.ts  MessageHandler
│   ├── api.go                   # ← src/api.ts           WeComApiClient（HTTP 下载）
│   ├── crypto.go                # ← src/crypto.ts        DecryptFile
│   ├── wecom-crypto.go          # ← src/wecom-crypto/index.ts  WecomCrypto
│   ├── logger.go                # ← src/logger.ts        DefaultLogger
│   ├── utils.go                 # ← src/utils.ts         GenerateReqId / GenerateRandomString
│   └── types/                   # 包 types（镜像 src/types/）
│       ├── index.go             # ← types/index.ts       类型 re-export
│       ├── config.go            # ← types/config.ts      WsClientOptions
│       ├── common.go            # ← types/common.ts      Logger 接口 + 错误类型
│       ├── message.go           # ← types/message.ts     消息类型
│       ├── event.go             # ← types/event.ts       事件类型
│       └── api.go               # ← types/api.ts         WsCmd / WsFrame / 卡片 / 回复体
└── examples/
    └── basic/
        └── main.go              # ← examples/basic.ts
```

- **不新增 Node 之外的文件**（仅 `*_test.go` 例外）。文件名用连字符贴合 Node（`message-handler.go`、`wecom-crypto.go`）。
- `types/` 是 Go 子包 `types`；`aibot/index.go` 镜像 Node `index.ts` 重新导出 `types` 的公开符号，使用户只 import `aibot` 即可用 `aibot.WsFrame` 等。

### 1.3 命名约定

- **缩写词仅首字母大写**（硬要求）：`Id`（非 `ID`）、`Ws`（非 `WS`）、`Aes`、`App`、`Url`、`Http`、`Tls`、`Md5`。
  - 例：`ReqId`、`BotId`、`MsgId`、`AibotId`、`AesKey`、`AppId`、`MediaId`、`UploadId`、`TaskId`、`ChatId`、`UserId`、`IconUrl`、`ImageUrl`、`WsUrl`。
- **类型/变量/函数名 1:1 镜像 Node**（仅大小写按 Go 导出规则 + 上述缩写规则调整）：
  - `WSClient`→`WsClient`、`WsConnectionManager`、`MessageHandler`、`WeComApiClient`、`DefaultLogger`、`WecomCrypto`
  - `WsFrame<T>`→`WsFrame[T]`、`WsFrameHeaders`、`WsCmd`、`WsClientOptions`
  - `MessageType`/`EventType`/`TemplateCardType`/`WeComMediaType` 同名
  - `BaseMessage`/`TextMessage`/…/`EventMessage`/`TemplateCard` 及所有子结构同名
  - `DecryptFile`、`GenerateReqId`、`GenerateRandomString` 同名
  - 错误：`WSAuthFailureError`→`WsAuthFailureError`、`WSReconnectExhaustedError`→`WsReconnectExhaustedError`（镜像 Node 的类）
- **JSON tag** 与服务端一致（snake_case：`req_id`/`bot_id`/`msgtype`/`aibotid`/`aeskey`/`msg_id` …）。

### 1.4 注释约定

- **结构体字段注释写行尾**（gofmt 自动对齐），不在字段上方：

```go
type WsFrame[T any] struct {
	Cmd     string         `json:"cmd,omitempty"`    // 命令类型；认证/心跳响应时可能为空
	Headers WsFrameHeaders `json:"headers"`          // 请求头（含 req_id）
	Body    T              `json:"body,omitempty"`   // 消息体
	ErrCode int            `json:"errcode,omitempty"` // 响应错误码，0 表示成功
	ErrMsg  string         `json:"errmsg,omitempty"`  // 响应错误信息
}
```

- 包/类型/函数文档注释写声明上方（Go doc 惯例）。

### 1.5 API 形态约定（镜像 Node，非 Go 惯例）

- **配置**：用 `WsClientOptions` 结构体（镜像 Node `WSClientOptions`），构造 `NewWsClient(opts WsClientOptions) *WsClient`。**不用**函数式 `With*` 选项。
- **回复方法签名**：镜像 Node `reply(frame: WsFrameHeaders, ...)`——回复类方法首参为 `WsFrameHeaders`，调用方传 `frame.Headers`（如 `client.ReplyStream(frame.Headers, streamId, ...)`）。`WsFrameHeaders` 即 headers 类型（`ReqId string`），`WsFrame.Headers` 字段类型为 `WsFrameHeaders`。
- **`Connect(ctx)`** 仍阻塞至首次认证成功（Go 友好，Node 是立即返回+事件；此点保留 Go 化）。

### 1.6 验证方法与 DoD

| 验证对象 | 方法 |
|---|---|
| 纯逻辑（types/crypto/utils/帧编解码） | `go test ./...` 单测（往返、边界、fuzz） |
| 连接/认证/心跳/重连/回复队列/分发 | `go test` 中的**本地 mock WebSocket 服务端**集成测试（无需真实凭证） |
| 端到端 | `examples/basic`（需真实 botId/secret，人工验证） |

- **每个任务 DoD**：`gofmt -l` 无差异 + `go build ./...` 通过 + 该任务的测试通过。
- 每个任务独立提交（`feat(aibot): 任务N …`）。验证不过不进入下一个。

### 1.7 加密要点（移植易错）

AES-256-CBC、IV = 密钥前 16 字节、**PKCS#7 块大小 = 32**（企微特殊约定）。文件附件用消息自带 `aeskey`；Webhook 用 `EncodingAesKey`（43 位，补 `=` 后 base64 解码 32 字节）；签名 `SHA1(sort([token,timestamp,nonce,encrypt]) join)`；明文结构 `[16随机][4大端len][msg][receiveId]`。

---

## 二、任务清单

> 🏁 = 关键里程碑。每个任务自带验证标准。先搭骨架（任务 1），再逐文件填充。

### A. 文件骨架

#### 任务 1：全量文件骨架 🏁（先定义基础文件结构） ✅ 已完成
- 文件（新增）：`go.mod`（`go 1.24`）、`.gitignore`、`CLAUDE.md`；以及下列**空包占位文件**（仅 `package` 声明 + 文件头注释标注镜像的 Node 源）：
  - `aibot/index.go`、`aibot/client.go`、`aibot/ws.go`、`aibot/message-handler.go`、`aibot/api.go`、`aibot/crypto.go`、`aibot/wecom-crypto.go`、`aibot/logger.go`、`aibot/utils.go`（均 `package aibot`）
  - `aibot/types/index.go`、`aibot/types/config.go`、`aibot/types/common.go`、`aibot/types/message.go`、`aibot/types/event.go`、`aibot/types/api.go`（均 `package types`）
- 目标：建立与 Node 1:1 的目录/文件骨架
- 验证：`go build ./...` 通过；`go vet ./...` 通过

### B. 基础设施（types/common、utils、logger）

#### 任务 2：types/common.go —— Logger 接口 + 错误类型 ✅ 已完成
- 文件（修改）：`aibot/types/common.go` —— `Logger` 接口（Debug/Info/Warn/Error）；`WsAuthFailureError`/`WsReconnectExhaustedError` 错误类型（带 `Code()`，镜像 Node 的类与 code 常量）
- 验证：单测 `errors.As` + `Code()` 返回稳定码

#### 任务 3：utils.go —— GenerateReqId / GenerateRandomString ✅ 已完成
- 文件（修改）：`aibot/utils.go` —— `GenerateReqId(prefix)` → `{prefix}_{unixMilli}_{randHex8}`；`GenerateRandomString(n)`
- 验证：单测格式（前缀/数字时间戳/8 位 hex）、唯一性、默认长度

#### 任务 4：logger.go —— DefaultLogger ✅ 已完成
- 文件（修改）：`aibot/logger.go` —— `DefaultLogger` 实现 `types.Logger`（可注入 `nowFunc` 便于测试）
- 验证：单测输出含时间戳/级别；`nowFunc` 注入返回固定时间

### C. 协议与类型（types/ 子包）

#### 任务 5：types/config.go —— WsClientOptions ✅ 已完成
- 文件（修改）：`aibot/types/config.go` —— `WsClientOptions` 结构（BotId/Secret/Scene/PlugVersion/ReconnectInterval/MaxReconnectAttempts/MaxAuthFailureAttempts/HeartbeatInterval/RequestTimeout/WsUrl/WsOptions/MaxReplyQueueSize/Logger），字段注释行尾
- 验证：单测字段齐全；默认值由 client 兜底（本任务只定义结构）

#### 任务 6：types/api.go（上）—— WsCmd / WsFrame / WsFrameHeaders ✅ 已完成
- 文件（修改）：`aibot/types/api.go` —— `WsCmd` 常量（Subscribe/Heartbeat/Response/ResponseWelcome/ResponseUpdate/SendMsg/UploadMediaInit/UploadMediaChunk/UploadMediaFinish/Callback/EventCallback）；`WsFrame[T]`（Cmd/Headers/Body/ErrCode/ErrMsg）；`WsFrameHeaders{ReqId}`
- 验证：单测 `WsFrame` JSON 往返；`WsFrameHeaders.ReqId` 正确

#### 任务 7：types/message.go —— 消息类型 ✅ 已完成
- 文件（修改）：`aibot/types/message.go` —— `MessageType` 常量；`MessageFrom`、`TextContent`/`ImageContent`/`VoiceContent`/`FileContent`/`VideoContent`/`MixedContent`/`QuoteContent`；`BaseMessage` + `TextMessage`/`ImageMessage`/`MixedMessage`/`VoiceMessage`/`FileMessage`/`VideoMessage`
- 验证：单测样例 JSON 反序列化为对应类型

#### 任务 8：types/event.go —— 事件类型 ✅ 已完成
- 文件（修改）：`aibot/types/event.go` —— `EventType` 常量；`EventFrom`；`EnterChatEvent`/`TemplateCardEventData`/`FeedbackEventData`/`DisconnectedEventData`；`EventMessage`（含 `DecodeEvent()`）
- 验证：单测 `EventMessage.DecodeEvent()` 按 eventtype 返回正确类型

#### 任务 9：types/index.go —— 包内 re-export ✅ 已完成
- 文件（修改）：`aibot/types/index.go` —— 镜像 Node `types/index.ts`，re-export 本子包各文件公开符号（便于 `types.WsFrame` 等统一入口）
- 验证：`go build ./...` 通过；单测从 `types` 引用各符号

#### 任务 10：aibot/index.go —— 跨包 re-export（Go 1.24 泛型别名）🏁 ✅ 已完成
- 文件（修改）：`aibot/index.go` —— 镜像 Node `src/index.ts`：把 `aibot/types` 的公开类型 re-export 到 `aibot`，含 `type WsFrame[T any] = types.WsFrame[T]` 等泛型别名，使 `aibot.WsFrame[*aibot.TextMessage]` 可用
- 验证：`go build ./...` 通过（Go 1.24）；单测 `aibot.WsFrame` 别名可用

### D. 连接核心（ws.go，逐步拆分 + mock 验证）

#### 任务 11：ws.go（上）—— 拨号 + 认证 ✅ 已完成
- 文件（修改）：`aibot/ws.go` —— `WsConnectionManager`（持有 logger/options/回调字段）+ `Connect(ctx)`/`connectOnce`（拨号 wss + onConnected + sendAuth）/`readLoop`/`handleFrame` 路由/`handleAuthResponse`（errcode=0 → onAuthenticated，失败标记并重连）
- 文件（新增）：`aibot/ws_test.go` —— 本地 mock WebSocket 服务端（gorilla upgrader）
- 验证：集成测试——连接 mock 服务端、发 `aibot_subscribe`、服务端回 `errcode=0`、`onAuthenticated` 触发

#### 任务 12：ws.go（中）—— 心跳 ✅ 已完成
- 文件（修改）：`aibot/ws.go` —— `startHeartbeat`/`heartbeatLoop`/`sendHeartbeat`/`missedPongCount`（连续 `maxMissedPong=2` 次无 ack 强制断连）；认证成功后启动
- 验证：mock 测试——不回 pong 连续 2 次后断连；回 pong 则归零

#### 任务 13：ws.go（下）—— 重连与生命周期 ✅ 已完成
- 文件（修改）：`aibot/ws.go` —— `scheduleReconnect`（指数退避 1s→30s，两套计数器：认证失败/连接断开，`-1` 无限）、`handleClose`（`closed` 去重）、`disconnect`（原子置 `closed` 触发 `onDisconnected`，避免双触发）、ctx 取消调 `disconnect` 防泄漏
- 验证：mock 测试——断线按退避重连并再次认证；认证耗尽返回 `WsAuthFailureError`；ctx 取消无 goroutine 泄漏

#### 任务 14：ws.go（末）—— 串行回复队列 + ack ✅ 已完成
- 文件（修改）：`aibot/ws.go` —— `SendReply(frame WsFrameHeaders, body, cmd)`（同一 reqId 串行队列）、`processReplyQueue`（即任务说明中的 replyProcessor：发一条→等 ack/5s 超时→下一条）、`pendingAcks`+seq 防竞态、`HasPendingAck`、`clearPendingMessages`（断开时清理待处理回复）
- 验证：mock 测试——回复收到 ack 解析成功；不回则超时报错；同 reqId 串行有序（含默认 cmd、HasPendingAck、断开清理）

### E. 客户端门面（client.go / message-handler.go）

#### 任务 15：client.go（上）—— WsClient + 连接 + 回调 ✅ 已完成
- 文件（修改）：`aibot/client.go` —— `WsClient` 结构（持有 `WsConnectionManager`/`MessageHandler`/回调字段 `OnMessage`/`OnText`/…/`OnConnected`/`OnAuthenticated`/`OnDisconnected`/`OnReconnecting`/`OnError`）+ `NewWsClient(opts WsClientOptions)`（兜底默认值，-1 无限保留）+ `Connect(ctx)`/`Disconnect()`/`IsConnected()` + 回调桥接（含 `OnServerDisconnect` 置 started=false）；`aibot/message-handler.go` —— `MessageHandler` 结构 + 构造 + 最小 `HandleFrame`（OnMessage 透传，任务 16 扩展）；`aibot/ws_test.go` —— mock 服务端加 `writePush`/`writeMu`（串行化写入）
- 文件（新增）：`aibot/client_test.go`
- 验证：mock 测试——连接+认证成功触发 `OnAuthenticated`（含 OnConnected、IsConnected、Disconnect、默认值兜底、-1 保留、scene/plug_version 透传、OnMessage 透传）

#### 任务 16：message-handler.go —— MessageHandler 分发 ✅ 已完成
- 文件（修改）：`aibot/message-handler.go` —— `HandleFrame`（探针解析 cmd+msgtype 路由）/`handleMessageCallback`（OnMessage + 文本 OnText）/`handleEventCallback`（OnEvent）；事件帧不触发 OnMessage；其余 msgtype 与类型化事件分发留待任务 19
- 文件（新增）：`aibot/message-handler_test.go`
- 验证：测试——文本帧触发 OnMessage+OnText 且 body 正确；事件帧触发 OnEvent 且 DecodeEvent 正确；事件不触发 OnMessage；未处理类型仅 OnMessage；缺 msgtype 无回调；nil 回调不 panic

#### 任务 17：client.go（中）—— Reply / ReplyStream / ReplyStreamNonBlocking ✅ 已完成
- 文件（修改）：`aibot/client.go` —— `Reply(frame WsFrameHeaders, body any, cmd string)`（委托 `wsManager.SendReply`，cmd 空兜底 Response）/`ReplyStream`（构建 `{msgtype:"stream", stream:{id,finish,content,msg_item?,feedback?}}`，msg_item 仅 finish 时附带）/`ReplyStreamNonBlocking`（`!finish && HasPendingReplyAck` 返回 `ErrReplySkipped`，最终帧不跳过）/`HasPendingReplyAck`；流式 body 暂用 `map[string]any`（类型化 `StreamReplyBody`/`ReplyMsgItem`/`ReplyFeedback` 留待任务 24，届时可重构）；测试追加到 `aibot/client_test.go`
- 验证：mock 测试——流式中间帧+最终帧 body 结构正确且收到回执；非阻塞 pending 时中间帧跳过（ErrReplySkipped）最终帧不跳过；通用 Reply 自定义 body；cmd 空兜底 Response

#### 任务 18：最小闭环示例与文档 🏁（首个「最小可验证功能」里程碑）
- 文件（新增）：`examples/basic/main.go` —— 连接+认证+收文本+流式回复+优雅退出（SIGINT）
- 文件（修改）：`README.md` —— 安装+import+快速开始（仅覆盖最小闭环）
- 验证：`go build ./examples/basic` 通过；README 与 API 一致；填凭证可连真实服务器

---

> ✅ 至此完成最小闭环：连接→认证→收文本→流式回复。以下逐步扩充，每任务独立可验证。

---

### F. 逐步扩充

#### 任务 19：message-handler.go —— 其余消息/事件回调
- 文件（修改）：`aibot/message-handler.go`（`OnImage`/`OnMixed`/`OnVoice`/`OnFile`/`OnVideo`/`OnEnterChat`/`OnTemplateCardEvent`/`OnFeedbackEvent`/`OnDisconnectedEvent` 分发）；`aibot/client.go`（补回调字段）；`aibot/ws.go`（`disconnected_event` 处理：置 `isManualClose` 阻止重连）
- 验证：mock 测试——各类型分别触发对应回调；`disconnected_event` 不重连

#### 任务 20：crypto.go —— 文件附件解密 DecryptFile
- 文件（修改）：`aibot/crypto.go` —— `DecryptFile(encrypted, aesKey)`（AES-256-CBC、IV=key[:16]、PKCS#7 块 32）；`PKCS7Pad`/`PKCS7Unpad`/常量
- 文件（新增）：`aibot/crypto_test.go`
- 验证：单测往返（变长）+ fuzz + 错误 key 失败 + 空输入

#### 任务 21：api.go —— WeComApiClient HTTP 下载
- 文件（修改）：`aibot/api.go` —— `WeComApiClient.DownloadFileRaw(url)`（解析 `Content-Disposition`，优先 RFC5987 `filename*=UTF-8''`）
- 验证：单测 `Content-Disposition` 多格式解析

#### 任务 22：client.go —— DownloadFile
- 文件（修改）：`aibot/client.go` —— `DownloadFile(url, aesKey)`（下载 + 调 `DecryptFile`）
- 验证：mock http + 加密往返解密

#### 任务 23：wecom-crypto.go —— WecomCrypto（Webhook 加解密）
- 文件（修改）：`aibot/wecom-crypto.go` —— `DecodeEncodingAesKey`、`WecomCrypto`（`ComputeSignature`/`VerifySignature`/`Encrypt`/`Decrypt`）
- 文件（新增）：`aibot/wecom-crypto_test.go`
- 验证：单测往返 + 签名格式 + receiveId 校验 + fuzz

#### 任务 24：types/api.go（下）—— 模板卡片 + 回复体类型
- 文件（修改）：`aibot/types/api.go` —— 追加 `TemplateCardType` + `TemplateCard` 及全部子结构 + 回复体（`TemplateCardReplyBody`/`StreamReplyBody`/`WelcomeTextReplyBody`/`WelcomeTemplateCardReplyBody`/`StreamWithTemplateCardReplyBody`/`UpdateTemplateCardBody`）+ `ReplyMsgItem`/`ReplyFeedback`
- 验证：单测 JSON `omitempty`（可选省略、必填保留如 `TemplateCardAction.Type`）

#### 任务 25：client.go —— 卡片与欢迎语回复
- 文件（修改）：`aibot/client.go` —— `ReplyWelcome`/`ReplyTemplateCard`/`ReplyStreamWithCard`/`UpdateTemplateCard`
- 验证：mock 测试——回复体 JSON 字段；feedback 合并；update 用 `ResponseUpdate` cmd

#### 任务 26：client.go —— 主动发送与媒体回复
- 文件（修改）：`aibot/client.go` —— `SendMessage`/`SendMediaMessage`/`ReplyMedia`；主动发送 body 类型（`SendMarkdownMsgBody`/`SendTemplateCardMsgBody`/`SendMediaMsgBody`）追加到 `types/api.go`
- 验证：mock 测试——chatid 合并；媒体 body 字段；video title/description

#### 任务 27：client.go —— UploadMedia 分片上传
- 文件（修改）：`aibot/client.go` —— `UploadMedia`（init→chunk×N→finish；512KB/片、chunk_index 从 0、动态并发 ≤4/3/2、单片重试 2 次）；上传 body 类型追加到 `types/api.go`
- 验证：mock 测试——上传往返返回 media_id；并发与重试路径；超大文件拒绝

#### 任务 28：index.go / types/index.go 完整 re-export 校对 + 完整文档 🏁
- 文件（修改）：`aibot/index.go`、`aibot/types/index.go` —— 校对所有公开符号已 re-export（`aibot.X` 可用）
- 文件（修改）：`README.md` —— 特性/安装/import/快速开始/API 表/配置/事件/消息类型/协议常量/加解密/完整示例
- 文件（修改）：`CLAUDE.md` —— 架构总览 + 构建/测试命令 + 命名/注释/镜像约定
- 验证：文档与代码 API 一致；`go build ./...` + `go test ./...` 全通过

---

## 三、执行规则

1. **严格按编号顺序**执行（types 先于 aibot；ws 先于 client；client 先于 message-handler 依赖处）。
2. 任务 1 先搭全量骨架；之后**逐个文件填充**。
3. **每个任务独立提交**；**每个任务必须达成 DoD**（gofmt 无差异 + `go build ./...` + 该任务测试通过）才进入下一个。
4. 单个任务 = 一个可验证小切片，绝不一次性堆砌。
5. 验证不过不进入下一个，先修复。
6. 文件/类型/变量命名、文件拆分一律以 Node 参考项目为准（1:1），不擅自新增文件（仅 `_test.go` 例外）。

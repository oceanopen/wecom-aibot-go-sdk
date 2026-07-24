# CLAUDE.md — wecom-aibot-go-sdk

企业微信智能机器人 Go SDK，**代码层面 1:1 镜像** Node SDK（`https://github.com/WecomTeam/aibot-node-sdk`）：
文件结构、文件命名、文件拆分、变量与类型命名均与 Node 参考项目保持一致。

>  Node SDK（`https://github.com/WecomTeam/aibot-node-sdk`） 这个项目已 download 到本地，详见：`~/MyFiles/Project/aibot-node-sdk`，所以不用从 `github` 上实时拉取。
>  每次执行完一个任务，更新 `task.md` 任务状态，执行后不要自动提交和继续，由我手动确认无误后自己提交。我主动告诉执行下个任务你再继续执行下个任务。

## module / 安装

- module：`github.com/oceanopen/wecom-aibot-go-sdk`
- Go 版本：**1.24**（`aibot/index.go` 重新导出泛型 `WsFrame[T]` 需要 1.24 泛型类型别名）
- import：`aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"`

## 结构（1:1 镜像 Node `src/`，无 `internal/`）

- `aibot/`：`index.go` / `client.go` / `ws.go` / `message-handler.go` / `api.go` / `crypto.go` / `wecom-crypto.go` / `logger.go` / `utils.go`
- `aibot/types/`（子包 `types`）：`index.go` / `config.go` / `common.go` / `message.go` / `event.go` / `api.go`
- `aibot/index.go` 镜像 Node `src/index.ts`，把 `aibot/types` 的公开符号重新导出到 `aibot`，使用户仅 import `aibot`。

## 架构总览

`WsClient`（`client.go`）是对外门面，聚合三个协作组件：

- **`WsConnectionManager`**（`ws.go`）：WebSocket 连接核心。拨号 → 认证 → 心跳 → 指数退避重连（认证失败/连接断开两套计数器，`-1` 无限）；维护按 `req_id` 分组的回复队列（同一 reqId 串行，不同 reqId 可并发），发一条 → 等 ack/超时 → 下一条。
- **`MessageHandler`**（`message-handler.go`）：消息/事件分发。探针解析 `cmd`+`msgtype`/`eventtype`，路由到 `WsClient` 的类型化回调（`OnText`/`OnImage`/…/`OnEnterChat`/…）。
- **`WeComApiClient`**（`api.go`）：仅负责 HTTP 文件下载（解析 `Content-Disposition`），消息收发均走 WebSocket。

数据流：
- **收**：服务端帧 → `ws.readLoop` → `handleFrame` 路由（认证/心跳响应、`disconnected_event`、消息/事件回调）→ `onMessage` → `MessageHandler.HandleFrame` → 类型化回调。
- **回复**：`client.Reply*` → `wsManager.SendReply(frame, body, cmd)`（首参 `WsFrameHeaders`，透传 `req_id`）→ 串行队列 + 等 ack。
- **主动发送**：`SendMessage` 生成新 `req_id`，把 `chatid` 合并进 body，走 `SendMsg` 通道。
- **上传**：`UploadMedia` 三步（init→chunk×N→finish），多分片动态并发（≤4/3/2 worker pool，不同 reqId 并发），单片失败重试 2 次。
- **加解密**：`DecryptFile`（文件附件，消息自带 `aeskey`，AES-256-CBC + PKCS#7 块 32）/ `WecomCrypto`（Webhook，`EncodingAesKey` + SHA1 签名）。

## 代码风格

- **字符串拼接一律用 `fmt.Sprintf`**，禁止 `"a" + x + "b"` 风格的 `+` 拼接。
  例：`fmt.Sprintf("Connecting to %s...", url)` 而非 `"Connecting to " + url + "..."`。

## 命名约定

- 缩写词仅首字母大写：`Id`（非 `ID`）、`Ws`（非 `WS`）、`Aes`、`App`、`Url`、`Http`、`Md5`。
  例：`ReqId`、`BotId`、`AesKey`、`AppId`、`IconUrl`。
- 类型/变量/函数/文件名 1:1 镜像 Node（`WsClient`、`WsFrame`、`WsCmd`、`MessageHandler`、`wecom-crypto.go` …）。
- 结构体字段注释写**行尾**（gofmt 自动对齐），不在字段上方。
- 不新增 Node 之外的文件（仅 `*_test.go` 例外）。
- 配置用 `WsClientOptions` 结构体（`NewWsClient(opts)`），不用函数式 `With*`。
- 回复方法首参为 `WsFrameHeaders`（镜像 Node `reply(frame: WsFrameHeaders)`），调用方传 `frame.Headers`。

## 常用命令

```bash
go build ./...            # 构建
go test ./...             # 测试
go run ./examples/basic   # 运行示例
```

任务计划与执行顺序见根目录 `task.md`。

## 环境提示

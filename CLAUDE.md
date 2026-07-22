# CLAUDE.md — wecom-aibot-go-sdk

企业微信智能机器人 Go SDK，**代码层面 1:1 镜像** Node SDK（`https://github.com/WecomTeam/aibot-node-sdk`）：
文件结构、文件命名、文件拆分、变量与类型命名均与 Node 参考项目保持一致。

## module / 安装

- module：`github.com/oceanopen/wecom-aibot-go-sdk`
- Go 版本：**1.24**（`aibot/index.go` 重新导出泛型 `WsFrame[T]` 需要 1.24 泛型类型别名）
- import：`aibot "github.com/oceanopen/wecom-aibot-go-sdk/aibot"`

## 结构（1:1 镜像 Node `src/`，无 `internal/`）

- `aibot/`：`index.go` / `client.go` / `ws.go` / `message-handler.go` / `api.go` / `crypto.go` / `wecom-crypto.go` / `logger.go` / `utils.go`
- `aibot/types/`（子包 `types`）：`index.go` / `config.go` / `common.go` / `message.go` / `event.go` / `api.go`
- `aibot/index.go` 镜像 Node `src/index.ts`，把 `aibot/types` 的公开符号重新导出到 `aibot`，使用户仅 import `aibot`。

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

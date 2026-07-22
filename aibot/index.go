package aibot

// Package aibot 是企业微信智能机器人 Go SDK，对应 Node SDK @wecom/aibot-node-sdk。
//
// 文件结构、命名、拆分 1:1 镜像 Node 的 src/。入口文件 index.go（对应 Node src/index.ts）
// 重新导出 aibot/types 子包的全部公开符号，使用户仅需
// import "github.com/oceanopen/wecom-aibot-go-sdk/aibot"
// 即可使用 aibot.WsClient、aibot.WsFrame[*aibot.TextMessage] 等。
//
// 详细任务计划与执行顺序见项目根目录 task.md。

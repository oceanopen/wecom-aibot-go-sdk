package aibot

// version.go 维护 SDK 版本号（Go 侧自有文件，Node 无对应源码文件，作用等同 Node package.json
// 的 version 字段）。
//
// Version 与 git tag（vX.Y.Z）保持同步，由 scripts/release.sh 在发版时自动更新，请勿手工修改。

// Version SDK 当前版本号（语义化版本，不带 v 前缀）。
const Version = "0.2.0"

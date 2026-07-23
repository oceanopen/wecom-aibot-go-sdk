package aibot

// utils.go 对应 Node src/utils.ts：GenerateReqId / GenerateRandomString。

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateRandomString 生成随机字符串，对应 Node generateRandomString。
//
// 默认长度 8：生成 ceil(n/2) 个随机字节，转小写 hex 后截取前 n 个字符。
// length 为期望字符数；n<=0 时返回空串。
func GenerateRandomString(length int) string {
	if length <= 0 {
		return ""
	}
	buf := make([]byte, (length+1)/2)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read 在正常系统环境下不会失败；若失败，返回固定占位以保持长度。
		return fmt.Sprintf("%0*x", length, 0)
	}
	return hex.EncodeToString(buf)[:length]
}

// GenerateReqId 生成唯一请求 ID，对应 Node generateReqId。
//
// 格式：{prefix}_{unixMilli}_{randomHex8}。prefix 通常为 cmd 名称。
func GenerateReqId(prefix string) string {
	timestamp := time.Now().UnixMilli()
	random := GenerateRandomString(8)
	return fmt.Sprintf("%s_%d_%s", prefix, timestamp, random)
}

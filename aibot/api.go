package aibot

// api.go 对应 Node src/api.ts：WeComApiClient（HTTP 文件下载 + Content-Disposition 解析）。
//
// 仅负责文件下载等 HTTP 辅助功能；消息收发均走 WebSocket 通道。

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// defaultApiTimeout 默认请求超时（毫秒），对应 Node WeComApiClient 构造默认参数 10000。
const defaultApiTimeout = 10000

// reFilenameExt 匹配 RFC 5987 扩展文件名（filename*，charset=UTF-8，空 language 段），优先解析。
//
// 镜像 Node 正则 /filename\*=UTF-8<quote><quote>([^;\s]+)/i：大小写不敏感，
// 捕获 charset 与 language 两段单引号之后的非 ';' 与空白字符序列（见下文字面量）。
var reFilenameExt = regexp.MustCompile(`(?i)filename\*=UTF-8''([^;\s]+)`)

// reFilename 匹配 filename="xxx" 或 filename=xxx（回退解析）。
//
// 镜像 Node 正则 /filename="?([^";\s]+)"?/i：可选引号，捕获非 '"'、';' 与空白字符序列。
var reFilename = regexp.MustCompile(`(?i)filename="?([^";\s]+)"?`)

// WeComApiClient 企业微信 API 客户端，对应 Node WeComApiClient。
//
// 持有复用的 HTTP 客户端（携带超时配置）与日志实现，仅用于文件下载等 HTTP 辅助功能。
type WeComApiClient struct {
	httpCli *http.Client // 复用的 HTTP 客户端（携带 timeout 配置），对应 Node httpClient
	logger  types.Logger // 日志实现
}

// NewWeComApiClient 构造 WeComApiClient，对应 Node new WeComApiClient(logger, timeout)。
//
// timeout 为请求超时（毫秒），<=0 时取默认值 10000（镜像 Node 默认参数）。
func NewWeComApiClient(logger types.Logger, timeout int) *WeComApiClient {
	if timeout <= 0 {
		timeout = defaultApiTimeout
	}
	return &WeComApiClient{
		httpCli: &http.Client{Timeout: time.Duration(timeout) * time.Millisecond},
		logger:  logger,
	}
}

// DownloadFileRaw 下载文件，返回原始字节与文件名，对应 Node downloadFileRaw。
//
// 文件名从 Content-Disposition 头解析：优先 RFC 5987 扩展形式（filename*，UTF-8），
// 回退到 filename="xxx" / filename=xxx；文件名做 URL 解码（等价 decodeURIComponent）。
// 无 Content-Disposition 或未匹配到 filename 时返回空文件名。
func (c *WeComApiClient) DownloadFileRaw(url string) ([]byte, string, error) {
	c.logger.Info("Downloading file...")

	resp, err := c.httpCli.Get(url)
	if err != nil {
		c.logger.Error("File download failed:", err.Error())
		return nil, "", err
	}
	defer resp.Body.Close()

	// 镜像 axios 默认行为：非 2xx 状态码视为失败
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("unexpected status code %d", resp.StatusCode)
		c.logger.Error("File download failed:", err.Error())
		return nil, "", err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("File download failed:", err.Error())
		return nil, "", err
	}

	// 从 Content-Disposition 头解析文件名
	filename := ""
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		name, decErr := parseContentDisposition(cd)
		if decErr != nil {
			c.logger.Error("File download failed:", decErr.Error())
			return nil, "", decErr
		}
		filename = name
	}

	c.logger.Info("File downloaded successfully")
	return data, filename, nil
}

// parseContentDisposition 从 Content-Disposition 头解析文件名，对应 Node 的两段正则逻辑。
//
// 优先 RFC 5987 扩展形式（filename*，charset=UTF-8），回退到 filename="xxx" / filename=xxx；
// 文件名做 URL 解码，解码失败返回错误（镜像 Node decodeURIComponent 抛错）。
// 未匹配到 filename 时返回空串。
func parseContentDisposition(header string) (string, error) {
	if m := reFilenameExt.FindStringSubmatch(header); m != nil {
		return decodeURIComponent(m[1])
	}
	if m := reFilename.FindStringSubmatch(header); m != nil {
		return decodeURIComponent(m[1])
	}
	return "", nil
}

// decodeURIComponent 等价 JavaScript decodeURIComponent：%XX 解码且不将 '+' 转为空格。
//
// Go 标准库 url.PathUnescape 与其行为一致（仅 %XX 解码，结果按 UTF-8 解释），
// 区别于会把 '+' 转空格的 url.QueryUnescape。
func decodeURIComponent(s string) (string, error) {
	return url.PathUnescape(s)
}

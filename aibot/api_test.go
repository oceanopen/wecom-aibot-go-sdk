package aibot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ========== parseContentDisposition 多格式解析 ==========

func TestParseContentDisposition(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "RFC5987 UTF-8 中文",
			header: `attachment; filename*=UTF-8''%E4%B8%AD%E6%96%87.txt`,
			want:   "中文.txt",
		},
		{
			name:   "RFC5987 ASCII",
			header: `attachment; filename*=UTF-8''report.pdf`,
			want:   "report.pdf",
		},
		{
			name:   "RFC5987 小写 utf-8（大小写不敏感）",
			header: `attachment; filename*=utf-8''lower.txt`,
			want:   "lower.txt",
		},
		{
			name:   "filename 带引号",
			header: `attachment; filename="report.pdf"`,
			want:   "report.pdf",
		},
		{
			name:   "filename 不带引号",
			header: `attachment; filename=report.pdf`,
			want:   "report.pdf",
		},
		{
			name:   "FILENAME 全大写（大小写不敏感）",
			header: `Attachment; FILENAME="upper.TXT"`,
			want:   "upper.TXT",
		},
		{
			name:   "RFC5987 与 filename 同在，RFC5987 优先",
			header: `attachment; filename="fallback.txt"; filename*=UTF-8''%E4%B8%AD.txt`,
			want:   "中.txt",
		},
		{
			name:   "filename 含空格（空格截断，未引号）",
			header: `attachment; filename=my file.txt`,
			want:   "my",
		},
		{
			name:   "仅 attachment，无 filename",
			header: `attachment`,
			want:   "",
		},
		{
			name:   "空 header",
			header: ``,
			want:   "",
		},
		{
			name:   "RFC5987 含百分号编码的空格 %20",
			header: `attachment; filename*=UTF-8''my%20file.txt`,
			want:   "my file.txt",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseContentDisposition(tc.header)
			if err != nil {
				t.Fatalf("parseContentDisposition(%q) unexpected error: %v", tc.header, err)
			}
			if got != tc.want {
				t.Errorf("parseContentDisposition(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

// decodeURIComponent 不应将 '+' 转为空格（区别于 QueryUnescape）。
func TestDecodeURIComponentPreservesPlus(t *testing.T) {
	got, err := decodeURIComponent("a+b%2Bc.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "a+b+c.txt" {
		t.Errorf("decodeURIComponent = %q, want %q", got, "a+b+c.txt")
	}
}

// 非法百分号编码应返回错误（镜像 Node decodeURIComponent 抛错）。
func TestParseContentDispositionInvalidEscape(t *testing.T) {
	_, err := parseContentDisposition(`attachment; filename*=UTF-8''%ZZ`)
	if err == nil {
		t.Error("parseContentDisposition with invalid escape should return error")
	}
}

// ========== DownloadFileRaw 集成测试（httptest） ==========

// newDownloadServer 构造一个返回指定 body、Content-Disposition 与状态码的 httptest 服务。
func newDownloadServer(t *testing.T, status int, contentType, contentDisposition string, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentDisposition != "" {
			w.Header().Set("Content-Disposition", contentDisposition)
		}
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
		}
		_, _ = w.Write(body)
	}))
}

func TestDownloadFileRawRFC5987(t *testing.T) {
	body := []byte("file-bytes")
	srv := newDownloadServer(t, http.StatusOK, "application/octet-stream",
		`attachment; filename*=UTF-8''%E4%B8%AD%E6%96%87.txt`, body)
	defer srv.Close()

	c := NewWeComApiClient(&DefaultLogger{}, 5000)
	got, filename, err := c.DownloadFileRaw(srv.URL)
	if err != nil {
		t.Fatalf("DownloadFileRaw error: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("body = %q, want %q", got, body)
	}
	if filename != "中文.txt" {
		t.Errorf("filename = %q, want %q", filename, "中文.txt")
	}
}

func TestDownloadFileRawQuotedFilename(t *testing.T) {
	body := []byte("abc")
	srv := newDownloadServer(t, http.StatusOK, "application/pdf",
		`attachment; filename="report.pdf"`, body)
	defer srv.Close()

	c := NewWeComApiClient(&DefaultLogger{}, 5000)
	got, filename, err := c.DownloadFileRaw(srv.URL)
	if err != nil {
		t.Fatalf("DownloadFileRaw error: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("body = %q, want %q", got, body)
	}
	if filename != "report.pdf" {
		t.Errorf("filename = %q, want %q", filename, "report.pdf")
	}
}

func TestDownloadFileRawNoContentDisposition(t *testing.T) {
	body := []byte("no-cd")
	srv := newDownloadServer(t, http.StatusOK, "application/octet-stream", "", body)
	defer srv.Close()

	c := NewWeComApiClient(&DefaultLogger{}, 5000)
	got, filename, err := c.DownloadFileRaw(srv.URL)
	if err != nil {
		t.Fatalf("DownloadFileRaw error: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("body = %q, want %q", got, body)
	}
	if filename != "" {
		t.Errorf("filename = %q, want empty", filename)
	}
}

func TestDownloadFileRawNon2xxFails(t *testing.T) {
	srv := newDownloadServer(t, http.StatusNotFound, "text/plain", "", []byte("not found"))
	defer srv.Close()

	c := NewWeComApiClient(&DefaultLogger{}, 5000)
	_, _, err := c.DownloadFileRaw(srv.URL)
	if err == nil {
		t.Error("DownloadFileRaw on 404 should fail")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should contain status code 404, got: %v", err)
	}
}

func TestDownloadFileRawTimeout(t *testing.T) {
	// 服务端故意阻塞 200ms，客户端超时 50ms 必然超时
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer srv.Close()

	c := NewWeComApiClient(&DefaultLogger{}, 50)
	_, _, err := c.DownloadFileRaw(srv.URL)
	if err == nil {
		t.Error("DownloadFileRaw should timeout")
	}
}

// NewWeComApiClient timeout<=0 时取默认值，确保客户端可用。
func TestNewWeComApiClientDefaultTimeout(t *testing.T) {
	c := NewWeComApiClient(&DefaultLogger{}, 0)
	if c.httpCli.Timeout != time.Duration(defaultApiTimeout)*time.Millisecond {
		t.Errorf("default timeout = %v, want %v", c.httpCli.Timeout, defaultApiTimeout*time.Millisecond)
	}
}

package aibot

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// reqId 形如 {prefix}_{digits}_{8hex}
var reqIdRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)_(\d+)_([0-9a-f]{8})$`)

func TestGenerateReqId_Format(t *testing.T) {
	before := time.Now().UnixMilli()
	id := GenerateReqId("aibot_subscribe")
	after := time.Now().UnixMilli()

	m := reqIdRe.FindStringSubmatch(id)
	if m == nil {
		t.Fatalf("GenerateReqId(%q) = %q, want format {prefix}_{digits}_{8hex}", "aibot_subscribe", id)
	}
	if m[1] != "aibot_subscribe" {
		t.Fatalf("prefix = %q, want aibot_subscribe", m[1])
	}

	// 时间戳为纯数字且落在调用窗口内
	var ts int64
	for _, c := range m[2] {
		if c < '0' || c > '9' {
			t.Fatalf("timestamp %q contains non-digit", m[2])
		}
	}
	// 逐字符解析避免额外依赖
	for _, c := range m[2] {
		ts = ts*10 + int64(c-'0')
	}
	if ts < before || ts > after {
		t.Fatalf("timestamp = %d, want in [%d, %d]", ts, before, after)
	}

	// 随机段为 8 位小写 hex
	if len(m[3]) != 8 {
		t.Fatalf("random segment len = %d, want 8", len(m[3]))
	}
}

func TestGenerateReqId_Uniqueness(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := GenerateReqId("cmd")
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate reqId generated at i=%d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestGenerateReqId_PrefixVariants(t *testing.T) {
	for _, p := range []string{"a", "send_msg", "UploadMediaInit", ""} {
		id := GenerateReqId(p)
		// 前缀固定出现在头部并紧跟下划线
		if !strings.HasPrefix(id, p+"_") {
			t.Fatalf("reqId %q should have prefix %q followed by _", id, p)
		}
	}
}

func TestGenerateRandomString_DefaultLength(t *testing.T) {
	// 镜像 Node 默认长度 8
	s := GenerateRandomString(8)
	if len(s) != 8 {
		t.Fatalf("len = %d, want 8", len(s))
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(s) {
		t.Fatalf("random string %q is not 8 lowercase hex chars", s)
	}
}

func TestGenerateRandomString_Lengths(t *testing.T) {
	for _, n := range []int{1, 2, 5, 7, 16, 32} {
		s := GenerateRandomString(n)
		if len(s) != n {
			t.Fatalf("len(GenerateRandomString(%d)) = %d, want %d", n, len(s), n)
		}
		// 全部为小写 hex 字符
		if !regexp.MustCompile(`^[0-9a-f]*$`).MatchString(s) {
			t.Fatalf("GenerateRandomString(%d) = %q contains non-hex", n, s)
		}
	}
}

func TestGenerateRandomString_NonPositive(t *testing.T) {
	if got := GenerateRandomString(0); got != "" {
		t.Fatalf("GenerateRandomString(0) = %q, want empty", got)
	}
	if got := GenerateRandomString(-3); got != "" {
		t.Fatalf("GenerateRandomString(-3) = %q, want empty", got)
	}
}

func TestGenerateRandomString_Uniqueness(t *testing.T) {
	const n = 500
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		s := GenerateRandomString(8)
		if _, dup := seen[s]; dup {
			t.Fatalf("duplicate random string at i=%d: %s", i, s)
		}
		seen[s] = struct{}{}
	}
}

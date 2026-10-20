package aibot

import (
	"encoding/json"
	"testing"
	"time"
)

// parseCreatedAt 双形态归一矩阵：unix 秒/毫秒数字、ISO 字符串、缺省/不可解析回退当前时刻。
func TestParseCreatedAt(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Time
	}{
		{`1761012345`, time.Unix(1761012345, 0)},
		{`1761012345000`, time.UnixMilli(1761012345000)},
		{`"2026-10-21T01:23:45Z"`, time.Date(2026, 10, 21, 1, 23, 45, 0, time.UTC)},
		{`"2026-10-21T09:23:45+08:00"`, time.Date(2026, 10, 21, 1, 23, 45, 0, time.UTC)},
	}
	for _, tc := range cases {
		got := parseCreatedAt(json.RawMessage(tc.raw))
		if want := tc.want.UTC().Format("2006-01-02T15:04:05.000Z07:00"); got != want {
			t.Fatalf("parseCreatedAt(%s)=%q, want %q", tc.raw, got, want)
		}
	}
	for _, raw := range []string{``, `null`, `"not-a-time"`} {
		got := parseCreatedAt(json.RawMessage(raw))
		if _, err := time.Parse(time.RFC3339, got); err != nil {
			t.Fatalf("parseCreatedAt(%s) 回退应产出合法 ISO 8601: %q", raw, got)
		}
	}
}

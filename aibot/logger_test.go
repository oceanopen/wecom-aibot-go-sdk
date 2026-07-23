package aibot

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// 固定时间源，便于断言时间戳
func fixedNow() time.Time {
	// 2026-07-23T12:34:56.789Z（UTC）
	return time.Date(2026, 7, 23, 12, 34, 56, 789000000, time.UTC)
}

func newTestLogger() (*DefaultLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	l := NewDefaultLogger("AiBotSDK", WithLoggerNowFunc(fixedNow), WithLoggerWriter(&buf))
	return l, &buf
}

func TestDefaultLogger_ImplementsLogger(t *testing.T) {
	var _ types.Logger = (*DefaultLogger)(nil) // 编译期校验实现 types.Logger 接口
	var _ = NewDefaultLogger("")               // 仅编译期保证可用
}

func TestDefaultLogger_LevelsContainTimestampAndLevel(t *testing.T) {
	l, buf := newTestLogger()

	l.Debug("debug-msg")
	l.Info("info-msg")
	l.Warn("warn-msg")
	l.Error("error-msg")

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}

	wantTime := "[2026-07-23T12:34:56.789Z]"
	wantPrefix := "[AiBotSDK]"
	cases := []struct {
		line  string
		level string
		msg   string
	}{
		{lines[0], "DEBUG", "debug-msg"},
		{lines[1], "INFO", "info-msg"},
		{lines[2], "WARN", "warn-msg"},
		{lines[3], "ERROR", "error-msg"},
	}
	for _, c := range cases {
		if !strings.Contains(c.line, wantTime) {
			t.Errorf("line %q missing timestamp %q", c.line, wantTime)
		}
		if !strings.Contains(c.line, wantPrefix) {
			t.Errorf("line %q missing prefix %q", c.line, wantPrefix)
		}
		if !strings.Contains(c.line, "["+c.level+"]") {
			t.Errorf("line %q missing level [%s]", c.line, c.level)
		}
		if !strings.Contains(c.line, c.msg) {
			t.Errorf("line %q missing message %q", c.line, c.msg)
		}
	}
}

func TestDefaultLogger_NowFuncInjectsFixedTime(t *testing.T) {
	l, buf := newTestLogger()
	l.Info("hello")
	if !strings.Contains(buf.String(), "2026-07-23T12:34:56.789Z") {
		t.Fatalf("expected fixed timestamp in output, got: %s", buf.String())
	}
}

func TestDefaultLogger_AppendsArgs(t *testing.T) {
	l, buf := newTestLogger()
	l.Info("downloaded", "file.txt", 1024)
	out := buf.String()
	if !strings.Contains(out, "downloaded") {
		t.Fatalf("missing message, got: %s", out)
	}
	if !strings.Contains(out, "file.txt") {
		t.Fatalf("missing arg file.txt, got: %s", out)
	}
	if !strings.Contains(out, "1024") {
		t.Fatalf("missing arg 1024, got: %s", out)
	}
}

func TestDefaultLogger_DefaultPrefix(t *testing.T) {
	// prefix 传空时使用默认 "AiBotSDK"
	var buf bytes.Buffer
	l := NewDefaultLogger("", WithLoggerNowFunc(fixedNow), WithLoggerWriter(&buf))
	l.Info("msg")
	if !strings.Contains(buf.String(), "[AiBotSDK]") {
		t.Fatalf("default prefix missing, got: %s", buf.String())
	}
}

func TestDefaultLogger_CustomPrefix(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger("MyBot", WithLoggerNowFunc(fixedNow), WithLoggerWriter(&buf))
	l.Warn("careful")
	if !strings.Contains(buf.String(), "[MyBot]") {
		t.Fatalf("custom prefix missing, got: %s", buf.String())
	}
	if strings.Contains(buf.String(), "[AiBotSDK]") {
		t.Fatalf("should not contain default prefix, got: %s", buf.String())
	}
}

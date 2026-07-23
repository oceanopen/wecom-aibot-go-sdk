package types

import (
	"crypto/tls"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// TestWsClientOptions_FieldsComplete 校验 WsClientOptions 字段齐全（镜像 Node WSClientOptions），
// 且字段名与类型符合约定。本任务只定义结构，默认值由 client 兜底。
func TestWsClientOptions_FieldsComplete(t *testing.T) {
	// 期望字段名 → 期望类型（镜像 Node WSClientOptions 的全部字段）
	want := map[string]string{
		"BotId":                  "string",
		"Secret":                 "string",
		"Scene":                  "int",
		"PlugVersion":            "string",
		"ReconnectInterval":      "int",
		"MaxReconnectAttempts":   "int",
		"MaxAuthFailureAttempts": "int",
		"HeartbeatInterval":      "int",
		"RequestTimeout":         "int",
		"WsUrl":                  "string",
		"WsOptions":              "types.WsOptions",
		"MaxReplyQueueSize":      "int",
		"Logger":                 "types.Logger",
	}

	typ := reflect.TypeOf(WsClientOptions{})
	if typ.Kind() != reflect.Struct {
		t.Fatalf("WsClientOptions kind = %v, want struct", typ.Kind())
	}
	if typ.NumField() != len(want) {
		t.Fatalf("WsClientOptions has %d fields, want %d", typ.NumField(), len(want))
	}

	got := map[string]string{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		got[f.Name] = f.Type.String()
	}

	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if g != w {
			t.Errorf("field %q type = %q, want %q", name, g, w)
		}
	}
}

// TestWsClientOptions_SetGet 字段可读写（结构体可正常赋值与读取）。
func TestWsClientOptions_SetGet(t *testing.T) {
	logger := &captureLogger{}
	opts := WsClientOptions{
		BotId:                  "bot-1",
		Secret:                 "secret-1",
		Scene:                  2,
		PlugVersion:            "1.0.0",
		ReconnectInterval:      1000,
		MaxReconnectAttempts:   10,
		MaxAuthFailureAttempts: 5,
		HeartbeatInterval:      30000,
		RequestTimeout:         10000,
		WsUrl:                  "wss://openws.work.weixin.qq.com",
		MaxReplyQueueSize:      500,
		Logger:                 logger,
		WsOptions: WsOptions{
			Header:           http.Header{"X-Test": []string{"v"}},
			DialTimeout:      5 * time.Second,
			HandshakeTimeout: 3 * time.Second,
			Subprotocols:     []string{"v1"},
		},
	}

	if opts.BotId != "bot-1" || opts.Secret != "secret-1" || opts.Scene != 2 {
		t.Fatalf("basic fields mismatch: %+v", opts)
	}
	if opts.MaxAuthFailureAttempts != 5 || opts.MaxReconnectAttempts != 10 {
		t.Fatalf("attempts mismatch: %+v", opts)
	}
	if opts.MaxReplyQueueSize != 500 || opts.HeartbeatInterval != 30000 || opts.RequestTimeout != 10000 {
		t.Fatalf("timing/queue fields mismatch: %+v", opts)
	}
	if opts.WsOptions.DialTimeout != 5*time.Second {
		t.Fatalf("WsOptions.DialTimeout = %v", opts.WsOptions.DialTimeout)
	}
	if opts.Logger == nil {
		t.Fatalf("Logger should be set")
	}
}

// TestWsOptions_FieldsComplete 校验 WsOptions 字段齐全（对应 Node wsOptions）。
func TestWsOptions_FieldsComplete(t *testing.T) {
	want := map[string]string{
		"TlsConfig":        "*tls.Config",
		"Header":           "http.Header",
		"DialTimeout":      "time.Duration",
		"HandshakeTimeout": "time.Duration",
		"Subprotocols":     "[]string",
	}
	typ := reflect.TypeOf(WsOptions{})
	got := map[string]string{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		got[f.Name] = f.Type.String()
	}
	if len(got) != len(want) {
		t.Fatalf("WsOptions has %d fields, want %d", len(got), len(want))
	}
	for name, w := range want {
		if g, ok := got[name]; !ok {
			t.Errorf("missing field %q", name)
		} else if g != w {
			t.Errorf("field %q type = %q, want %q", name, g, w)
		}
	}
}

// TestWsOptions_TlsConfigWritable TlsConfig 字段可赋值，确认导入与类型可用。
func TestWsOptions_TlsConfigWritable(t *testing.T) {
	ws := WsOptions{TlsConfig: &tls.Config{ServerName: "example.com"}}
	if ws.TlsConfig == nil || ws.TlsConfig.ServerName != "example.com" {
		t.Fatalf("TlsConfig not set correctly: %+v", ws.TlsConfig)
	}
}

// captureLogger 仅用于在测试中充当 Logger 占位实现。
type captureLogger struct{}

func (*captureLogger) Debug(string, ...any) {}
func (*captureLogger) Info(string, ...any)  {}
func (*captureLogger) Warn(string, ...any)  {}
func (*captureLogger) Error(string, ...any) {}

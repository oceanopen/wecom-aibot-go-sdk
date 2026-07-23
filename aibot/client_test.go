package aibot

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// ========== WsClient 连接 + 回调集成测试 ==========

// TestWsClientConnectAndAuth 验证连接 + 认证成功触发 OnConnected/OnAuthenticated。
func TestWsClientConnectAndAuth(t *testing.T) {
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	var mu sync.Mutex
	var connected, authenticated bool
	client := NewWsClient(types.WsClientOptions{
		BotId:  "test_bot",
		Secret: "test_secret",
		WsUrl:  srv.url(),
		Logger: &DefaultLogger{},
	})
	client.OnConnected = func() {
		mu.Lock()
		connected = true
		mu.Unlock()
	}
	client.OnAuthenticated = func() {
		mu.Lock()
		authenticated = true
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	mu.Lock()
	gotConnected, gotAuthenticated := connected, authenticated
	mu.Unlock()
	if !gotConnected {
		t.Error("OnConnected was not called")
	}
	if !gotAuthenticated {
		t.Error("OnAuthenticated was not called")
	}
	if !client.IsConnected() {
		t.Error("IsConnected should be true after Connect")
	}

	client.Disconnect()
	if client.IsConnected() {
		t.Error("IsConnected should be false after Disconnect")
	}
}

// TestNewWsClientDefaults 验证零值字段兜底为默认值。
func TestNewWsClientDefaults(t *testing.T) {
	client := NewWsClient(types.WsClientOptions{
		BotId:  "b",
		Secret: "s",
	})

	if client.options.ReconnectInterval != 1000 {
		t.Errorf("ReconnectInterval = %d, want 1000", client.options.ReconnectInterval)
	}
	if client.options.MaxReconnectAttempts != 10 {
		t.Errorf("MaxReconnectAttempts = %d, want 10", client.options.MaxReconnectAttempts)
	}
	if client.options.MaxAuthFailureAttempts != 5 {
		t.Errorf("MaxAuthFailureAttempts = %d, want 5", client.options.MaxAuthFailureAttempts)
	}
	if client.options.HeartbeatInterval != 30000 {
		t.Errorf("HeartbeatInterval = %d, want 30000", client.options.HeartbeatInterval)
	}
	if client.options.RequestTimeout != 10000 {
		t.Errorf("RequestTimeout = %d, want 10000", client.options.RequestTimeout)
	}
	if client.options.MaxReplyQueueSize != 500 {
		t.Errorf("MaxReplyQueueSize = %d, want 500", client.options.MaxReplyQueueSize)
	}
	if client.options.Logger == nil {
		t.Error("Logger should default to DefaultLogger")
	}
	if client.wsManager == nil {
		t.Error("wsManager should be initialized")
	}
	if client.messageHandler == nil {
		t.Error("messageHandler should be initialized")
	}
}

// TestNewWsClientPreservesInfiniteReconnect 验证 -1（无限）不被默认值覆盖。
func TestNewWsClientPreservesInfiniteReconnect(t *testing.T) {
	client := NewWsClient(types.WsClientOptions{
		BotId:                  "b",
		Secret:                 "s",
		MaxReconnectAttempts:   -1,
		MaxAuthFailureAttempts: -1,
	})
	if client.options.MaxReconnectAttempts != -1 {
		t.Errorf("MaxReconnectAttempts = %d, want -1", client.options.MaxReconnectAttempts)
	}
	if client.options.MaxAuthFailureAttempts != -1 {
		t.Errorf("MaxAuthFailureAttempts = %d, want -1", client.options.MaxAuthFailureAttempts)
	}
}

// TestWsClientCredentials 验证 scene/plug_version 透传到认证帧 body。
func TestWsClientCredentials(t *testing.T) {
	var gotBody map[string]any
	var bodyMu sync.Mutex
	srv := newMockWsServer(func(msg []byte) []byte {
		if extractCmd(msg) == types.WsCmd.Subscribe {
			var f struct {
				Body map[string]any `json:"body"`
			}
			_ = json.Unmarshal(msg, &f)
			bodyMu.Lock()
			gotBody = f.Body
			bodyMu.Unlock()
			return authSuccessResponse(extractReqId(msg))
		}
		return nil
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{
		BotId:       "mybot",
		Secret:      "mysecret",
		Scene:       7,
		PlugVersion: "1.0.0",
		WsUrl:       srv.url(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	bodyMu.Lock()
	defer bodyMu.Unlock()
	if gotBody["bot_id"] != "mybot" {
		t.Errorf("body.bot_id = %v, want mybot", gotBody["bot_id"])
	}
	if gotBody["secret"] != "mysecret" {
		t.Errorf("body.secret = %v, want mysecret", gotBody["secret"])
	}
	if gotBody["scene"] != float64(7) {
		t.Errorf("body.scene = %v, want 7", gotBody["scene"])
	}
	if gotBody["plug_version"] != "1.0.0" {
		t.Errorf("body.plug_version = %v, want 1.0.0", gotBody["plug_version"])
	}

	client.Disconnect()
}

// TestWsClientDisconnectNotConnected 验证未启动时 Disconnect 仅告警不 panic。
func TestWsClientDisconnectNotConnected(t *testing.T) {
	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s"})
	client.Disconnect() // 不应 panic
	if client.IsConnected() {
		t.Error("IsConnected should be false for never-connected client")
	}
}

// TestWsClientOnMessagePassthrough 验证任务 15 的 OnMessage 通用透传。
func TestWsClientOnMessagePassthrough(t *testing.T) {
	// 构造一条 aibot_msg_callback 文本帧由服务端推送
	textFrame := map[string]any{
		"cmd": types.WsCmd.Callback,
		"headers": map[string]string{
			"req_id": "callback_req_1",
		},
		"body": map[string]any{
			"msgid":    "m1",
			"aibotid":  "bot1",
			"msgtype":  "text",
			"chattype": "single",
			"text":     map[string]string{"content": "hello"},
		},
	}
	pushData, _ := json.Marshal(textFrame)

	var srv *mockWsServer
	srv = newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			// 认证成功后立即推送一条文本帧
			go func() {
				time.Sleep(50 * time.Millisecond)
				_ = srv.writePush(pushData)
			}()
			return authSuccessResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	var mu sync.Mutex
	var gotMsgId, gotMsgType string
	var received bool
	client := NewWsClient(types.WsClientOptions{
		BotId:  "test_bot",
		Secret: "test_secret",
		WsUrl:  srv.url(),
	})
	client.OnMessage = func(frame *types.WsFrame[types.BaseMessage]) {
		mu.Lock()
		defer mu.Unlock()
		received = true
		gotMsgId = frame.Body.MsgId
		gotMsgType = frame.Body.MsgType
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 等待推送帧到达
	time.Sleep(200 * time.Millisecond)
	client.Disconnect()

	mu.Lock()
	defer mu.Unlock()
	if !received {
		t.Fatal("OnMessage was not called for pushed frame")
	}
	if gotMsgId != "m1" {
		t.Errorf("OnMessage body.msgid = %q, want m1", gotMsgId)
	}
	if gotMsgType != "text" {
		t.Errorf("OnMessage body.msgtype = %q, want text", gotMsgType)
	}
}

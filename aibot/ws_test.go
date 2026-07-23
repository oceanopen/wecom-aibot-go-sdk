package aibot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// ========== Mock WebSocket 服务端 ==========

// mockWsServer 本地 mock WebSocket 服务端，用于集成测试。
type mockWsServer struct {
	server      *httptest.Server
	upgrader    websocket.Upgrader
	onMessage   func(msg []byte) []byte // 收到消息时的响应逻辑
	mu          sync.Mutex
	connected   bool
	lastMessage []byte
}

func newMockWsServer(onMessage func(msg []byte) []byte) *mockWsServer {
	s := &mockWsServer{
		upgrader:  websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		onMessage: onMessage,
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.connected = true
		s.mu.Unlock()
		defer func() {
			conn.Close()
			s.mu.Lock()
			s.connected = false
			s.mu.Unlock()
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.lastMessage = msg
			s.mu.Unlock()

			if s.onMessage != nil {
				if resp := s.onMessage(msg); resp != nil {
					conn.WriteMessage(websocket.TextMessage, resp)
				}
			}
		}
	}))
	return s
}

func (s *mockWsServer) url() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http")
}

func (s *mockWsServer) close() {
	s.server.Close()
}

// ========== 认证响应辅助 ==========

// authSuccessResponse 构造认证成功响应帧。
func authSuccessResponse(reqId string) []byte {
	resp := map[string]any{
		"headers": map[string]string{"req_id": reqId},
		"errcode": 0,
		"errmsg":  "ok",
	}
	data, _ := json.Marshal(resp)
	return data
}

// authFailureResponse 构造认证失败响应帧。
func authFailureResponse(reqId string) []byte {
	resp := map[string]any{
		"headers": map[string]string{"req_id": reqId},
		"errcode": 40001,
		"errmsg":  "invalid credential",
	}
	data, _ := json.Marshal(resp)
	return data
}

// extractReqId 从帧 JSON 中提取 req_id。
func extractReqId(data []byte) string {
	var f struct {
		Headers struct {
			ReqId string `json:"req_id"`
		} `json:"headers"`
	}
	json.Unmarshal(data, &f)
	return f.Headers.ReqId
}

// extractCmd 从帧 JSON 中提取 cmd。
func extractCmd(data []byte) string {
	var f struct {
		Cmd string `json:"cmd,omitempty"`
	}
	json.Unmarshal(data, &f)
	return f.Cmd
}

// ========== 集成测试 ==========

func TestConnectAndAuth(t *testing.T) {
	// Mock 服务端：收到认证帧后回 errcode=0
	srv := newMockWsServer(func(msg []byte) []byte {
		if extractCmd(msg) == types.WsCmd.Subscribe {
			reqId := extractReqId(msg)
			return authSuccessResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	var authenticated bool
	mgr.OnAuthenticated = func() {
		authenticated = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !authenticated {
		t.Error("OnAuthenticated was not called")
	}
}

func TestConnectAuthFailure(t *testing.T) {
	// Mock 服务端：收到认证帧后回 errcode 非 0
	srv := newMockWsServer(func(msg []byte) []byte {
		if extractCmd(msg) == types.WsCmd.Subscribe {
			reqId := extractReqId(msg)
			return authFailureResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "wrong_secret", nil)

	var gotError error
	mgr.OnError = func(err error) {
		gotError = err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mgr.Connect(ctx)
	if err == nil {
		t.Fatal("Connect should fail on auth failure, but got nil")
	}

	// 验证是 AuthError
	var authErr *AuthError
	if !isAuthError(err, &authErr) {
		t.Errorf("Connect returned %T, want *AuthError", err)
	}

	if gotError == nil {
		t.Error("OnError was not called on auth failure")
	}
}

func TestConnectCtxCancel(t *testing.T) {
	// Mock 服务端：收到认证帧后不回复（模拟超时）
	srv := newMockWsServer(func(msg []byte) []byte {
		return nil // 不回复
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	ctx, cancel := context.WithCancel(context.Background())

	// 100ms 后取消 context
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := mgr.Connect(ctx)
	if err == nil {
		t.Fatal("Connect should return error on ctx cancel, but got nil")
	}

	if err != context.Canceled {
		t.Errorf("Connect returned %v, want context.Canceled", err)
	}
}

func TestSendFrame(t *testing.T) {
	// Mock 服务端：收到消息后记录
	var receivedCmd string
	var receivedMu sync.Mutex
	srv := newMockWsServer(func(msg []byte) []byte {
		if cmd := extractCmd(msg); cmd != "" {
			receivedMu.Lock()
			receivedCmd = cmd
			receivedMu.Unlock()
		}
		if extractCmd(msg) == types.WsCmd.Subscribe {
			reqId := extractReqId(msg)
			return authSuccessResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 认证帧应已发送
	time.Sleep(50 * time.Millisecond)
	receivedMu.Lock()
	cmd := receivedCmd
	receivedMu.Unlock()
	if cmd != types.WsCmd.Subscribe {
		t.Errorf("server received cmd = %q, want %q", cmd, types.WsCmd.Subscribe)
	}
}

func TestDisconnect(t *testing.T) {
	srv := newMockWsServer(func(msg []byte) []byte {
		if extractCmd(msg) == types.WsCmd.Subscribe {
			reqId := extractReqId(msg)
			return authSuccessResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !mgr.IsConnected() {
		t.Error("IsConnected should be true after Connect")
	}

	var disconnected bool
	mgr.OnDisconnected = func(reason string) {
		disconnected = true
	}
	mgr.Disconnect()

	if mgr.IsConnected() {
		t.Error("IsConnected should be false after Disconnect")
	}

	// 给 goroutine 时间执行
	time.Sleep(50 * time.Millisecond)
	if !disconnected {
		t.Error("OnDisconnected was not called after Disconnect")
	}
}

func TestAuthFrameBody(t *testing.T) {
	// 验证认证帧 body 包含 bot_id/secret/extra params
	srv := newMockWsServer(func(msg []byte) []byte {
		if extractCmd(msg) == types.WsCmd.Subscribe {
			// 验证帧内容
			var frame struct {
				Cmd     string            `json:"cmd"`
				Headers map[string]string `json:"headers"`
				Body    map[string]any    `json:"body"`
			}
			if err := json.Unmarshal(msg, &frame); err != nil {
				t.Errorf("failed to parse auth frame: %v", err)
			}
			if frame.Body["bot_id"] != "my_bot" {
				t.Errorf("body.bot_id = %v, want %q", frame.Body["bot_id"], "my_bot")
			}
			if frame.Body["secret"] != "my_secret" {
				t.Errorf("body.secret = %v, want %q", frame.Body["secret"], "my_secret")
			}
			if frame.Body["scene"] != float64(1) {
				t.Errorf("body.scene = %v, want 1", frame.Body["scene"])
			}

			reqId := extractReqId(msg)
			return authSuccessResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("my_bot", "my_secret", map[string]any{"scene": 1})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

// ========== 辅助函数 ==========

// isAuthError 检查错误是否为 *AuthError（兼容 AuthError 值/指针类型）。
func isAuthError(err error, target **AuthError) bool {
	if e, ok := err.(*AuthError); ok {
		*target = e
		return true
	}
	return false
}

// DefaultLogger 用于测试的 stub（如果 logger.go 已有 DefaultLogger 则直接用）。
// 此处仅做编译保证，实际使用 aibot.DefaultLogger。
var _ = fmt.Sprintf

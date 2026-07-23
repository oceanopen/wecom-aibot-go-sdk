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

// heartbeatAckResponse 构造心跳 ack 响应帧。
func heartbeatAckResponse(reqId string) []byte {
	resp := map[string]any{
		"headers": map[string]string{"req_id": reqId},
		"errcode": 0,
		"errmsg":  "ok",
	}
	data, _ := json.Marshal(resp)
	return data
}

// ========== 心跳集成测试 ==========

func TestHeartbeatAckResetsCount(t *testing.T) {
	// Mock 服务端：收到认证帧回 ok，收到心跳帧回 ack
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

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 连接成功后 missedPongCount 应为 0
	if mgr.missedPongCount != 0 {
		t.Errorf("missedPongCount = %d, want 0 after auth", mgr.missedPongCount)
	}

	// 等待几轮心跳（心跳间隔 30s，这里验证状态即可）
	// 心跳已启动，missedPongCount 仍应为 0（因为 mock 回了 ack）
	time.Sleep(200 * time.Millisecond)
	if mgr.missedPongCount > mgr.maxMissedPong {
		t.Errorf("missedPongCount = %d, should not exceed maxMissedPong=%d with ack", mgr.missedPongCount, mgr.maxMissedPong)
	}

	mgr.Disconnect()
}

func TestHeartbeatMissedPongDisconnect(t *testing.T) {
	// Mock 服务端：收到认证帧回 ok，收到心跳帧不回 ack（模拟心跳超时）
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		// 心跳帧不回复
		return nil
	})
	defer srv.close()

	// 用较短的心跳间隔加速测试（100ms）
	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 100, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	var disconnected bool
	var disconnectMu sync.Mutex
	mgr.OnDisconnected = func(reason string) {
		disconnectMu.Lock()
		disconnected = true
		disconnectMu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 等待足够时间让心跳超时触发断连（maxMissedPong=2, interval=100ms, 等待 ~500ms）
	time.Sleep(500 * time.Millisecond)

	disconnectMu.Lock()
	wasDisconnected := disconnected
	disconnectMu.Unlock()

	if !wasDisconnected {
		t.Error("expected disconnect due to missed heartbeat ack, but OnDisconnected was not called")
	}
}

func TestStopHeartbeatOnDisconnect(t *testing.T) {
	// 验证 Disconnect 后心跳定时器停止
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

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 心跳应已启动
	if mgr.heartbeatTimer == nil {
		t.Error("heartbeatTimer should be non-nil after auth")
	}

	mgr.Disconnect()

	// 断开后心跳定时器应已停止
	if mgr.heartbeatTimer != nil {
		t.Error("heartbeatTimer should be nil after Disconnect")
	}
}

// DefaultLogger 用于测试的 stub（如果 logger.go 已有 DefaultLogger 则直接用）。
// 此处仅做编译保证，实际使用 aibot.DefaultLogger。
var _ = fmt.Sprintf

// ========== 重连集成测试 ==========

func TestReconnectOnConnectionLost(t *testing.T) {
	// 使用两个 mock 服务端模拟断连重连：首次连接到 srv1，断开后重连到 srv2
	srv2 := newMockWsServer(func(msg []byte) []byte {
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
	defer srv2.close()

	var reconnecting bool
	var reconnMu sync.Mutex

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 100, 10, srv2.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	mgr.OnReconnecting = func(attempt int) {
		reconnMu.Lock()
		reconnecting = true
		reconnMu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 强制关闭底层连接，模拟连接断开
	mgr.mu.Lock()
	ws := mgr.ws
	mgr.mu.Unlock()
	if ws != nil {
		ws.Close()
	}

	// 等待重连触发
	time.Sleep(300 * time.Millisecond)

	reconnMu.Lock()
	wasReconnecting := reconnecting
	reconnMu.Unlock()
	if !wasReconnecting {
		t.Error("expected OnReconnecting to be called after connection loss")
	}
}

func TestAuthFailureExhausted(t *testing.T) {
	// Mock 服务端：所有认证请求都失败 → 认证耗尽
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authFailureResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 50, 10, srv.url(), types.WsOptions{}, 500, 3,
	)
	mgr.SetCredentials("test_bot", "wrong_secret", nil)

	var gotAuthExhausted bool
	var errMu sync.Mutex
	mgr.OnError = func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if _, ok := err.(*types.WsAuthFailureError); ok {
			gotAuthExhausted = true
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect 会因首次认证失败返回(返回错误
	_ = mgr.Connect(ctx)

	// 等待重连+认证耗尽（3次认证失败，每次退避50/100/200ms）
	time.Sleep(500 * time.Millisecond)

	errMu.Lock()
	wasExhausted := gotAuthExhausted
	errMu.Unlock()
	if !wasExhausted {
		t.Error("expected WsAuthFailureError when auth attempts exhausted, but OnError was not called with it")
	}
}

func TestReconnectExhausted(t *testing.T) {
	// 直接测试 scheduleReconnect 的耗尽逻辑：设置 maxReconnectAttempts=2，
	// 手动触发多次 scheduleReconnect 验证耗尽后 OnError 收到 WsReconnectExhaustedError
	srv := newMockWsServer(func(msg []byte) []byte {
		return nil
	})
	defer srv.close()

	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 50, 2, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)

	var gotReconnectExhausted bool
	var errMu sync.Mutex
	mgr.OnError = func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if _, ok := err.(*types.WsReconnectExhaustedError); ok {
			gotReconnectExhausted = true
		}
	}

	// 模拟连接断开场景（非认证失败），直接调用 scheduleReconnect 3 次
	mgr.lastCloseWasAuthFail = false
	mgr.scheduleReconnect() // attempt 1
	mgr.scheduleReconnect() // attempt 2
	mgr.scheduleReconnect() // attempt 3 → 应耗尽

	errMu.Lock()
	wasExhausted := gotReconnectExhausted
	errMu.Unlock()
	if !wasExhausted {
		t.Error("expected WsReconnectExhaustedError when reconnect attempts exhausted")
	}

	if mgr.reconnectAttempts != 2 {
		t.Errorf("reconnectAttempts = %d, want 2", mgr.reconnectAttempts)
	}
}

func TestDisconnectPreventsReconnect(t *testing.T) {
	// 验证 Disconnect 后不会触发重连
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
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

	var reconnecting bool
	var reconnMu sync.Mutex
	mgr.OnReconnecting = func(attempt int) {
		reconnMu.Lock()
		reconnecting = true
		reconnMu.Unlock()
	}

	mgr.Disconnect()

	// 等待一段时间，确认不会重连
	time.Sleep(200 * time.Millisecond)

	reconnMu.Lock()
	wasReconnecting := reconnecting
	reconnMu.Unlock()
	if wasReconnecting {
		t.Error("Disconnect should prevent reconnect, but OnReconnecting was called")
	}

	if !mgr.isManualClose {
		t.Error("isManualClose should be true after Disconnect")
	}
}

// ========== 回复队列 + ack 集成测试 ==========

// replyAckResponse 构造回复成功回执帧。
func replyAckResponse(reqId string) []byte {
	resp := map[string]any{
		"headers": map[string]string{"req_id": reqId},
		"errcode": 0,
		"errmsg":  "ok",
	}
	data, _ := json.Marshal(resp)
	return data
}

// replyAckErrorResponse 构造回复失败回执帧（errcode 非 0）。
func replyAckErrorResponse(reqId string, code int, msg string) []byte {
	resp := map[string]any{
		"headers": map[string]string{"req_id": reqId},
		"errcode": code,
		"errmsg":  msg,
	}
	data, _ := json.Marshal(resp)
	return data
}

// newAuthedMgr 创建并连接一个已认证的 WsConnectionManager，供回复测试复用。
func newAuthedMgr(t *testing.T, onMessage func(msg []byte) []byte) (*WsConnectionManager, *mockWsServer) {
	t.Helper()
	srv := newMockWsServer(onMessage)
	mgr := NewWsConnectionManager(
		&DefaultLogger{}, 30000, 1000, 10, srv.url(), types.WsOptions{}, 500, 5,
	)
	mgr.SetCredentials("test_bot", "test_secret", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mgr.Connect(ctx); err != nil {
		srv.close()
		t.Fatalf("Connect failed: %v", err)
	}
	return mgr, srv
}

// TestSendReplyAckSuccess 验证回复收到 ack 后解析成功。
func TestSendReplyAckSuccess(t *testing.T) {
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			return replyAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()
	defer mgr.Disconnect()

	reqId := GenerateReqId(types.WsCmd.Response)
	ack, err := mgr.SendReply(
		types.WsFrameHeaders{ReqId: reqId},
		map[string]any{"msgtype": "text", "text": map[string]string{"content": "hi"}},
		types.WsCmd.Response,
	)
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	if ack == nil {
		t.Fatal("ack frame is nil")
	}
	if ack.ErrCode != 0 {
		t.Errorf("ack errcode = %d, want 0", ack.ErrCode)
	}
}

// TestSendReplyTimeout 验证不回 ack 则超时报错。
func TestSendReplyTimeout(t *testing.T) {
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		// Response 帧不回复（模拟回执超时）
		return nil
	})
	defer srv.close()
	defer mgr.Disconnect()

	mgr.replyAckTimeout = 200 // 缩短超时加速测试

	reqId := GenerateReqId(types.WsCmd.Response)
	_, err := mgr.SendReply(
		types.WsFrameHeaders{ReqId: reqId},
		map[string]any{"x": 1},
		types.WsCmd.Response,
	)
	if err == nil {
		t.Fatal("SendReply should timeout, got nil error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout, got: %v", err)
	}
}

// TestSendReplyAckError 验证 errcode 非 0 时报错。
func TestSendReplyAckError(t *testing.T) {
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			return replyAckErrorResponse(reqId, 40002, "reply rejected")
		}
		return nil
	})
	defer srv.close()
	defer mgr.Disconnect()

	reqId := GenerateReqId(types.WsCmd.Response)
	ack, err := mgr.SendReply(
		types.WsFrameHeaders{ReqId: reqId},
		map[string]any{"x": 1},
		types.WsCmd.Response,
	)
	if err == nil {
		t.Fatal("SendReply should return error on errcode!=0")
	}
	if ack == nil || ack.ErrCode != 40002 {
		t.Errorf("ack frame errcode = %v (ack nil? %v), want 40002", ack, ack == nil)
	}
}

// TestSendReplySerialOrder 验证同一 reqId 的回复串行发送：
// 第一条回复延迟 ack，第二条必须等到第一条 ack 后才发送，因此服务端两次收帧间隔 ≥ 延迟。
func TestSendReplySerialOrder(t *testing.T) {
	var mu sync.Mutex
	var firstSeen, secondSeen time.Time
	seen := 0
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			mu.Lock()
			seen++
			n := seen
			if n == 1 {
				firstSeen = time.Now()
			} else if n == 2 {
				secondSeen = time.Now()
			}
			mu.Unlock()
			if n == 1 {
				time.Sleep(150 * time.Millisecond) // 第一条延迟 ack，验证第二条被阻塞
			}
			return replyAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()
	defer mgr.Disconnect()

	reqId := GenerateReqId(types.WsCmd.Response)
	headers := types.WsFrameHeaders{ReqId: reqId}

	// 并发发送两条回复（同一 reqId）
	var wg sync.WaitGroup
	var err1, err2 error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = mgr.SendReply(headers, map[string]any{"seq": 1}, types.WsCmd.Response)
	}()
	go func() {
		defer wg.Done()
		_, err2 = mgr.SendReply(headers, map[string]any{"seq": 2}, types.WsCmd.Response)
	}()
	wg.Wait()

	if err1 != nil {
		t.Errorf("first SendReply error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second SendReply error: %v", err2)
	}

	mu.Lock()
	gap := secondSeen.Sub(firstSeen)
	mu.Unlock()
	if gap < 100*time.Millisecond {
		t.Errorf("replies not serial: gap between server receipts = %v, want >= 100ms", gap)
	}
}

// TestSendReplyDefaultCmd 验证 cmd 为空时默认 WsCmd.Response。
func TestSendReplyDefaultCmd(t *testing.T) {
	var gotCmd string
	var cmdMu sync.Mutex
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			cmdMu.Lock()
			gotCmd = cmd
			cmdMu.Unlock()
			return replyAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()
	defer mgr.Disconnect()

	reqId := GenerateReqId(types.WsCmd.Response)
	_, err := mgr.SendReply(types.WsFrameHeaders{ReqId: reqId}, map[string]any{"x": 1}, "")
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	cmdMu.Lock()
	defer cmdMu.Unlock()
	if gotCmd != types.WsCmd.Response {
		t.Errorf("default cmd = %q, want %q", gotCmd, types.WsCmd.Response)
	}
}

// TestHasPendingAck 验证回复发送后、ack 前存在 pending。
func TestHasPendingAck(t *testing.T) {
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			// 延迟 ack，确保能在 pending 窗口内检查
			time.Sleep(100 * time.Millisecond)
			return replyAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()
	defer mgr.Disconnect()

	reqId := GenerateReqId(types.WsCmd.Response)
	done := make(chan struct{})
	go func() {
		_, _ = mgr.SendReply(types.WsFrameHeaders{ReqId: reqId}, map[string]any{"x": 1}, types.WsCmd.Response)
		close(done)
	}()

	// 等待回复发送、pending 建立
	time.Sleep(30 * time.Millisecond)
	if !mgr.HasPendingAck(reqId) {
		t.Error("HasPendingAck should be true while waiting for ack")
	}

	<-done
	// ack 完成后应不再有 pending
	if mgr.HasPendingAck(reqId) {
		t.Error("HasPendingAck should be false after ack received")
	}
}

// TestClearPendingOnDisconnect 验证断开连接时清理待处理回复，避免 goroutine 泄漏。
func TestClearPendingOnDisconnect(t *testing.T) {
	mgr, srv := newAuthedMgr(t, func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		// Response 帧不回复（保持 pending）
		return nil
	})
	defer srv.close()

	mgr.replyAckTimeout = 10000 // 足够长，确保由 disconnect 触发清理而非超时

	reqId := GenerateReqId(types.WsCmd.Response)
	done := make(chan error, 1)
	go func() {
		_, err := mgr.SendReply(types.WsFrameHeaders{ReqId: reqId}, map[string]any{"x": 1}, types.WsCmd.Response)
		done <- err
	}()

	// 等待回复入队并发送（pending 已建立）
	time.Sleep(100 * time.Millisecond)
	mgr.Disconnect()

	select {
	case err := <-done:
		if err == nil {
			t.Error("SendReply should return error after disconnect clears pending")
		}
	case <-time.After(2 * time.Second):
		t.Error("SendReply did not return after disconnect (possible goroutine leak)")
	}
}

package aibot

import (
	"context"
	"encoding/json"
	"errors"
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

// ========== Reply / ReplyStream / ReplyStreamNonBlocking 测试 ==========

// newReplyMockServer 构造一个 ack 流式回复的 mock 服务端，并通过 record 记录收到的 body。
func newReplyMockServer(t *testing.T, record func(body map[string]any), slowFirst bool) (*mockWsServer, *sync.Mutex, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var contents []string
	firstSeen := false
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			var f struct {
				Body struct {
					MsgType string `json:"msgtype"`
					Stream  struct {
						Content string `json:"content"`
					} `json:"stream"`
				} `json:"body"`
			}
			_ = json.Unmarshal(msg, &f)
			mu.Lock()
			if record != nil {
				var full struct {
					Body map[string]any `json:"body"`
				}
				_ = json.Unmarshal(msg, &full)
				record(full.Body)
			}
			contents = append(contents, f.Body.Stream.Content)
			isFirst := slowFirst && !firstSeen
			if isFirst {
				firstSeen = true
			}
			mu.Unlock()
			if isFirst {
				time.Sleep(300 * time.Millisecond) // 第一条延迟 ack，制造 pending
			}
			return replyAckResponse(reqId)
		}
		return nil
	})
	return srv, &mu, &contents
}

// TestReplyStreamMiddleAndFinal 验证流式中间帧 + 最终帧发送，且 body 结构正确。
func TestReplyStreamMiddleAndFinal(t *testing.T) {
	var mu sync.Mutex
	var streams []map[string]any
	srv, recMu, _ := newReplyMockServer(t, func(body map[string]any) {
		mu.Lock()
		streams = append(streams, body)
		mu.Unlock()
	}, false)
	_ = recMu
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url()})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	headers := types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}
	if _, err := client.ReplyStream(headers, "sid1", "chunk1", false, nil, nil); err != nil {
		t.Fatalf("middle frame error: %v", err)
	}
	if _, err := client.ReplyStream(headers, "sid1", "final", true, nil, nil); err != nil {
		t.Fatalf("final frame error: %v", err)
	}
	client.Disconnect()

	mu.Lock()
	defer mu.Unlock()
	if len(streams) != 2 {
		t.Fatalf("server received %d stream frames, want 2", len(streams))
	}
	for i, s := range streams {
		if s["msgtype"] != "stream" {
			t.Errorf("frame[%d].msgtype = %v, want stream", i, s["msgtype"])
		}
		stream, ok := s["stream"].(map[string]any)
		if !ok {
			t.Errorf("frame[%d].stream is not an object", i)
			continue
		}
		if stream["id"] != "sid1" {
			t.Errorf("frame[%d].stream.id = %v, want sid1", i, stream["id"])
		}
	}
	mid := streams[0]["stream"].(map[string]any)
	if mid["finish"] != false || mid["content"] != "chunk1" {
		t.Errorf("middle frame = %v, want finish=false content=chunk1", mid)
	}
	fin := streams[1]["stream"].(map[string]any)
	if fin["finish"] != true || fin["content"] != "final" {
		t.Errorf("final frame = %v, want finish=true content=final", fin)
	}
}

// TestReplyStreamNonBlockingSkip 验证非阻塞跳过路径：pending 时中间帧跳过，最终帧不跳过。
func TestReplyStreamNonBlockingSkip(t *testing.T) {
	srv, recMu, recContents := newReplyMockServer(t, nil, true)
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url()})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	headers := types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}

	// 第一条（非最终）在 goroutine 中发送，服务端延迟 ack 制造 pending
	var err1 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 = client.ReplyStream(headers, "sid", "chunk1", false, nil, nil)
	}()

	// 轮询等待第一条 pending 建立
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && !client.HasPendingReplyAck(headers) {
		time.Sleep(5 * time.Millisecond)
	}
	if !client.HasPendingReplyAck(headers) {
		t.Fatal("first frame should be pending before non-blocking calls")
	}

	// 非最终帧：上一条未 ack → 应跳过
	_, err2 := client.ReplyStreamNonBlocking(headers, "sid", "chunk2", false, nil, nil)
	if !errors.Is(err2, ErrReplySkipped) {
		t.Errorf("non-blocking middle frame err = %v, want ErrReplySkipped", err2)
	}

	// 最终帧：始终发送（阻塞至第一条 ack 后发送）
	_, err3 := client.ReplyStreamNonBlocking(headers, "sid", "final", true, nil, nil)
	if err3 != nil {
		t.Errorf("non-blocking final frame err = %v, want nil", err3)
	}

	wg.Wait()
	if err1 != nil {
		t.Errorf("first frame err = %v, want nil", err1)
	}
	client.Disconnect()

	recMu.Lock()
	defer recMu.Unlock()
	want := []string{"chunk1", "final"}
	if len(*recContents) != len(want) {
		t.Fatalf("server received %v, want %v", *recContents, want)
	}
	for i, w := range want {
		if (*recContents)[i] != w {
			t.Errorf("received[%d] = %q, want %q (chunk2 should be skipped)", i, (*recContents)[i], w)
		}
	}
}

// TestReplyGeneric 验证通用 Reply 转发自定义 body 且收到回执。
func TestReplyGeneric(t *testing.T) {
	var mu sync.Mutex
	var gotCmd, gotMsgType string
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		if cmd == types.WsCmd.Response {
			var f struct {
				Body struct {
					MsgType string `json:"msgtype"`
				} `json:"body"`
			}
			_ = json.Unmarshal(msg, &f)
			mu.Lock()
			gotCmd = cmd
			gotMsgType = f.Body.MsgType
			mu.Unlock()
			return replyAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url()})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	headers := types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}
	body := map[string]any{"msgtype": "text", "text": map[string]string{"content": "hi"}}
	ack, err := client.Reply(headers, body, types.WsCmd.Response)
	if err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if ack == nil || ack.ErrCode != 0 {
		t.Errorf("ack = %+v, want errcode 0", ack)
	}
	client.Disconnect()

	mu.Lock()
	defer mu.Unlock()
	if gotCmd != types.WsCmd.Response {
		t.Errorf("server got cmd = %q, want %q", gotCmd, types.WsCmd.Response)
	}
	if gotMsgType != "text" {
		t.Errorf("server got body.msgtype = %q, want text", gotMsgType)
	}
}

// TestReplyDefaultCmd 验证 cmd 为空时兜底为 WsCmd.Response。
func TestReplyDefaultCmd(t *testing.T) {
	var mu sync.Mutex
	var gotCmd string
	srv := newMockWsServer(func(msg []byte) []byte {
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
			gotCmd = cmd
			mu.Unlock()
			return replyAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url()})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	headers := types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}
	if _, err := client.Reply(headers, map[string]any{"msgtype": "text"}, ""); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	client.Disconnect()

	mu.Lock()
	defer mu.Unlock()
	if gotCmd != types.WsCmd.Response {
		t.Errorf("default cmd = %q, want %q", gotCmd, types.WsCmd.Response)
	}
}

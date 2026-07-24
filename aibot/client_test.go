package aibot

import (
	"context"
	"encoding/base64"
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

// ========== 任务 19：disconnected_event 不重连 ==========

// TestDisconnectedEventNoReconnect 验证服务端推送 disconnected_event 后：
// OnDisconnectedEvent/OnDisconnected 触发，且不自动重连（isManualClose 阻止）。
func TestDisconnectedEventNoReconnect(t *testing.T) {
	dcFrame := map[string]any{
		"cmd":     types.WsCmd.EventCallback,
		"headers": map[string]string{"req_id": "dc_1"},
		"body": map[string]any{
			"msgid": "dc", "msgtype": "event",
			"event": map[string]string{"eventtype": "disconnected_event"},
		},
	}
	dcData, _ := json.Marshal(dcFrame)

	var srv *mockWsServer
	srv = newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		if cmd == types.WsCmd.Subscribe {
			// 认证成功后推送 disconnected_event
			go func() {
				time.Sleep(80 * time.Millisecond)
				_ = srv.writePush(dcData)
			}()
			return authSuccessResponse(reqId)
		}
		if cmd == types.WsCmd.Heartbeat {
			return heartbeatAckResponse(reqId)
		}
		return nil
	})
	defer srv.close()

	var mu sync.Mutex
	var gotDisconnectedEvent, gotDisconnected, gotReconnecting bool
	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url()})
	client.OnDisconnectedEvent = func(*types.WsFrame[types.EventMessage]) {
		mu.Lock()
		gotDisconnectedEvent = true
		mu.Unlock()
	}
	client.OnDisconnected = func(reason string) {
		mu.Lock()
		gotDisconnected = true
		mu.Unlock()
	}
	client.OnReconnecting = func(attempt int) {
		mu.Lock()
		gotReconnecting = true
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 等待 disconnected_event 推送与处理
	time.Sleep(400 * time.Millisecond)

	mu.Lock()
	wasDE := gotDisconnectedEvent
	wasD := gotDisconnected
	wasR := gotReconnecting
	mu.Unlock()

	if !wasDE {
		t.Error("OnDisconnectedEvent should fire for disconnected_event frame")
	}
	if !wasD {
		t.Error("OnDisconnected should fire (via OnServerDisconnect bridge)")
	}
	if wasR {
		t.Error("should NOT reconnect after disconnected_event (isManualClose)")
	}
	if client.IsConnected() {
		t.Error("client should be disconnected after disconnected_event")
	}
}

// ========== DownloadFile（mock http + 加密往返解密）==========

// newWsClientNoConn 构造一个未连接的 WsClient，仅用于文件下载测试。
func newWsClientNoConn(t *testing.T) *WsClient {
	t.Helper()
	return NewWsClient(types.WsClientOptions{
		BotId:  "test_bot",
		Secret: "test_secret",
		Logger: &DefaultLogger{},
	})
}

// TestWsClientDownloadFileDecrypt 验证下载 + AES-256-CBC 解密往返，且文件名从 Content-Disposition 解析。
func TestWsClientDownloadFileDecrypt(t *testing.T) {
	key, aesKey := testAesKey(t)
	plain := []byte("confidential attachment content")
	encrypted := encryptForTest(t, plain, key)

	srv := newDownloadServer(t, 200, "application/octet-stream",
		`attachment; filename*=UTF-8''%E6%8A%A5%E5%91%8A.txt`, encrypted)
	defer srv.Close()

	client := newWsClientNoConn(t)
	got, filename, err := client.DownloadFile(srv.URL, aesKey)
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}
	if string(got) != string(plain) {
		t.Errorf("decrypted body = %q, want %q", got, plain)
	}
	if filename != "报告.txt" {
		t.Errorf("filename = %q, want %q", filename, "报告.txt")
	}
}

// TestWsClientDownloadFileNoAesKey 验证无 aesKey 时返回未解密的原始数据。
func TestWsClientDownloadFileNoAesKey(t *testing.T) {
	raw := []byte("raw-bytes-not-encrypted")
	srv := newDownloadServer(t, 200, "application/octet-stream", "", raw)
	defer srv.Close()

	client := newWsClientNoConn(t)
	got, _, err := client.DownloadFile(srv.URL, "")
	if err != nil {
		t.Fatalf("DownloadFile error: %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("raw body = %q, want %q", got, raw)
	}
}

// TestWsClientDownloadFileWrongAesKey 验证错误 aesKey 解密失败报错。
func TestWsClientDownloadFileWrongAesKey(t *testing.T) {
	key1, _ := testAesKey(t)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = byte(i + 100)
	}
	wrongAesKey := base64.StdEncoding.EncodeToString(key2)

	plain := []byte("sensitive payload")
	encrypted := encryptForTest(t, plain, key1)

	srv := newDownloadServer(t, 200, "application/octet-stream", "", encrypted)
	defer srv.Close()

	client := newWsClientNoConn(t)
	_, _, err := client.DownloadFile(srv.URL, wrongAesKey)
	if err == nil {
		t.Error("DownloadFile with wrong aesKey should fail")
	}
}

// TestWsClientDownloadFileDownloadFails 验证下载失败（404）时返回错误。
func TestWsClientDownloadFileDownloadFails(t *testing.T) {
	srv := newDownloadServer(t, 404, "text/plain", "", []byte("not found"))
	defer srv.Close()

	client := newWsClientNoConn(t)
	_, _, err := client.DownloadFile(srv.URL, "someAesKey")
	if err == nil {
		t.Error("DownloadFile on 404 should fail")
	}
}

// TestWsClientApiGetter 验证 Api() 返回内部 apiClient（非空）。
func TestWsClientApiGetter(t *testing.T) {
	client := newWsClientNoConn(t)
	if client.Api() == nil {
		t.Error("Api() should return non-nil apiClient")
	}
}

// ========== 卡片与欢迎语回复（任务 25）==========

// newCardReplyMockServer 构造一个对各类回复 cmd 都回 ack 的 mock 服务端，并通过 record 记录 (cmd, body)。
func newCardReplyMockServer(t *testing.T, record func(cmd string, body map[string]any)) *mockWsServer {
	t.Helper()
	srv := newMockWsServer(func(msg []byte) []byte {
		cmd := extractCmd(msg)
		reqId := extractReqId(msg)
		switch cmd {
		case types.WsCmd.Subscribe:
			return authSuccessResponse(reqId)
		case types.WsCmd.Heartbeat:
			return heartbeatAckResponse(reqId)
		case types.WsCmd.Response, types.WsCmd.ResponseWelcome, types.WsCmd.ResponseUpdate, types.WsCmd.SendMsg:
			if record != nil {
				var full struct {
					Body map[string]any `json:"body"`
				}
				_ = json.Unmarshal(msg, &full)
				record(cmd, full.Body)
			}
			return replyAckResponse(reqId)
		}
		return nil
	})
	return srv
}

// connectForTest 连接 client 至认证成功，返回断开+取消函数。
func connectForTest(t *testing.T, client *WsClient) func() {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := client.Connect(ctx); err != nil {
		cancel()
		t.Fatalf("Connect failed: %v", err)
	}
	return func() {
		client.Disconnect()
		cancel()
	}
}

func TestReplyWelcome(t *testing.T) {
	var mu sync.Mutex
	var gotCmd string
	var gotBody map[string]any
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		gotCmd, gotBody = cmd, body
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	welcome := types.WelcomeTextReplyBody{MsgType: "text"}
	welcome.Text.Content = "欢迎"
	headers := types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.ResponseWelcome)}
	if _, err := client.ReplyWelcome(headers, welcome); err != nil {
		t.Fatalf("ReplyWelcome error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	if gotCmd != types.WsCmd.ResponseWelcome {
		t.Errorf("cmd = %q, want %q", gotCmd, types.WsCmd.ResponseWelcome)
	}
	if gotBody["msgtype"] != "text" {
		t.Errorf("msgtype = %v, want text", gotBody["msgtype"])
	}
	txt, _ := gotBody["text"].(map[string]any)
	if txt == nil || txt["content"] != "欢迎" {
		t.Errorf("text.content = %v, want 欢迎", txt)
	}
}

func TestReplyTemplateCard(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	card := types.TemplateCard{CardType: types.TemplateCardType.TextNotice, TaskId: "t1"}
	// 带 feedback：合并到卡片
	if _, err := client.ReplyTemplateCard(types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}, card, &types.ReplyFeedback{Id: "fb1"}); err != nil {
		t.Fatalf("ReplyTemplateCard error: %v", err)
	}
	// 不带 feedback（且验证上一条未污染调用方 card）
	if _, err := client.ReplyTemplateCard(types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}, card, nil); err != nil {
		t.Fatalf("ReplyTemplateCard(nil) error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("received %d bodies, want 2", len(bodies))
	}
	if bodies[0]["msgtype"] != "template_card" {
		t.Errorf("body[0].msgtype = %v, want template_card", bodies[0]["msgtype"])
	}
	c1, _ := bodies[0]["template_card"].(map[string]any)
	if c1 == nil || c1["card_type"] != "text_notice" || c1["task_id"] != "t1" {
		t.Errorf("body[0].template_card = %v", c1)
	}
	fb, _ := c1["feedback"].(map[string]any)
	if fb == nil || fb["id"] != "fb1" {
		t.Errorf("feedback not merged into card: %v", fb)
	}
	// 第二条：feedback 应缺失（调用方 card 未被污染 + 传 nil）
	c2, _ := bodies[1]["template_card"].(map[string]any)
	if c2 == nil {
		t.Fatalf("body[1].template_card missing")
	}
	if _, ok := c2["feedback"]; ok {
		t.Errorf("feedback should be absent when nil, got %v", c2["feedback"])
	}
}

func TestReplyStreamWithCard(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	card := types.TemplateCard{CardType: types.TemplateCardType.TextNotice}
	// 全选项：streamFeedback + templateCard + cardFeedback + finish
	opts := ReplyStreamWithCardOptions{
		StreamFeedback: &types.ReplyFeedback{Id: "sfb"},
		TemplateCard:   &card,
		CardFeedback:   &types.ReplyFeedback{Id: "cfb"},
	}
	if _, err := client.ReplyStreamWithCard(types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}, "sid", "hello", true, opts); err != nil {
		t.Fatalf("ReplyStreamWithCard error: %v", err)
	}
	// 无 templateCard
	if _, err := client.ReplyStreamWithCard(types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}, "sid", "world", false, ReplyStreamWithCardOptions{}); err != nil {
		t.Fatalf("ReplyStreamWithCard(no card) error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("received %d bodies, want 2", len(bodies))
	}
	// 第一条
	if bodies[0]["msgtype"] != "stream_with_template_card" {
		t.Errorf("body[0].msgtype = %v", bodies[0]["msgtype"])
	}
	s1, _ := bodies[0]["stream"].(map[string]any)
	if s1 == nil || s1["content"] != "hello" || s1["finish"] != true {
		t.Errorf("body[0].stream = %v", s1)
	}
	sfb, _ := s1["feedback"].(map[string]any)
	if sfb == nil || sfb["id"] != "sfb" {
		t.Errorf("stream.feedback not set: %v", sfb)
	}
	tc, _ := bodies[0]["template_card"].(map[string]any)
	if tc == nil || tc["card_type"] != "text_notice" {
		t.Errorf("body[0].template_card = %v", tc)
	}
	cfb, _ := tc["feedback"].(map[string]any)
	if cfb == nil || cfb["id"] != "cfb" {
		t.Errorf("template_card.feedback not merged: %v", cfb)
	}
	// 第二条：无 template_card，无 stream.feedback
	if _, ok := bodies[1]["template_card"]; ok {
		t.Errorf("body[1] should not have template_card")
	}
	s2, _ := bodies[1]["stream"].(map[string]any)
	if s2 == nil || s2["content"] != "world" || s2["finish"] != false {
		t.Errorf("body[1].stream = %v", s2)
	}
	if _, ok := s2["feedback"]; ok {
		t.Errorf("body[1].stream.feedback should be absent")
	}
}

func TestUpdateTemplateCard(t *testing.T) {
	var mu sync.Mutex
	var recorded []struct {
		cmd  string
		body map[string]any
	}
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		recorded = append(recorded, struct {
			cmd  string
			body map[string]any
		}{cmd, body})
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	card := types.TemplateCard{CardType: types.TemplateCardType.TextNotice, TaskId: "task1"}
	// 带 userIds
	if _, err := client.UpdateTemplateCard(types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.ResponseUpdate)}, card, []string{"u1", "u2"}); err != nil {
		t.Fatalf("UpdateTemplateCard error: %v", err)
	}
	// 不带 userIds
	if _, err := client.UpdateTemplateCard(types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.ResponseUpdate)}, card, nil); err != nil {
		t.Fatalf("UpdateTemplateCard(nil) error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	if len(recorded) != 2 {
		t.Fatalf("received %d, want 2", len(recorded))
	}
	// 第一条
	if recorded[0].cmd != types.WsCmd.ResponseUpdate {
		t.Errorf("cmd = %q, want %q", recorded[0].cmd, types.WsCmd.ResponseUpdate)
	}
	if recorded[0].body["response_type"] != "update_template_card" {
		t.Errorf("response_type = %v", recorded[0].body["response_type"])
	}
	tc, _ := recorded[0].body["template_card"].(map[string]any)
	if tc == nil || tc["task_id"] != "task1" {
		t.Errorf("template_card = %v", tc)
	}
	uids, _ := recorded[0].body["userids"].([]any)
	if len(uids) != 2 || uids[0] != "u1" || uids[1] != "u2" {
		t.Errorf("userids = %v, want [u1 u2]", uids)
	}
	// 第二条：无 userids
	if _, ok := recorded[1].body["userids"]; ok {
		t.Errorf("body[1].userids should be absent when nil")
	}
}

// ========== 主动发送与媒体回复（任务 26）==========

func TestSendMessageMarkdown(t *testing.T) {
	var mu sync.Mutex
	var gotCmd string
	var gotBody map[string]any
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		gotCmd, gotBody = cmd, body
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	md := types.SendMarkdownMsgBody{MsgType: "markdown"}
	md.Markdown.Content = "**主动推送**"
	if _, err := client.SendMessage("chat_001", md); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	if gotCmd != types.WsCmd.SendMsg {
		t.Errorf("cmd = %q, want %q", gotCmd, types.WsCmd.SendMsg)
	}
	if gotBody["chatid"] != "chat_001" {
		t.Errorf("chatid = %v, want chat_001（应合并进 body）", gotBody["chatid"])
	}
	if gotBody["msgtype"] != "markdown" {
		t.Errorf("msgtype = %v, want markdown", gotBody["msgtype"])
	}
	mc, _ := gotBody["markdown"].(map[string]any)
	if mc == nil || mc["content"] != "**主动推送**" {
		t.Errorf("markdown.content = %v", mc)
	}
}

func TestSendMediaMessage(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	// image
	if _, err := client.SendMediaMessage("chat_002", types.WeComMediaImage, "img_mid", nil); err != nil {
		t.Fatalf("SendMediaMessage(image) error: %v", err)
	}
	// video with title/description
	if _, err := client.SendMediaMessage("chat_003", types.WeComMediaVideo, "vid_mid", &VideoOptions{Title: "标题", Description: "描述"}); err != nil {
		t.Fatalf("SendMediaMessage(video) error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("received %d bodies, want 2", len(bodies))
	}
	// image：仅 image 字段，无 file/voice/video
	if bodies[0]["msgtype"] != "image" || bodies[0]["chatid"] != "chat_002" {
		t.Errorf("body[0] = %v", bodies[0])
	}
	img, _ := bodies[0]["image"].(map[string]any)
	if img == nil || img["media_id"] != "img_mid" {
		t.Errorf("body[0].image = %v", img)
	}
	for _, k := range []string{"file", "voice", "video"} {
		if _, ok := bodies[0][k]; ok {
			t.Errorf("body[0].%s should be absent for image", k)
		}
	}
	// video：title/description
	if bodies[1]["msgtype"] != "video" || bodies[1]["chatid"] != "chat_003" {
		t.Errorf("body[1] = %v", bodies[1])
	}
	vid, _ := bodies[1]["video"].(map[string]any)
	if vid == nil || vid["media_id"] != "vid_mid" {
		t.Errorf("body[1].video = %v", vid)
	}
	if vid["title"] != "标题" || vid["description"] != "描述" {
		t.Errorf("video title/description = %v", vid)
	}
}

func TestReplyMedia(t *testing.T) {
	var mu sync.Mutex
	var gotCmd string
	var gotBody map[string]any
	srv := newCardReplyMockServer(t, func(cmd string, body map[string]any) {
		mu.Lock()
		gotCmd, gotBody = cmd, body
		mu.Unlock()
	})
	defer srv.close()

	client := NewWsClient(types.WsClientOptions{BotId: "b", Secret: "s", WsUrl: srv.url(), Logger: &DefaultLogger{}})
	disconnect := connectForTest(t, client)

	headers := types.WsFrameHeaders{ReqId: GenerateReqId(types.WsCmd.Response)}
	if _, err := client.ReplyMedia(headers, types.WeComMediaFile, "file_mid", nil); err != nil {
		t.Fatalf("ReplyMedia error: %v", err)
	}
	disconnect()

	mu.Lock()
	defer mu.Unlock()
	// 被动回复：cmd=Response，无 chatid
	if gotCmd != types.WsCmd.Response {
		t.Errorf("cmd = %q, want %q", gotCmd, types.WsCmd.Response)
	}
	if _, ok := gotBody["chatid"]; ok {
		t.Errorf("ReplyMedia body should not have chatid（被动回复不合并 chatid）")
	}
	if gotBody["msgtype"] != "file" {
		t.Errorf("msgtype = %v, want file", gotBody["msgtype"])
	}
	f, _ := gotBody["file"].(map[string]any)
	if f == nil || f["media_id"] != "file_mid" {
		t.Errorf("file.media_id = %v", f)
	}
	for _, k := range []string{"image", "voice", "video"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("body.%s should be absent for file", k)
		}
	}
}

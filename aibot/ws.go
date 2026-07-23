package aibot

// ws.go 对应 Node src/ws.ts：WsConnectionManager 连接管理器（拨号/认证/心跳/重连/串行回复队列+ack）。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/oceanopen/wecom-aibot-go-sdk/aibot/types"
)

// DefaultWsUrl SDK 内置默认 WebSocket 连接地址，对应 Node DEFAULT_WS_URL。
const DefaultWsUrl = "wss://openws.work.weixin.qq.com"

// authResult 认证结果，通过 channel 传递给阻塞等待的 Connect。
type authResult struct {
	Success bool   // 认证是否成功
	ErrMsg  string // 错误信息（失败时）
}

// WsConnectionManager WebSocket 长连接管理器，对应 Node WsConnectionManager。
//
// 负责维护与企业微信的 WebSocket 长连接，包括认证、心跳、重连等。
type WsConnectionManager struct {
	mu sync.Mutex // 保护并发访问

	// 连接状态
	ws     *websocket.Conn // 当前 WebSocket 连接
	closed atomic.Bool     // 是否已断开（去重，防止 onDisconnected 双触发）

	// 配置
	logger                 types.Logger
	wsUrl                  string
	wsOptions              types.WsOptions
	heartbeatInterval      int // 心跳间隔（毫秒）
	maxReconnectAttempts   int // 连接断开最大重连次数，-1 无限
	maxAuthFailureAttempts int // 认证失败最大重试次数，-1 无限
	reconnectBaseDelay     int // 重连基础延迟（毫秒）
	maxReplyQueueSize      int // 单 req_id 回复队列最大长度
	replyAckTimeout        int // 回执超时时间（毫秒）

	// 认证凭证
	botId           string
	botSecret       string
	extraAuthParams map[string]any

	// 认证结果通道（Connect 阻塞等待用）
	authCh chan authResult

	// 回调
	OnConnected        func()                      // 连接建立回调（WebSocket open 事件，认证尚未完成）
	OnAuthenticated    func()                      // 认证成功回调
	OnDisconnected     func(reason string)         // 连接断开回调
	OnMessage          func(frame json.RawMessage) // 收到消息回调（原始 JSON）
	OnReconnecting     func(attempt int)           // 重连回调
	OnError            func(err error)             // 错误回调
	OnServerDisconnect func(reason string)         // 服务端主动断开回调
}

// NewWsConnectionManager 构造 WsConnectionManager，参数镜像 Node WsConnectionManager constructor。
func NewWsConnectionManager(
	logger types.Logger,
	heartbeatInterval int,
	reconnectBaseDelay int,
	maxReconnectAttempts int,
	wsUrl string,
	wsOptions types.WsOptions,
	maxReplyQueueSize int,
	maxAuthFailureAttempts int,
) *WsConnectionManager {
	url := wsUrl
	if url == "" {
		url = DefaultWsUrl
	}
	return &WsConnectionManager{
		logger:                 logger,
		heartbeatInterval:      heartbeatInterval,
		reconnectBaseDelay:     reconnectBaseDelay,
		maxReconnectAttempts:   maxReconnectAttempts,
		maxAuthFailureAttempts: maxAuthFailureAttempts,
		wsUrl:                  url,
		wsOptions:              wsOptions,
		maxReplyQueueSize:      maxReplyQueueSize,
		replyAckTimeout:        5000,
		authCh:                 make(chan authResult, 1),
	}
}

// SetCredentials 设置认证凭证，对应 Node setCredentials。
func (m *WsConnectionManager) SetCredentials(botId, botSecret string, extraAuthParams map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.botId = botId
	m.botSecret = botSecret
	m.extraAuthParams = extraAuthParams
}

// Connect 建立 WebSocket 连接并阻塞至首次认证成功，对应 Node connect()。
//
// 与 Node 版本不同，Go 版本阻塞等待认证结果，更符合 Go 惯例。
// ctx 取消时中断等待并关闭连接。
func (m *WsConnectionManager) Connect(ctx context.Context) error {
	m.closed.Store(false)

	// 拨号
	if err := m.connectOnce(); err != nil {
		return err
	}

	// 阻塞等待认证结果
	select {
	case result := <-m.authCh:
		if !result.Success {
			return &AuthError{Msg: result.ErrMsg}
		}
		return nil
	case <-ctx.Done():
		m.disconnect("context cancelled")
		return ctx.Err()
	}
}

// connectOnce 单次拨号：建立 WebSocket 连接并设置事件处理器。
func (m *WsConnectionManager) connectOnce() error {
	m.logger.Info(fmt.Sprintf("Connecting to WebSocket: %s...", m.wsUrl))

	// 构建 gorilla dialer
	dialer := websocket.Dialer{
		TLSClientConfig:  m.wsOptions.TlsConfig,
		HandshakeTimeout: m.wsOptions.HandshakeTimeout,
		Subprotocols:     m.wsOptions.Subprotocols,
	}

	// 构建请求 Header
	header := http.Header{}
	for k, v := range m.wsOptions.Header {
		header[k] = v
	}

	// 拨号
	ws, _, err := dialer.DialContext(context.Background(), m.wsUrl, header)
	if err != nil {
		m.logger.Error(fmt.Sprintf("Failed to create WebSocket connection: %s", err.Error()))
		return err
	}

	m.mu.Lock()
	m.ws = ws
	m.mu.Unlock()

	// 设置事件处理器
	m.setupEventHandlers()

	return nil
}

// setupEventHandlers 设置 WebSocket 事件处理器，对应 Node setupEventHandlers。
func (m *WsConnectionManager) setupEventHandlers() {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()
	if ws == nil {
		return
	}

	// 连接建立：发送认证帧
	m.logger.Info("WebSocket connection established, sending auth...")
	m.sendAuth()
	if m.OnConnected != nil {
		m.OnConnected()
	}

	// 启动读循环
	go m.readLoop()
}

// readLoop 持续读取 WebSocket 消息并分发。
func (m *WsConnectionManager) readLoop() {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()
	if ws == nil {
		return
	}

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			// 连接关闭或读取错误
			if !m.closed.Load() {
				m.closed.Store(true)
				reason := err.Error()
				m.logger.Warn(fmt.Sprintf("WebSocket connection closed: %s", reason))
				if m.OnDisconnected != nil {
					m.OnDisconnected(reason)
				}
			}
			return
		}

		// 解析帧
		var frame json.RawMessage
		frame = message
		m.handleFrame(frame)
	}
}

// handleFrame 处理收到的帧数据，对应 Node handleFrame。
//
// 任务 11 仅实现认证响应路由；心跳/事件/回复回执在后续任务补充。
func (m *WsConnectionManager) handleFrame(raw json.RawMessage) {
	// 先解析出 cmd 和 headers.req_id 以路由
	var probe struct {
		Cmd     string `json:"cmd,omitempty"`
		Headers struct {
			ReqId string `json:"req_id"`
		} `json:"headers"`
		ErrCode int    `json:"errcode,omitempty"`
		ErrMsg  string `json:"errmsg,omitempty"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		m.logger.Error(fmt.Sprintf("Failed to parse WebSocket frame: %s", err.Error()))
		return
	}

	reqId := probe.Headers.ReqId

	// 无 cmd 的帧：认证响应或心跳响应，通过 req_id 前缀区分
	if probe.Cmd == "" {
		// 认证响应（req_id 以 aibot_subscribe 开头）
		if reqId != "" && len(reqId) > len(types.WsCmd.Subscribe) && reqId[:len(types.WsCmd.Subscribe)] == types.WsCmd.Subscribe {
			m.handleAuthResponse(probe.ErrCode, probe.ErrMsg)
			return
		}
		// 心跳响应（req_id 以 ping 开头）— 任务 12 补充
		if reqId != "" && len(reqId) >= 4 && reqId[:4] == types.WsCmd.Heartbeat {
			m.handleHeartbeatResponse(probe.ErrCode, probe.ErrMsg)
			return
		}
		// 未知无 cmd 帧
		m.logger.Warn(fmt.Sprintf("Received unknown frame (no cmd): %s", string(raw)))
		return
	}

	// 有 cmd 的帧
	switch probe.Cmd {
	case types.WsCmd.Callback:
		m.logger.Debug(fmt.Sprintf("[server -> plugin] cmd=%s, reqId=%s", probe.Cmd, reqId))
		if m.OnMessage != nil {
			m.OnMessage(raw)
		}
	case types.WsCmd.EventCallback:
		m.logger.Debug(fmt.Sprintf("[server -> plugin] cmd=%s, reqId=%s", probe.Cmd, reqId))
		if m.OnMessage != nil {
			m.OnMessage(raw)
		}
	default:
		m.logger.Warn(fmt.Sprintf("Received unknown cmd: %s", probe.Cmd))
	}
}

// handleAuthResponse 处理认证响应，对应 Node handleFrame 中认证响应分支。
func (m *WsConnectionManager) handleAuthResponse(errCode int, errMsg string) {
	if errCode != 0 {
		m.logger.Error(fmt.Sprintf("Authentication failed: errcode=%d, errmsg=%s", errCode, errMsg))
		if m.OnError != nil {
			m.OnError(&AuthError{Msg: fmt.Sprintf("Authentication failed: %s (code: %d)", errMsg, errCode)})
		}
		// 发送认证失败结果
		select {
		case m.authCh <- authResult{Success: false, ErrMsg: errMsg}:
		default:
		}
		// 认证失败，关闭连接
		m.disconnect("authentication failed")
		return
	}

	m.logger.Info("Authentication successful")
	// 发送认证成功结果
	select {
	case m.authCh <- authResult{Success: true}:
	default:
	}
	if m.OnAuthenticated != nil {
		m.OnAuthenticated()
	}
}

// handleHeartbeatResponse 处理心跳响应，任务 12 补充完整逻辑。
func (m *WsConnectionManager) handleHeartbeatResponse(errCode int, errMsg string) {
	_ = errCode
	_ = errMsg
	// 任务 12 补充：重置 missedPongCount
}

// sendAuth 发送认证帧，对应 Node sendAuth。
//
// 格式：{ cmd: "aibot_subscribe", headers: { req_id }, body: { bot_id, secret, ... } }
func (m *WsConnectionManager) sendAuth() {
	m.mu.Lock()
	botId := m.botId
	botSecret := m.botSecret
	extra := m.extraAuthParams
	m.mu.Unlock()

	reqId := GenerateReqId(types.WsCmd.Subscribe)

	// 构建 body
	body := map[string]any{
		"bot_id": botId,
		"secret": botSecret,
	}
	for k, v := range extra {
		body[k] = v
	}

	frame := map[string]any{
		"cmd": types.WsCmd.Subscribe,
		"headers": map[string]string{
			"req_id": reqId,
		},
		"body": body,
	}

	if err := m.sendJSON(frame); err != nil {
		m.logger.Error(fmt.Sprintf("Failed to send auth frame: %s", err.Error()))
	} else {
		m.logger.Info("Auth frame sent")
	}
}

// sendJSON 发送 JSON 数据帧，对应 Node send。
func (m *WsConnectionManager) sendJSON(v any) error {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()

	if ws == nil {
		return &NotConnectedError{}
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return ws.WriteMessage(websocket.TextMessage, data)
}

// disconnect 主动断开连接，设置 closed 标记防止双触发。
func (m *WsConnectionManager) disconnect(reason string) {
	if m.closed.Swap(true) {
		return // 已经断开，避免双触发
	}

	m.mu.Lock()
	ws := m.ws
	m.ws = nil
	m.mu.Unlock()

	if ws != nil {
		ws.Close()
	}

	m.logger.Info(fmt.Sprintf("WebSocket connection closed: %s", reason))
	if m.OnDisconnected != nil {
		m.OnDisconnected(reason)
	}
}

// Disconnect 主动断开连接（公开方法），对应 Node disconnect。
func (m *WsConnectionManager) Disconnect() {
	m.disconnect("manual close")
}

// IsConnected 检查当前连接状态，对应 Node get isConnected。
func (m *WsConnectionManager) IsConnected() bool {
	m.mu.Lock()
	ws := m.ws
	m.mu.Unlock()
	return ws != nil && !m.closed.Load()
}

// ========== 辅助错误类型 ==========

// AuthError 认证错误。
type AuthError struct {
	Msg string
}

func (e *AuthError) Error() string { return e.Msg }

// NotConnectedError WebSocket 未连接错误。
type NotConnectedError struct{}

func (e *NotConnectedError) Error() string { return "WebSocket not connected" }

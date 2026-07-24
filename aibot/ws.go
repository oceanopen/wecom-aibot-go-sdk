package aibot

// ws.go 对应 Node src/ws.ts：WsConnectionManager 连接管理器（拨号/认证/心跳/重连/串行回复队列+ack）。

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

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

	// 心跳状态
	missedPongCount int         // 连续未收到心跳 ack 的次数
	maxMissedPong   int         // 连续未 ack 最大次数，超过后视为连接死亡
	heartbeatTimer  *time.Timer // 心跳定时器

	// 重连状态
	reconnectAttempts    int         // 连接断开重连次数
	authFailureAttempts  int         // 认证失败重试次数
	isManualClose        bool        // 是否主动关闭（阻止自动重连）
	lastCloseWasAuthFail bool        // 最近一次关闭是否因认证失败（用于 scheduleReconnect 区分重连类型）
	reconnectTimer       *time.Timer // 重连定时器
	reconnectMaxDelay    int         // 重连延迟上限（毫秒）

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

	// 回复队列状态
	replyQueues   map[string][]*replyQueueItem // 按 req_id 分组的回复队列，保证同一 req_id 串行发送
	pendingAcks   map[string]*pendingAck       // 正在等待回执的 req_id，含超时定时器与序列号
	pendingAckSeq int                          // 自增序列号，区分同一 reqId 的不同 pending，防超时与 ack 竞态
	replyMu       sync.Mutex                   // 保护 replyQueues/pendingAcks/pendingAckSeq

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
		maxMissedPong:          2,
		reconnectMaxDelay:      30000,
		authCh:                 make(chan authResult, 1),
		replyQueues:            make(map[string][]*replyQueueItem),
		pendingAcks:            make(map[string]*pendingAck),
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
	m.isManualClose = false
	m.closed.Store(false)

	// 取消挂起的重连定时器，防止与当前 Connect 竞态
	if m.reconnectTimer != nil {
		m.reconnectTimer.Stop()
		m.reconnectTimer = nil
	}

	// 拨号
	if err := m.connectOnce(ctx); err != nil {
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
func (m *WsConnectionManager) connectOnce(ctx context.Context) error {
	m.logger.Info(fmt.Sprintf("Connecting to WebSocket: %s...", m.wsUrl))

	// 构建 gorilla dialer
	dialer := websocket.Dialer{
		TLSClientConfig:  m.wsOptions.TlsConfig,
		HandshakeTimeout: m.wsOptions.HandshakeTimeout,
		Subprotocols:     m.wsOptions.Subprotocols,
	}

	// 构建请求 Header
	header := http.Header{}
	maps.Copy(header, m.wsOptions.Header)

	// 拨号
	ws, _, err := dialer.DialContext(ctx, m.wsUrl, header)
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

	// 连接建立：重置心跳计数和认证失败标记，发送认证帧
	m.missedPongCount = 0
	m.lastCloseWasAuthFail = false
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
			m.stopHeartbeat()
			if !m.closed.Load() {
				m.closed.Store(true)
				reason := err.Error()
				m.logger.Warn(fmt.Sprintf("WebSocket connection closed: %s", reason))
				// 清理所有待处理回复（对应 Node close 事件中的 clearPendingMessages）
				m.clearPendingMessages(fmt.Sprintf("WebSocket connection closed (%s)", reason))
				if m.OnDisconnected != nil {
					m.OnDisconnected(reason)
				}
				// 非主动关闭时触发重连
				if !m.isManualClose {
					m.scheduleReconnect()
				}
			}
			return
		}

		// 解析帧
		var frame json.RawMessage = message
		m.handleFrame(frame)
	}
}

// handleFrame 处理收到的帧数据，对应 Node handleFrame。
//
// 路由：无 cmd 帧按 req_id 前缀区分认证/心跳响应，否则按 pendingAcks 匹配回复回执；
// 有 cmd 帧按 cmd 分发消息/事件回调（disconnected_event 触发服务端断连处理）。
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
		// 心跳响应（req_id 以 ping 开头）
		if reqId != "" && len(reqId) >= 4 && reqId[:4] == types.WsCmd.Heartbeat {
			m.handleHeartbeatResponse(probe.ErrCode, probe.ErrMsg)
			return
		}
		// 回复消息回执（req_id 存在于 pendingAcks 中）
		m.replyMu.Lock()
		_, hasPendingAck := m.pendingAcks[reqId]
		m.replyMu.Unlock()
		if hasPendingAck {
			m.handleReplyAck(reqId, raw)
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
		// disconnected_event：有新连接建立，服务端通知旧连接即将被断开
		if isDisconnectedEvent(raw) {
			m.logger.Warn("Received disconnected_event: a new connection has been established, this connection will be closed by server")
			// 先分发事件给上层（OnEvent/OnDisconnectedEvent），再做清理与断连
			if m.OnMessage != nil {
				m.OnMessage(raw)
			}
			m.handleServerDisconnect("New connection established, server disconnected this connection")
			return
		}
		if m.OnMessage != nil {
			m.OnMessage(raw)
		}
	default:
		m.logger.Warn(fmt.Sprintf("Received unknown cmd: %s", probe.Cmd))
	}
}

// handleServerDisconnect 处理 disconnected_event：服务端因新连接建立主动断开旧连接，
// 对应 Node ws.ts handleFrame 中 disconnected_event 分支。
//
// 顺序：stopHeartbeat → clearPendingMessages → 置 isManualClose（阻止重连）→
// 置 closed（防止 readLoop 错误路径双触发 onDisconnected）→ OnServerDisconnect → 关闭 socket。
func (m *WsConnectionManager) handleServerDisconnect(reason string) {
	m.stopHeartbeat()
	m.clearPendingMessages("Server disconnected due to new connection")
	// 阻止自动重连（服务端正常行为，重连也会被再次断开）
	m.isManualClose = true
	// 置 closed 标记，避免 ws.Close 触发 readLoop 错误路径再次触发 onDisconnected
	m.closed.Store(true)
	if m.OnServerDisconnect != nil {
		m.OnServerDisconnect(reason)
	}
	m.mu.Lock()
	ws := m.ws
	m.ws = nil
	m.mu.Unlock()
	if ws != nil {
		ws.Close()
	}
}

// isDisconnectedEvent 判断帧是否为 disconnected_event 事件推送。
func isDisconnectedEvent(raw json.RawMessage) bool {
	var probe struct {
		Body struct {
			Event struct {
				EventType string `json:"eventtype"`
			} `json:"event"`
		} `json:"body"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.Body.Event.EventType == types.EventType.Disconnected
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
		// 标记为认证失败，readLoop 退出后 scheduleReconnect 会据此使用 authFailureAttempts 计数器
		m.lastCloseWasAuthFail = true
		// 认证失败，关闭底层连接，触发 readLoop 退出进而 scheduleReconnect
		m.mu.Lock()
		ws := m.ws
		m.mu.Unlock()
		if ws != nil {
			ws.Close()
		}
		return
	}

	m.logger.Info("Authentication successful")
	// 认证成功：重置所有重连计数器
	m.reconnectAttempts = 0
	m.authFailureAttempts = 0
	m.startHeartbeat()
	// 发送认证成功结果
	select {
	case m.authCh <- authResult{Success: true}:
	default:
	}
	if m.OnAuthenticated != nil {
		m.OnAuthenticated()
	}
}

// handleHeartbeatResponse 处理心跳响应，对应 Node handleFrame 中心跳响应分支。
func (m *WsConnectionManager) handleHeartbeatResponse(errCode int, errMsg string) {
	if errCode != 0 {
		m.logger.Warn(fmt.Sprintf("Heartbeat ack error: errcode=%d, errmsg=%s", errCode, errMsg))
		return
	}
	m.missedPongCount = 0
}

// startHeartbeat 启动心跳定时器，对应 Node startHeartbeat。
func (m *WsConnectionManager) startHeartbeat() {
	m.stopHeartbeat()
	interval := time.Duration(m.heartbeatInterval) * time.Millisecond
	m.heartbeatTimer = time.AfterFunc(interval, func() {
		m.sendHeartbeat()
		// 重新调度下一次心跳
		if !m.closed.Load() {
			m.startHeartbeat()
		}
	})
	m.logger.Debug(fmt.Sprintf("Heartbeat timer started, interval: %dms", m.heartbeatInterval))
}

// stopHeartbeat 停止心跳定时器，对应 Node stopHeartbeat。
func (m *WsConnectionManager) stopHeartbeat() {
	if m.heartbeatTimer != nil {
		m.heartbeatTimer.Stop()
		m.heartbeatTimer = nil
		m.logger.Debug("Heartbeat timer stopped")
	}
}

// sendHeartbeat 发送心跳，对应 Node sendHeartbeat。
//
// 格式：{ cmd: "ping", headers: { req_id } }
// 发送前检查 missedPongCount，若连续未 ack 次数达到 maxMissedPong 则视为连接死亡并强制断连。
func (m *WsConnectionManager) sendHeartbeat() {
	// 检查连续未收到 ack 的次数，在发送下一条心跳前判定连接是否死亡
	if m.missedPongCount >= m.maxMissedPong {
		m.logger.Warn(fmt.Sprintf("No heartbeat ack received for %d consecutive pings, connection considered dead", m.missedPongCount))
		m.stopHeartbeat()
		// 强制关闭底层 socket，触发 readLoop 退出
		m.mu.Lock()
		ws := m.ws
		m.mu.Unlock()
		if ws != nil {
			ws.Close()
		}
		return
	}

	m.missedPongCount++
	frame := map[string]any{
		"cmd": types.WsCmd.Heartbeat,
		"headers": map[string]string{
			"req_id": GenerateReqId(types.WsCmd.Heartbeat),
		},
	}
	if err := m.sendJSON(frame); err != nil {
		m.logger.Error(fmt.Sprintf("Failed to send heartbeat: %s", err.Error()))
	}
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
	maps.Copy(body, extra)

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

	m.stopHeartbeat()

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
	m.isManualClose = true
	m.stopHeartbeat()

	// 清理所有待处理回复（对应 Node disconnect 中的 clearPendingMessages）
	m.clearPendingMessages("Connection manually closed")

	// 取消挂起的重连定时器
	if m.reconnectTimer != nil {
		m.reconnectTimer.Stop()
		m.reconnectTimer = nil
	}

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

// scheduleReconnect 安排重连，对应 Node scheduleReconnect。
//
// 区分两种重连场景，使用独立的计数器和最大重试次数：
//   - 认证失败（lastCloseWasAuthFail=true）：使用 authFailureAttempts / maxAuthFailureAttempts
//   - 连接断开（lastCloseWasAuthFail=false）：使用 reconnectAttempts / maxReconnectAttempts
//
// disconnected_event（被踢下线）不会触发重连，因为 isManualClose 已被设为 true。
func (m *WsConnectionManager) scheduleReconnect() {
	if m.lastCloseWasAuthFail {
		// 认证失败场景
		if m.maxAuthFailureAttempts != -1 && m.authFailureAttempts >= m.maxAuthFailureAttempts {
			m.logger.Error(fmt.Sprintf("Max auth failure attempts reached (%d), giving up", m.maxAuthFailureAttempts))
			if m.OnError != nil {
				m.OnError(types.NewWsAuthFailureError(m.maxAuthFailureAttempts))
			}
			return
		}
		m.authFailureAttempts++

		delay := m.reconnectBaseDelay * (1 << (m.authFailureAttempts - 1)) // 2^(n-1)
		if delay > m.reconnectMaxDelay {
			delay = m.reconnectMaxDelay
		}

		m.logger.Info(fmt.Sprintf("Auth failed, reconnecting in %dms (auth attempt %d/%d)...", delay, m.authFailureAttempts, m.maxAuthFailureAttempts))
		if m.OnReconnecting != nil {
			m.OnReconnecting(m.authFailureAttempts)
		}

		m.reconnectTimer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
			m.reconnectTimer = nil
			if m.isManualClose {
				return
			}
			m.reconnect()
		})
	} else {
		// 连接断开场景（网络异常、心跳超时等）
		if m.maxReconnectAttempts != -1 && m.reconnectAttempts >= m.maxReconnectAttempts {
			m.logger.Error(fmt.Sprintf("Max reconnect attempts reached (%d), giving up", m.maxReconnectAttempts))
			if m.OnError != nil {
				m.OnError(types.NewWsReconnectExhaustedError(m.maxReconnectAttempts))
			}
			return
		}
		m.reconnectAttempts++

		delay := m.reconnectBaseDelay * (1 << (m.reconnectAttempts - 1)) // 2^(n-1)
		if delay > m.reconnectMaxDelay {
			delay = m.reconnectMaxDelay
		}

		m.logger.Info(fmt.Sprintf("Connection lost, reconnecting in %dms (attempt %d/%d)...", delay, m.reconnectAttempts, m.maxReconnectAttempts))
		if m.OnReconnecting != nil {
			m.OnReconnecting(m.reconnectAttempts)
		}

		m.reconnectTimer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
			m.reconnectTimer = nil
			if m.isManualClose {
				return
			}
			m.reconnect()
		})
	}
}

// reconnect 执行一次重连：重新拨号并发送认证帧。
func (m *WsConnectionManager) reconnect() {
	m.closed.Store(false)
	if err := m.connectOnce(context.Background()); err != nil {
		m.logger.Error(fmt.Sprintf("Reconnect failed: %s", err.Error()))
		m.scheduleReconnect()
	}
}

// ========== 串行回复队列 + ack ==========

// replyQueueItem 回复队列中的单个任务项，对应 Node ReplyQueueItem。
type replyQueueItem struct {
	frame any            // 要发送的帧数据
	ackCh chan ackResult // 回执结果通道（成功传入回执帧，失败/超时传入错误）
}

// ackResult 回执结果，通过 replyQueueItem.ackCh 传递。
type ackResult struct {
	frame *types.WsFrame[json.RawMessage] // 回执帧（成功/errcode 非 0 时提供）
	err   error                           // 错误（超时/errcode 非 0/发送失败）
}

// pendingAck 正在等待回执的状态，对应 Node pendingAcks 的 value。
type pendingAck struct {
	item  *replyQueueItem // 关联的队列项（即队首）
	timer *time.Timer     // 回执超时定时器
	seq   int             // 唯一序列号，用于超时回调校验是否是当前 pending
}

// SendReply 通过 WebSocket 通道发送回复消息（串行队列版本），对应 Node sendReply。
//
// 同一个 req_id 的消息会被放入队列中串行发送：发送一条后等待服务端回执，
// 收到回执或超时后才发送下一条。阻塞至收到回执（返回回执帧）、超时或出错。
//
// 格式：{ cmd, headers: { req_id }, body }
// cmd 为空时默认 WsCmd.Response。
func (m *WsConnectionManager) SendReply(frame types.WsFrameHeaders, body any, cmd string) (*types.WsFrame[json.RawMessage], error) {
	if cmd == "" {
		cmd = types.WsCmd.Response
	}
	reqId := frame.ReqId

	item := &replyQueueItem{
		frame: map[string]any{
			"cmd":     cmd,
			"headers": map[string]string{"req_id": reqId},
			"body":    body,
		},
		ackCh: make(chan ackResult, 1),
	}

	// 入队
	m.replyMu.Lock()
	queue := m.replyQueues[reqId]
	// 防止队列无限增长导致内存泄漏
	if len(queue) >= m.maxReplyQueueSize {
		m.replyMu.Unlock()
		err := fmt.Errorf("Reply queue for reqId %s exceeds max size (%d)", reqId, m.maxReplyQueueSize)
		m.logger.Warn(err.Error())
		return nil, err
	}
	queue = append(queue, item)
	m.replyQueues[reqId] = queue
	// 队列中只有这一条，说明当前空闲，立即开始处理
	startProcessing := len(queue) == 1
	m.replyMu.Unlock()

	if startProcessing {
		m.processReplyQueue(reqId)
	}

	// 阻塞等待回执
	result := <-item.ackCh
	return result.frame, result.err
}

// processReplyQueue 处理指定 req_id 的回复队列，对应 Node processReplyQueue（即 replyProcessor）：
// 取出队首消息发送，并注册 pendingAck 等待回执。
//
// 每次只处理一条：发送成功后注册 pending 并返回；回执/超时会触发下一次调用。
// 发送失败时循环跳过当前项继续下一条，避免递归栈溢出。
func (m *WsConnectionManager) processReplyQueue(reqId string) {
	m.replyMu.Lock()
	defer m.replyMu.Unlock()

	for {
		queue := m.replyQueues[reqId]
		if len(queue) == 0 {
			// 队列为空，清理
			delete(m.replyQueues, reqId)
			return
		}

		item := queue[0]

		// 发送帧
		if err := m.sendJSON(item.frame); err != nil {
			m.logger.Error(fmt.Sprintf("Failed to send reply for reqId %s: %s", reqId, err.Error()))
			// 发送失败：移除当前项并通知，继续处理下一条
			m.replyQueues[reqId] = queue[1:]
			item.ackCh <- ackResult{err: err}
			continue
		}

		// 分配唯一序列号，用于超时回调校验是否是当前 pending（防竞态）
		m.pendingAckSeq++
		seq := m.pendingAckSeq

		// 注册到 pendingAcks，并设置回执超时定时器
		m.pendingAcks[reqId] = &pendingAck{
			item: item,
			seq:  seq,
			timer: time.AfterFunc(time.Duration(m.replyAckTimeout)*time.Millisecond, func() {
				m.handleReplyTimeout(reqId, seq)
			}),
		}
		m.logger.Debug(fmt.Sprintf("Reply message sent via WebSocket, reqId: %s, queue length: %d", reqId, len(queue)))
		// 等待回执；ack/超时后会再次触发 processReplyQueue 处理下一条
		return
	}
}

// handleReplyAck 处理回复消息的回执，对应 Node handleReplyAck。
//
// 收到回执后释放当前项，继续处理队列中的下一条。
func (m *WsConnectionManager) handleReplyAck(reqId string, raw json.RawMessage) {
	m.replyMu.Lock()
	pending, ok := m.pendingAcks[reqId]
	if !ok {
		m.replyMu.Unlock()
		return
	}

	// 清除超时定时器
	pending.timer.Stop()
	delete(m.pendingAcks, reqId)

	item := pending.item
	// 移除队首（即当前 pending 项）
	if q := m.replyQueues[reqId]; len(q) > 0 && q[0] == item {
		m.replyQueues[reqId] = q[1:]
	}
	m.replyMu.Unlock()

	// 解析回执帧
	var ack types.WsFrame[json.RawMessage]
	result := ackResult{}
	if err := json.Unmarshal(raw, &ack); err != nil {
		result.err = fmt.Errorf("failed to parse reply ack for reqId %s: %w", reqId, err)
		m.logger.Error(result.err.Error())
	} else if ack.ErrCode != 0 {
		// 失败：errcode 非 0，同时提供回执帧便于诊断
		result.err = fmt.Errorf("Reply ack error: reqId=%s, errcode=%d, errmsg=%s", reqId, ack.ErrCode, ack.ErrMsg)
		result.frame = &ack
		m.logger.Warn(result.err.Error())
	} else {
		// 成功：回执帧
		result.frame = &ack
		m.logger.Debug(fmt.Sprintf("Reply ack received for reqId: %s", reqId))
	}
	item.ackCh <- result

	// 继续处理队列中的下一条
	m.processReplyQueue(reqId)
}

// handleReplyTimeout 处理回复回执超时，对应 Node processReplyQueue 中的超时分支。
//
// 通过 seq 校验：若不匹配说明当前 pending 已被正常 ack 处理过，直接忽略（过期回调）。
func (m *WsConnectionManager) handleReplyTimeout(reqId string, seq int) {
	m.replyMu.Lock()
	pending, ok := m.pendingAcks[reqId]
	if !ok || pending.seq != seq {
		// 过期的超时回调，忽略
		m.replyMu.Unlock()
		return
	}

	delete(m.pendingAcks, reqId)
	m.logger.Warn(fmt.Sprintf("Reply ack timeout (%dms) for reqId: %s", m.replyAckTimeout, reqId))

	item := pending.item
	// 移除队首（即当前 pending 项）
	if q := m.replyQueues[reqId]; len(q) > 0 && q[0] == item {
		m.replyQueues[reqId] = q[1:]
	}
	m.replyMu.Unlock()

	// 超时：通知当前项失败
	item.ackCh <- ackResult{err: fmt.Errorf("Reply ack timeout (%dms) for reqId: %s", m.replyAckTimeout, reqId)}

	// 继续处理队列中的下一条
	m.processReplyQueue(reqId)
}

// HasPendingAck 检查指定 reqId 是否有待回执的消息（即上一条消息还未收到 ack），
// 对应 Node hasPendingAck。
//
// 用于流式场景：调用方可据此决定是否跳过当前帧，避免排队积压。
func (m *WsConnectionManager) HasPendingAck(reqId string) bool {
	m.replyMu.Lock()
	defer m.replyMu.Unlock()
	_, ok := m.pendingAcks[reqId]
	return ok
}

// clearPendingMessages 清理所有待处理的消息和回执，对应 Node clearPendingMessages。
//
// 先清理 pendingAcks（正在等待回执的队首项），再清理 replyQueues（排队中的后续项），
// 跳过已在 pendingAcks 中被 reject 的项，避免双重通知。
func (m *WsConnectionManager) clearPendingMessages(reason string) {
	m.replyMu.Lock()

	// 先清理 pendingAcks：清除定时器并通知正在等待回执的消息
	notified := make(map[*replyQueueItem]bool)
	for reqId, pending := range m.pendingAcks {
		pending.timer.Stop()
		notified[pending.item] = true
		pending.item.ackCh <- ackResult{err: fmt.Errorf("%s, reply for reqId: %s cancelled", reason, reqId)}
	}
	m.pendingAcks = make(map[string]*pendingAck)

	// 再清理 replyQueues：跳过已在 pendingAcks 中被 reject 过的队首项
	for reqId, queue := range m.replyQueues {
		for _, item := range queue {
			if notified[item] {
				continue
			}
			item.ackCh <- ackResult{err: fmt.Errorf("%s, reply for reqId: %s cancelled", reason, reqId)}
		}
	}
	m.replyQueues = make(map[string][]*replyQueueItem)

	m.replyMu.Unlock()
}

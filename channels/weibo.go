package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/smallnest/goclaw/bus"
	"github.com/smallnest/goclaw/config"
	"github.com/smallnest/goclaw/internal/logger"
	"go.uber.org/zap"
)

const (
	defaultWeiboWSEndpoint    = "ws://open-im.api.weibo.com/ws/stream"
	defaultWeiboTokenEndpoint = "http://open-im.api.weibo.com/open/auth/ws_token"
	weiboPingInterval         = 30 * time.Second
	weiboPongTimeout          = 10 * time.Second
	weiboInitialReconnectDelay = 1 * time.Second
	weiboMaxReconnectDelay    = 60 * time.Second
)

type WeiboChannel struct {
	*BaseChannelImpl
	appID         string
	appSecret     string
	wsEndpoint    string
	tokenEndpoint string
	dmPolicy      string
	allowFrom     []string

	conn          *websocket.Conn
	connMu        sync.Mutex
	token         string
	tokenExpiry   time.Time
	tokenMu       sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	lastPongTime  time.Time
}

type weiboTokenResponse struct {
	Data struct {
		Token    string `json:"token"`
		ExpireIn int    `json:"expire_in"`
	} `json:"data"`
}

type weiboMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type weiboMessagePayload struct {
	MessageID   string `json:"messageId"`
	FromUserID  string `json:"fromUserId"`
	Text        string `json:"text"`
	CreateTime  int64  `json:"createTime"`
}

type weiboSendMessage struct {
	Type    string `json:"type"`
	Payload struct {
		ToUserID  string `json:"toUserId"`
		Text      string `json:"text"`
		MessageID string `json:"messageId"`
		ChunkID   int    `json:"chunkId"`
		Done      bool   `json:"done"`
	} `json:"payload"`
}

func NewWeiboChannel(accountID string, cfg config.WeiboChannelConfig, bus *bus.MessageBus) (*WeiboChannel, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("weibo app_id and app_secret are required")
	}

	wsEndpoint := cfg.WSEndpoint
	if wsEndpoint == "" {
		wsEndpoint = defaultWeiboWSEndpoint
	}

	tokenEndpoint := cfg.TokenEndpoint
	if tokenEndpoint == "" {
		tokenEndpoint = defaultWeiboTokenEndpoint
	}

	dmPolicy := cfg.DMPolicy
	if dmPolicy == "" {
		dmPolicy = "open"
	}

	baseCfg := BaseChannelConfig{
		Enabled:    cfg.Enabled,
		AccountID:  accountID,
		AllowedIDs: cfg.AllowFrom,
	}

	return &WeiboChannel{
		BaseChannelImpl: NewBaseChannelImpl("weibo", accountID, baseCfg, bus),
		appID:           cfg.AppID,
		appSecret:       cfg.AppSecret,
		wsEndpoint:      wsEndpoint,
		tokenEndpoint:   tokenEndpoint,
		dmPolicy:        dmPolicy,
		allowFrom:       cfg.AllowFrom,
	}, nil
}

func (c *WeiboChannel) Start(ctx context.Context) error {
	if err := c.BaseChannelImpl.Start(ctx); err != nil {
		return err
	}

	logger.Info("Starting Weibo channel",
		zap.String("app_id", c.appID),
		zap.String("ws_endpoint", c.wsEndpoint))

	c.ctx, c.cancel = context.WithCancel(ctx)

	c.wg.Add(1)
	go c.run()

	return nil
}

func (c *WeiboChannel) run() {
	defer c.wg.Done()

	reconnectDelay := weiboInitialReconnectDelay

	for {
		select {
		case <-c.ctx.Done():
			logger.Info("Weibo channel stopped by context")
			return
		default:
			if err := c.connect(); err != nil {
				logger.Error("Weibo connection failed, will retry",
					zap.Error(err),
					zap.Duration("retry_after", reconnectDelay))

				select {
				case <-c.ctx.Done():
					return
				case <-time.After(reconnectDelay):
				}

				reconnectDelay *= 2
				if reconnectDelay > weiboMaxReconnectDelay {
					reconnectDelay = weiboMaxReconnectDelay
				}
				continue
			}

			reconnectDelay = weiboInitialReconnectDelay
			c.handleConnection()
		}
	}
}

func (c *WeiboChannel) connect() error {
	token, err := c.getValidToken()
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	u, err := url.Parse(c.wsEndpoint)
	if err != nil {
		return fmt.Errorf("invalid ws endpoint: %w", err)
	}

	q := u.Query()
	q.Set("app_id", c.appID)
	q.Set("token", token)
	u.RawQuery = q.Encode()

	c.connMu.Lock()
	defer c.connMu.Unlock()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(c.ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to dial websocket: %w", err)
	}

	c.conn = conn
	c.lastPongTime = time.Now()

	logger.Info("Weibo WebSocket connected")
	return nil
}

func (c *WeiboChannel) getValidToken() (string, error) {
	c.tokenMu.RLock()
	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-60*time.Second)) {
		token := c.token
		c.tokenMu.RUnlock()
		return token, nil
	}
	c.tokenMu.RUnlock()

	return c.fetchToken()
}

func (c *WeiboChannel) fetchToken() (string, error) {
	payload := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	resp, err := http.Post(c.tokenEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token fetch failed: %d %s", resp.StatusCode, resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp weiboTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Data.Token == "" {
		return "", fmt.Errorf("invalid token response: missing token")
	}

	c.tokenMu.Lock()
	c.token = tokenResp.Data.Token
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.Data.ExpireIn) * time.Second)
	c.tokenMu.Unlock()

	logger.Debug("Weibo token fetched",
		zap.Int("expires_in", tokenResp.Data.ExpireIn))

	return tokenResp.Data.Token, nil
}

func (c *WeiboChannel) handleConnection() {
	defer c.closeConnection()

	pingTicker := time.NewTicker(weiboPingInterval)
	defer pingTicker.Stop()

	msgChan := make(chan []byte, 100)
	errChan := make(chan error, 1)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			c.connMu.Lock()
			conn := c.conn
			c.connMu.Unlock()

			if conn == nil {
				errChan <- fmt.Errorf("connection closed")
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- message
		}
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-pingTicker.C:
			c.sendPing()
		case message := <-msgChan:
			c.handleMessage(message)
		case err := <-errChan:
			logger.Warn("Weibo WebSocket error", zap.Error(err))
			return
		}
	}
}

func (c *WeiboChannel) sendPing() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return
	}

	if time.Since(c.lastPongTime) > weiboPingInterval+weiboPongTimeout {
		logger.Warn("Weibo pong timeout, closing connection")
		c.conn.Close()
		return
	}

	pingMsg := `{"type":"ping"}`
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(pingMsg)); err != nil {
		logger.Warn("Failed to send ping", zap.Error(err))
	}
}

func (c *WeiboChannel) handleMessage(message []byte) {
	text := string(message)
	if text == "pong" || text == `{"type":"pong"}` {
		c.lastPongTime = time.Now()
		return
	}

	var msg weiboMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		logger.Warn("Failed to parse Weibo message", zap.Error(err))
		return
	}

	if msg.Type == "message" {
		c.handleInboundMessage(msg.Payload)
	}
}

func (c *WeiboChannel) handleInboundMessage(payload json.RawMessage) {
	var msgPayload weiboMessagePayload
	if err := json.Unmarshal(payload, &msgPayload); err != nil {
		logger.Warn("Failed to parse Weibo message payload", zap.Error(err))
		return
	}

	senderID := msgPayload.FromUserID
	if !c.isAllowed(senderID) {
		return
	}

	msg := &bus.InboundMessage{
		ID:        msgPayload.MessageID,
		Content:   msgPayload.Text,
		AccountID: c.AccountID(),
		SenderID:  senderID,
		ChatID:    "user:" + senderID,
		Channel:   c.Name(),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"create_time": msgPayload.CreateTime,
			"msg_id":      msgPayload.MessageID,
		},
	}

	logger.Debug("Weibo message received",
		zap.String("sender", senderID),
		zap.String("content", msgPayload.Text))

	_ = c.PublishInbound(context.Background(), msg)
}

func (c *WeiboChannel) isAllowed(senderID string) bool {
	if c.dmPolicy == "open" {
		return true
	}

	if len(c.allowFrom) == 0 {
		return c.dmPolicy != "closed"
	}

	for _, id := range c.allowFrom {
		if id == senderID {
			return true
		}
	}

	return false
}

func (c *WeiboChannel) Send(msg *bus.OutboundMessage) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("Weibo WebSocket not connected")
	}

	userID := msg.ChatID
	if strings.HasPrefix(userID, "user:") {
		userID = strings.TrimPrefix(userID, "user:")
	}

	messageID := fmt.Sprintf("msg_%d_%s", time.Now().UnixMilli(), generateRandomString(8))
	if id, ok := msg.Metadata["message_id"].(string); ok && id != "" {
		messageID = id
	}

	sendMsg := weiboSendMessage{
		Type: "send_message",
	}
	sendMsg.Payload.ToUserID = userID
	sendMsg.Payload.Text = msg.Content
	sendMsg.Payload.MessageID = messageID
	sendMsg.Payload.ChunkID = 0
	sendMsg.Payload.Done = true

	data, err := json.Marshal(sendMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	logger.Debug("Weibo message sent",
		zap.String("to_user", userID),
		zap.Int("content_len", len(msg.Content)))

	return nil
}

func (c *WeiboChannel) Stop() error {
	logger.Info("Stopping Weibo channel")

	if c.cancel != nil {
		c.cancel()
	}

	c.closeConnection()
	c.wg.Wait()

	return c.BaseChannelImpl.Stop()
}

func (c *WeiboChannel) closeConnection() {
	c.connMu.Lock()
	conn := c.conn
	c.conn = nil
	c.connMu.Unlock()

	if conn != nil {
		conn.Close()
	}
}

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

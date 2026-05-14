// Package ws 提供基于 topic 的 WebSocket 进度推送中心。
//
// 使用方式:
//
//	hub := ws.NewHub(log)
//	go hub.Run(ctx)
//	// HTTP/Gin handler
//	hub.ServeWS(c.Writer, c.Request, c.Query("topic"))
//	// 生产者(worker/service)
//	hub.Publish("script:42", ws.Event{Type: "progress", Percent: 0.6, Message: "splitting"})
package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"git.myscrm.cn/ganqx01/ai-script/backend/pkg/metrics"
)

// DefaultRedisChannel 跨进程事件桥接默认 Redis 通道。
const DefaultRedisChannel = "ai-script:ws:events"

// Event 任务进度事件
type Event struct {
	Topic   string         `json:"topic"`
	Type    string         `json:"type"` // progress / done / error / log
	Percent float64        `json:"percent,omitempty"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
	Time    int64          `json:"time"`
}

// Client 单条 WS 连接 + 订阅的 topic
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	topic    string
	send     chan Event
	lastPong time.Time
	mu       sync.Mutex
}

// Hub 管理所有 client + topic 路由
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{} // topic -> clients

	register   chan *Client
	unregister chan *Client
	broadcast  chan Event

	upgrader websocket.Upgrader
	log      *zap.Logger

	// 可选的 Redis 跨进程桥接(worker 进程 publish -> server 进程订阅 -> 推 client)
	rdb     *redis.Client
	channel string
}

func NewHub(log *zap.Logger, allowedOrigins []string) *Hub {
	// 修复 P0 A4 — WebSocket CheckOrigin 不再无条件放行
	checkOrigin := func(r *http.Request) bool {
		if len(allowedOrigins) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Referer")
		}
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				return true
			}
		}
		return false
	}
	return &Hub{
		clients:    make(map[string]map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan Event, 4096),
		log:        log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     checkOrigin,
		},
	}
}

// BindRedis 启用跨进程 Redis Pub/Sub 桥接。当被绑定时:
//   - Publish 会把事件发送到 Redis 通道而不是直接进入本地 broadcast
//   - Run 中会启动订阅协程,把从 Redis 收到的事件路由到本地 broadcast
//
// 必须在 Run 之前调用。channel 为空时使用 DefaultRedisChannel。
func (h *Hub) BindRedis(rdb *redis.Client, channel string) {
	if channel == "" {
		channel = DefaultRedisChannel
	}
	h.rdb = rdb
	h.channel = channel
}

// Run 启动调度循环。应在独立 goroutine 中运行。
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 如果绑定了 Redis,启动订阅协程把跨进程事件路由到本地 broadcast
	if h.rdb != nil {
		go h.subscribeRedis(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case c := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[c.topic]; !ok {
				h.clients[c.topic] = make(map[*Client]struct{})
			}
			h.clients[c.topic][c] = struct{}{}
			h.mu.Unlock()
			metrics.ActiveWSConnections.Add(1)
		case c := <-h.unregister:
			h.mu.Lock()
			if set, ok := h.clients[c.topic]; ok {
				if _, exists := set[c]; exists {
					delete(set, c)
					close(c.send)
					if len(set) == 0 {
						delete(h.clients, c.topic)
					}
				}
			}
			h.mu.Unlock()
			metrics.ActiveWSConnections.Add(-1)
		case ev := <-h.broadcast:
			h.mu.RLock()
			set := h.clients[ev.Topic]
			for c := range set {
				select {
				case c.send <- ev:
				default:
					// slow consumer, drop & evict next round
				}
			}
			h.mu.RUnlock()
		case <-ticker.C:
			// keepalive ping + 检测超时未 pong 的连接
			now := time.Now()
			h.mu.RLock()
			for _, set := range h.clients {
				for c := range set {
					c.mu.Lock()
					lastPong := c.lastPong
					c.mu.Unlock()
					if !lastPong.IsZero() && now.Sub(lastPong) > pongWait {
						// 超时未收到 pong，关闭连接（由 readPump/writePump 清理）
						_ = c.conn.Close()
						continue
					}
					_ = c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second))
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Publish 投递事件到指定 topic 的所有订阅者。
// 若 Hub 绑定了 Redis,则发布到 Redis 通道;由订阅协程负责再次路由进 broadcast。
func (h *Hub) Publish(topic string, ev Event) {
	ev.Topic = topic
	if ev.Time == 0 {
		ev.Time = time.Now().Unix()
	}
	// 无论是否绑定 Redis,都先走本地 broadcast,确保本进程订阅者能立即收到
	select {
	case h.broadcast <- ev:
	default:
		if h.log != nil {
			h.log.Warn("ws broadcast queue full, dropping event", zap.String("topic", topic))
		}
	}
	// 若绑定了 Redis,再发布到 Redis 通道供跨进程消费
	if h.rdb != nil {
		buf, err := json.Marshal(ev)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := h.rdb.Publish(ctx, h.channel, buf).Err(); err != nil && h.log != nil {
				h.log.Warn("ws redis publish failed", zap.String("topic", topic), zap.Error(err))
			}
		}
	}
}

// subscribeRedis 订阅 Redis 通道,把收到的事件投递到本地 broadcast 通道。
func (h *Hub) subscribeRedis(ctx context.Context) {
	sub := h.rdb.Subscribe(ctx, h.channel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev Event
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			select {
			case h.broadcast <- ev:
			default:
				if h.log != nil {
					h.log.Warn("ws broadcast queue full, dropping event", zap.String("topic", ev.Topic))
				}
			}
		}
	}
}

// ServeWS 把 HTTP 升级为 WS 并把 client 注册到指定 topic。
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, topic string) error {
	if topic == "" {
		http.Error(w, "topic required", http.StatusBadRequest)
		return nil
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	c := &Client{
		hub:      h,
		conn:     conn,
		topic:    topic,
		send:     make(chan Event, 32),
		lastPong: time.Now(),
	}
	h.register <- c
	go c.writePump()
	go c.readPump()
	return nil
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	maxMsgSize = 1024
)

func (c *Client) writePump() {
	defer func() {
		_ = c.conn.Close()
	}()
	for ev := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
		buf, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, buf); err != nil {
			return
		}
	}
	_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
	}()
	c.conn.SetReadLimit(maxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.mu.Lock()
		c.lastPong = time.Now()
		c.mu.Unlock()
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		// 我们不期望客户端发消息(纯 server push),读到 EOF 时退出
		if _, _, err := c.conn.NextReader(); err != nil {
			return
		}
	}
}

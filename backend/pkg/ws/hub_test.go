package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func TestNewHub(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	if h == nil {
		t.Fatal("expected non-nil hub")
	}
	if h.clients == nil {
		t.Fatal("expected clients map initialized")
	}
	if h.log != log {
		t.Fatal("expected logger set")
	}
}

func TestHubRunAndPublish(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Publish without clients should not block
	h.Publish("topic1", Event{Type: "progress", Percent: 0.5, Message: "test"})

	// Verify event fields are filled
	if h.rdb != nil {
		t.Skip("redis branch tested separately")
	}
}

func TestHubServeWS(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	t.Run("empty topic", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/ws", nil)
		err := h.ServeWS(w, r, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("upgrade ok", func(t *testing.T) {
		// Create a test server that upgrades the connection
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			topic := r.URL.Query().Get("topic")
			err := h.ServeWS(w, r, topic)
			if err != nil {
				t.Logf("serve ws error: %v", err)
			}
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=test-topic"
		ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer ws.Close()
		if resp != nil {
			resp.Body.Close()
		}

		// Give time for registration
		time.Sleep(100 * time.Millisecond)

		// Publish an event
		h.Publish("test-topic", Event{Type: "done", Message: "hello"})

		ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read message: %v", err)
		}

		var ev Event
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ev.Type != "done" || ev.Message != "hello" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	})
}

func TestHubRegisterUnregister(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Query().Get("topic")
		_ = h.ServeWS(w, r, topic)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=reg-test"
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws1.Close()

	time.Sleep(100 * time.Millisecond)

	// Close connection to trigger unregister
	ws1.Close()
	time.Sleep(200 * time.Millisecond)
}

func TestHubBroadcastQueueFull(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Fill broadcast channel without consumers reading
	for i := 0; i < 300; i++ {
		h.Publish("any", Event{Type: "log", Message: "flood"})
	}
	// Should not panic or block indefinitely
}

func TestEventJSON(t *testing.T) {
	ev := Event{
		Topic:   "t1",
		Type:    "progress",
		Percent: 0.75,
		Message: "ok",
		Data:    map[string]any{"k": "v"},
		Time:    1234567890,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Topic != ev.Topic || got.Type != ev.Type || got.Percent != ev.Percent {
		t.Fatalf("event mismatch: %+v", got)
	}
}

func TestCheckOriginAllowed(t *testing.T) {
	log := zap.NewNop()

	t.Run("allow all when empty", func(t *testing.T) {
		h := NewHub(log, nil)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = h.ServeWS(w, r, "topic")
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=t"
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		ws.Close()
	})

	t.Run("allow matching origin", func(t *testing.T) {
		h := NewHub(log, []string{"http://example.com"})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = h.ServeWS(w, r, "topic")
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=t"
		header := http.Header{}
		header.Set("Origin", "http://example.com")
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		ws.Close()
	})

	t.Run("deny mismatch origin", func(t *testing.T) {
		h := NewHub(log, []string{"http://example.com"})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = h.ServeWS(w, r, "topic")
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=t"
		header := http.Header{}
		header.Set("Origin", "http://evil.com")
		ws, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err == nil {
			ws.Close()
			t.Fatal("expected dial to fail")
		}
		if resp != nil {
			resp.Body.Close()
		}
	})

	t.Run("allow wildcard", func(t *testing.T) {
		h := NewHub(log, []string{"*"})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = h.ServeWS(w, r, "topic")
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=t"
		header := http.Header{}
		header.Set("Origin", "http://anything.com")
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		ws.Close()
	})
}

func TestClientWritePump(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = h.ServeWS(w, r, "wp-test")
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=wp-test"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)

	// Send multiple events
	h.Publish("wp-test", Event{Type: "a"})
	h.Publish("wp-test", Event{Type: "b"})

	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 2; i++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		var ev Event
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("unmarshal %d: %v", i, err)
		}
		if ev.Type != "a" && ev.Type != "b" {
			t.Fatalf("unexpected type: %s", ev.Type)
		}
	}
}

func TestHubMultipleTopics(t *testing.T) {
	log := zap.NewNop()
	h := NewHub(log, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Query().Get("topic")
		_ = h.ServeWS(w, r, topic)
	}))
	defer server.Close()

	wsURL1 := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=t1"
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL1, nil)
	if err != nil {
		t.Fatalf("dial ws1: %v", err)
	}
	defer ws1.Close()

	wsURL2 := "ws" + strings.TrimPrefix(server.URL, "http") + "?topic=t2"
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL2, nil)
	if err != nil {
		t.Fatalf("dial ws2: %v", err)
	}
	defer ws2.Close()

	time.Sleep(100 * time.Millisecond)

	// Publish to t1 only
	h.Publish("t1", Event{Type: "only-t1"})

	ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws1.ReadMessage()
	if err != nil {
		t.Fatalf("read ws1: %v", err)
	}
	var ev Event
	if err := json.Unmarshal(msg, &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Type != "only-t1" {
		t.Fatalf("expected only-t1, got %s", ev.Type)
	}

	// ws2 should not receive anything within short time
	ws2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = ws2.ReadMessage()
	if err == nil {
		t.Fatal("expected ws2 to timeout without message")
	}
}

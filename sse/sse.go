package sse

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

type Hub struct {
	mu      sync.Mutex
	clients map[chan Event]bool
}

type Event struct {
	Event string
	Data  string
}

// NewHub は新しい Hub を作成して返します。
func NewHub() *Hub {
	return &Hub{
		mu:      sync.Mutex{},
		clients: make(map[chan Event]bool),
	}
}

// Subscribe は新しいクライアント用のチャネルを作成し、Hub に登録して返します。
func (h *Hub) Subscribe() chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	// バッファ付きチャネルにしておくことでブロックを防止
	ch := make(chan Event, 200)
	h.clients[ch] = true
	return ch
}

// UnSubscribe は指定されたクライアント用のチャネルを Hub から削除します。
func (h *Hub) UnSubscribe(key chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, key)
}

// Publish は Hub に登録されている全てのクライアントにメッセージを送信します。
func (h *Hub) Publish(msg Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// クライアントがまだデータを受け取っていない場合、そのクライアントへのメッセージは破棄
			slog.LogAttrs(context.Background(), slog.LevelWarn, "client channel is full, dropping message")
		}
	}
}

// NewSSEHandler は Hub に接続するための SSE ハンドラを返します。
func (h *Hub) NewSSEHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch := h.Subscribe()
		defer h.UnSubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-ch:
				writeData(w, event)
				flusher.Flush()
			}
		}
	})
}

// writeData は指定された io.Writer に対して SSE 形式で Event を書き込みます。
func writeData(w io.Writer, e Event) {
	// SSE形式でデータを書き込む。SSEでは改行が重要なため、eの各フィールドの文字列については末尾の改行を削除しておく。
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", strings.TrimRight(e.Event, "\n"), strings.TrimRight(e.Data, "\n"))
}

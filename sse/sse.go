package sse

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

type Hub struct {
	mu      sync.Mutex
	clients map[chan string]bool
}

func NewHub() *Hub {
	return &Hub{
		mu:      sync.Mutex{},
		clients: make(map[chan string]bool),
	}
}

func (h *Hub) Subscribe() chan string {
	h.mu.Lock()
	defer h.mu.Unlock()

	// バッファ付きチャネルにしておくことでブロックを防止
	ch := make(chan string, 200)
	h.clients[ch] = true
	return ch
}

func (h *Hub) UnSubscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, ch)
}

func (h *Hub) Publish(msg string) {
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

		fmt.Fprint(w, "data: connected to sse server\n\n")
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				// クライアントが接続を切った場合
				return
			case msg := <-ch:
				// TODO: id, eventもSSEデータに追加したい...
				fmt.Fprintf(w, "data: %s\n\n", strings.TrimSpace(msg))
				flusher.Flush()
			}
		}
	})
}

package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/takaya-47/lg-monitor/internal/monitor"
)

type Hub struct {
	mu      sync.Mutex
	clients map[chan event]bool
}

type event struct {
	Event string
	Data  string
}

type monitorResultPayload struct {
	MonitorTargetID int       `json:"monitor_target_id"`
	CheckedAt       time.Time `json:"checked_at"`
	IsSuccess       bool      `json:"is_success"`
	StatusCode      *int      `json:"status_code"`
	ResponseTimeMs  *int      `json:"response_time_ms"`
	ErrorMessage    string    `json:"error_message"`
}

// NewHub は新しい Hub を作成して返します。
func NewHub() *Hub {
	return &Hub{
		mu:      sync.Mutex{},
		clients: make(map[chan event]bool),
	}
}

// subscribe は新しいクライアント用のチャネルを作成し、Hub に登録して返します。
func (h *Hub) subscribe() chan event {
	h.mu.Lock()
	defer h.mu.Unlock()

	// バッファ付きチャネルにしておくことでブロックを防止
	ch := make(chan event, 200)
	h.clients[ch] = true
	return ch
}

// unSubscribe は指定されたクライアント用のチャネルを Hub から削除します。
func (h *Hub) unSubscribe(key chan event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, key)
}

// Publish は監視結果を全てのクライアントに配信します。
func (h *Hub) Publish(results []monitor.Result) error {
	for _, result := range results {
		b, err := json.Marshal(monitorResultPayload{
			MonitorTargetID: result.Target.ID,
			CheckedAt:       result.CheckedAt,
			IsSuccess:       result.IsSuccess,
			StatusCode:      result.StatusCode,
			ResponseTimeMs:  result.ResponseTimeMs,
			ErrorMessage:    result.ErrorMessage,
		})
		if err != nil {
			return fmt.Errorf("failed to convert to JSON: %w", err)
		}

		h.broadcast(event{
			Event: "monitoring completed",
			Data:  string(b),
		})
	}

	return nil
}

// broadcast は Hub に登録されている全てのクライアントにメッセージを送信します。
func (h *Hub) broadcast(msg event) {
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

		ch := h.subscribe()
		defer h.unSubscribe(ch)

		e := event{
			Event: "connected to server",
			Data:  "no data",
		}
		writeData(w, e, flusher)

		for {
			select {
			case <-r.Context().Done():
				return
			case e := <-ch:
				writeData(w, e, flusher)
			}
		}
	})
}

// writeData は指定された io.Writer に対して SSE 形式で Event を書き込みます。
func writeData(w io.Writer, e event, f http.Flusher) {
	// SSE形式でデータを書き込む。SSEでは改行が重要なため、eの各フィールドの文字列については末尾の改行を削除しておく。
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", strings.TrimRight(e.Event, "\n"), strings.TrimRight(e.Data, "\n"))
	f.Flush()
}

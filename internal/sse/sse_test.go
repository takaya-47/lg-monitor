package sse_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takaya-47/lg-monitor/internal/monitor"
	"github.com/takaya-47/lg-monitor/internal/sse"
)

func TestNewHub(t *testing.T) {
	if got := sse.NewHub(); got == nil {
		t.Errorf("got %v, want non-nil", got)
	}
}

func TestNewSSEHandlerSendConnectedEvent(t *testing.T) {
	// Arrange
	// キャンセル可能なコンテキストを用意し、cancel()を呼ぶことでこのコンテキストのDoneチャネルを閉じておく。
	// これによりNewSSEHandler内部のselectにおいて、確実にctx.Done()が選択されるため、NewSSEHandlerを終了することができる（テストがブロックしない）。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/sse", nil)
	rec := httptest.NewRecorder()
	h := sse.NewHub()

	// Act
	h.NewSSEHandler().ServeHTTP(rec, req)

	// Assert
	want := "event: connected to server\ndata: no data\n\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

type ResponseWriterNotSupportFlusher struct {
	http.ResponseWriter
}

func TestNewSSEHandlerFlusherNotSupported(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/sse", nil)
	rec := httptest.NewRecorder()
	h := sse.NewHub()

	// Act
	h.NewSSEHandler().ServeHTTP(ResponseWriterNotSupportFlusher{ResponseWriter: rec}, req)

	// Assert
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestNewSSEHandlerSendCompletedEvent(t *testing.T) {
	// Arrange
	h := sse.NewHub()
	s := httptest.NewServer(h.NewSSEHandler())
	defer s.Close()

	// テスト終了後、テスト対象のハンドラも終了させるためにキャンセル可能なコンテキストを作り、deferでキャンセルする
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		t.Fatalf("fail to create request: %v", err)
	}
	// リクエストを送信。エラーがなければハンドラのfor-selectループでポーズする。
	res, err := s.Client().Do(req)
	if err != nil {
		t.Fatalf("fail to send request: %v", err)
	}
	defer res.Body.Close()

	statusOK := http.StatusOK
	responseTimeMs := 100
	results := []monitor.Result{
		{
			Target: monitor.Target{
				ID:  1,
				URL: "http://example.com",
			},
			CheckedAt:      time.Now(),
			IsSuccess:      true,
			StatusCode:     &statusOK,
			ResponseTimeMs: &responseTimeMs,
			ErrorMessage:   "",
		},
	}
	// ハブに監視結果を送信し、各クライアントにブロードキャストさせる
	h.Publish(results)

	// Assert
	if res.StatusCode != http.StatusOK {
		t.Errorf("got %d, want %d", res.StatusCode, http.StatusOK)
	}

	// SSEで送信されるメッセージを検証する
	scanner := bufio.NewScanner(res.Body)
	// 最初のSSEメッセージ（event: connected to server）を読み飛ばし、次のメッセージ（event: monitoring completed）を検証する
	readOneSSEMessage(t, scanner)
	completedMsg := readOneSSEMessage(t, scanner)
	data, err := json.Marshal(
		struct {
			MonitorTargetID int       `json:"monitor_target_id"`
			CheckedAt       time.Time `json:"checked_at"`
			IsSuccess       bool      `json:"is_success"`
			StatusCode      *int      `json:"status_code"`
			ResponseTimeMs  *int      `json:"response_time_ms"`
			ErrorMessage    string    `json:"error_message"`
		}{
			MonitorTargetID: results[0].Target.ID,
			CheckedAt:       results[0].CheckedAt,
			IsSuccess:       results[0].IsSuccess,
			StatusCode:      results[0].StatusCode,
			ResponseTimeMs:  results[0].ResponseTimeMs,
			ErrorMessage:    results[0].ErrorMessage,
		},
	)
	if err != nil {
		t.Fatalf("fail to marshal results: %v", err)
	}
	want := "event: monitoring completed\n" + "data: " + string(data) + "\n" + "\n"
	if completedMsg != want {
		t.Errorf("got %q, want %q", completedMsg, want)
	}

}

func readOneSSEMessage(t *testing.T, scanner *bufio.Scanner) string {
	t.Helper()

	msg := ""
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		msg += line + "\n"
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("error reading SSE response: %v", err)
	}
	return msg + "\n"
}

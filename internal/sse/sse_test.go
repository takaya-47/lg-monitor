package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takaya-47/lg-monitor/internal/sse"
)

func TestNewHub(t *testing.T) {
	if got := sse.NewHub(); got == nil {
		t.Errorf("got %v, want non-nil", got)
	}
}

func TestNewSSEHandler(t *testing.T) {
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

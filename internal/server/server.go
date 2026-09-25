package server

import (
	"context"
	"net"
	"net/http"
	"time"
)

// newServer はHTTPサーバーを生成して返却します。
func NewServer(ctx context.Context, port string, sseHandler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(sseHandler),
		ReadHeaderTimeout: 30 * time.Second,
		// 以下、SSEでの通信中に接続が切られないようにするため0に設定
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  0,
		// このサーバーへのリクエストが持つベースコンテキストを指定
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
}

// newRouter はHTTPルーターを生成して返却します。
func newRouter(sseHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /", rootHandler())
	mux.Handle("GET /sse", sseHandler)

	return mux
}

// rootHandler はルートパスにアクセスされた際のHTTPハンドラーを返却します。
func rootHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ルートとなるHTMLを返す
		http.ServeFile(w, r, "./index.html")
	})
}

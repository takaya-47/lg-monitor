package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/takaya-47/lg-monitor/sse"
)

func main() {
	if err := run(); err != nil {
		slog.LogAttrs(context.Background(), slog.LevelError, "failed to run", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// run はアプリケーションを開始します。
func run() error {
	// 手動でCtrl+Cまたはコンテナ終了（SIGTERM）のシグナルが送られた場合にコンテキストをキャンセルする
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := connectDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	err = monitor(ctx, db)
	if err != nil {
		return err
	}
	return nil
}

// connectDBはMySQLへの疎通を確認し、接続可能な場合はコネクションプールを返します。
func connectDB(ctx context.Context) (*sql.DB, error) {
	// DSNの検証
	db, err := sql.Open("mysql", os.Getenv("DB_DSN"))
	if err != nil {
		return nil, fmt.Errorf("dsn is invalid: %w", err)
	}

	db.SetConnMaxLifetime(3 * time.Minute) // MySQLサーバへの接続の寿命。経過後は接続を最初からやり直す。
	db.SetMaxOpenConns(10)                 // コネクションプールに対して同時に開くことができる最大接続数
	db.SetMaxIdleConns(10)                 // コネクションプールがアイドル状態で保持する接続の最大数

	// 接続チェック
	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("error when connecting to database: %w", err)
	}
	return db, nil
}

// monitorは指定された間隔で監視を実行します。
func monitor(ctx context.Context, db *sql.DB) error {
	slog.LogAttrs(ctx, slog.LevelInfo, "monitoring started", slog.String("interval_minutes", os.Getenv("MONITOR_INTERVAL_MINUTES")))

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	intervalMinutes, err := strconv.Atoi(os.Getenv("MONITOR_INTERVAL_MINUTES"))
	if err != nil {
		return fmt.Errorf("invalid env value: MONITOR_INTERVAL_MINUTES: %w", err)
	}
	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	hub := sse.NewHub()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// ルートHTMLを返すのみ
		http.ServeFile(w, r, "./index.html")
	})
	mux.Handle("GET /sse", hub.NewSSEHandler())
	s := http.Server{
		Addr:              ":" + os.Getenv("SERVER_PORT"),
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
		// 以下、SSEでの通信中に接続が切られないようにするため0に設定
		ReadTimeout:  0,
		WriteTimeout: 0,
		IdleTimeout:  0,
	}
	// ListenAndServeはサーバー停止まで返らないため、別ゴルーチンで起動して監視ループに進めるようにする
	serverErr := make(chan error, 1)
	go func() {
		err := s.ListenAndServe()
		// ErrServerClosedはこちらのShutdown由来なので異常ではない
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("failed to start sse server: %w", err)
		}
	}()

	for {
		select {
		case err := <-serverErr:
			// SSEサーバーの起動に失敗した場合
			return err
		case <-ctx.Done():
			// アプリケーションがシグナルにより終了する場合
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring stopped", slog.String("reason", ctx.Err().Error()))
			// TODO: グレースフルシャットダウン
			_ = s.Shutdown(context.Background())

			return nil
		case <-ticker.C:
			checkTargets(ctx, &client, db, hub)
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring was completed")
		}
	}
}

// checkTargetsは1回分の監視を実行します。
func checkTargets(ctx context.Context, client *http.Client, db *sql.DB, hub *sse.Hub) {
	targets, err := fetchMonitorTargets(ctx, db)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "cannot fetch monitor targets, skipping this cycle", slog.String("error", err.Error()))
		return
	}

	if len(targets) == 0 {
		slog.LogAttrs(ctx, slog.LevelInfo, "nothing to monitor, skipping this cycle")
		return
	}

	// 監視対象1件の結果を格納するバッファ付きチャネル。
	// バッファ付きチャネルを作成することで、複数のゴルーチンが結果を送信する際にブロックされるのを防げる。
	ch := make(chan monitorResult, len(targets))
	// ここでfan-outして各監視対象に対して並行処理でHTTPリクエストを送信
	for _, target := range targets {
		go func(target monitorTarget) {
			ch <- check(ctx, client, target)
		}(target)
	}

	// 長さ0、容量が監視対象数となるスライス（メモリの追加割り当てによるパフォーマンス劣化を防止）
	results := make([]monitorResult, 0, len(targets))
	// ここでfan-inしてリクエスト結果を集約
	for i := 0; i < len(targets); i++ {
		results = append(results, <-ch)
	}

	err = saveMonitorResults(ctx, db, results)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "cannot save monitor results, skipping this cycle", slog.String("error", err.Error()))
		return
	}

	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, result := range results {
		err := enc.Encode(newMonitorResultPayload(result))
		if err != nil {
			slog.LogAttrs(ctx, slog.LevelError, "cannot encode monitor result to JSON", slog.String("error", err.Error()))
			continue
		}

		hub.Publish(b.String())
		b.Reset()
	}
}

type monitorTarget struct {
	id  int
	url string
}

// fetchMonitorTargetsは監視対象のURLを取得します。
func fetchMonitorTargets(ctx context.Context, db *sql.DB) ([]monitorTarget, error) {
	const query string = `
		SELECT id, url
	      FROM monitor_targets
		 WHERE is_active = 1
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error when fetching monitor targets: %w", err)
	}
	defer rows.Close()

	var targets []monitorTarget
	for rows.Next() {
		var target monitorTarget
		err := rows.Scan(&target.id, &target.url)
		if err != nil {
			return nil, fmt.Errorf("error when scanning record: %w", err)
		}
		targets = append(targets, target)
	}

	// forループで行読み取り中に発生したエラーが存在すれば、ここでチェックする。
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error when iterating rows: %w", err)
	}

	return targets, nil
}

type monitorResult struct {
	monitorTargetID int
	checkedAt       time.Time
	isSuccess       bool
	statusCode      sql.Null[int]
	responseTimeMs  sql.Null[int]
	errorMessage    string
}

// checkは監視対象にHTTPリクエストを送信し、結果を返却します
func check(ctx context.Context, client *http.Client, target monitorTarget) monitorResult {
	result := monitorResult{
		monitorTargetID: target.id,
		checkedAt:       time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.url, nil)
	if err != nil {
		result.errorMessage = fmt.Errorf("error when creating request: %v", err).Error()
		return result
	}

	res, err := client.Do(req)
	if err != nil {
		result.errorMessage = fmt.Errorf("error when sending request: %v", err).Error()
		return result
	}
	defer res.Body.Close()

	result.statusCode = sql.Null[int]{V: res.StatusCode, Valid: true}
	result.responseTimeMs = sql.Null[int]{V: int(time.Since(result.checkedAt).Milliseconds()), Valid: true}

	if res.StatusCode != http.StatusOK {
		result.errorMessage = fmt.Errorf("status code is not 2xx: %v", res.Status).Error()
		return result
	}

	result.isSuccess = true
	return result
}

// saveMonitorResultsは監視結果をDBに保存します。
func saveMonitorResults(ctx context.Context, db *sql.DB, results []monitorResult) error {
	if len(results) == 0 {
		return nil
	}

	columns := 6
	placeholders := make([]string, 0, len(results))
	values := make([]any, 0, len(results)*columns)
	for _, r := range results {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
		values = append(values, r.monitorTargetID, r.checkedAt, r.isSuccess, r.statusCode, r.responseTimeMs, r.errorMessage)
	}
	query := fmt.Sprintf(
		"INSERT INTO monitor_results (monitor_target_id, checked_at, is_success, status_code, response_time_ms, error_message) VALUES %s",
		strings.Join(placeholders, ","),
	)

	_, err := db.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("error when executing insert: %w", err)
	}

	return nil
}

type monitorResultPayload struct {
	MonitorTargetID int       `json:"monitor_target_id"`
	CheckedAt       time.Time `json:"checked_at"`
	IsSuccess       bool      `json:"is_success"`
	StatusCode      *int      `json:"status_code"`      // nullableカラムのため構造体のフィールドをポインタ型にしてnilを許容
	ResponseTimeMs  *int      `json:"response_time_ms"` // nullableカラムのため構造体のフィールドをポインタ型にしてnilを許容
	ErrorMessage    string    `json:"error_message"`
}

func newMonitorResultPayload(result monitorResult) monitorResultPayload {
	p := monitorResultPayload{
		MonitorTargetID: result.monitorTargetID,
		CheckedAt:       result.checkedAt,
		IsSuccess:       result.isSuccess,
		ErrorMessage:    result.errorMessage,
	}

	if result.statusCode.Valid {
		p.StatusCode = &result.statusCode.V
	}
	if result.responseTimeMs.Valid {
		p.ResponseTimeMs = &result.responseTimeMs.V
	}

	return p
}

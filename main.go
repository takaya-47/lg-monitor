package main

import (
	"context"
	"database/sql"
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
	"github.com/takaya-47/lg-monitor/internal/monitor"
	"github.com/takaya-47/lg-monitor/internal/rdb"
	"github.com/takaya-47/lg-monitor/internal/server"
	"github.com/takaya-47/lg-monitor/internal/sse"
)

func main() {
	if err := run(); err != nil {
		slog.LogAttrs(context.Background(), slog.LevelError, "fatal error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// run はアプリケーションを開始します。
func run() error {
	// 手動でCtrl+Cまたはコンテナ終了（SIGTERM）のシグナルが送られた場合にコンテキストをキャンセルする
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := configValue()
	if err != nil {
		return err
	}

	db, err := rdb.Connect(ctx, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	err = execMonitor(ctx, db, cfg)
	if err != nil {
		return err
	}
	return nil
}

type config struct {
	DBDSN                  string
	monitorIntervalMinutes int
	serverPort             string
}

// configValue はアプリケーションの設定値を返却します。
func configValue() (config, error) {
	monitorIntervalMinutes, err := intervalMinutesForMonitoring()
	if err != nil {
		return config{}, err
	}

	return config{
		DBDSN:                  os.Getenv("DB_DSN"),
		monitorIntervalMinutes: monitorIntervalMinutes,
		serverPort:             os.Getenv("SERVER_PORT"),
	}, nil
}

// intervalMinutesForMonitoring は監視間隔（分）を返却します。
func intervalMinutesForMonitoring() (int, error) {
	v, err := strconv.Atoi(os.Getenv("MONITOR_INTERVAL_MINUTES"))
	if err != nil {
		return 0, fmt.Errorf("failed to get monitor interval minutes: %w", err)
	}
	return v, nil
}

// execMonitor は指定された間隔で監視を実行します。
func execMonitor(ctx context.Context, db *sql.DB, cfg config) error {
	slog.LogAttrs(ctx, slog.LevelInfo, "monitoring started", slog.Int("interval_minutes", cfg.monitorIntervalMinutes))

	hub := sse.NewHub()
	s := server.NewServer(ctx, cfg.serverPort, hub.NewSSEHandler())
	serverErr := make(chan error, 1)
	// ListenAndServeはサーバー停止までその行でポーズしてしまうため、ゴルーチンで起動して後続処理へ進めるようにする
	go func() {
		err := s.ListenAndServe()
		// s.Shutdownによる終了時はErrServerClosedが返るので異常ではない。ここでエラー扱いとすべきはサーバー起動時のエラーのみ。
		if !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("failed to start http server: %w", err)
		}
	}()

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	ticker := time.NewTicker(time.Duration(cfg.monitorIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case err := <-serverErr:
			return err
		case <-ctx.Done():
			// アプリケーションがシグナルにより終了する場合
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring stopped", slog.String("reason", ctx.Err().Error()))

			// SSEサーバーのグレースフルシャットダウンを開始する
			slog.LogAttrs(ctx, slog.LevelInfo, "starting graceful shutdown of http server")
			// コンテナ停止までの猶予期間までにシャットダウンが完了しないと、サーバーを強制終了しつつコンテナも強制終了してしまい、この場合のロギングができない。
			// よってシャットダウンに期限を設け、期限内に終了しない場合にエラーを受け取りつつ、コンテナの強制終了までにアプリケーションの片付けやロギングができるようにしている。
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// タイムアウトエラーもしくはその他のシャットダウンエラーを受け取る
			err := s.Shutdown(timeoutCtx)
			if err != nil {
				return fmt.Errorf("failed to shutdown http server gracefully: %w", err)
			}

			slog.LogAttrs(ctx, slog.LevelInfo, "http server shutdown gracefully")
			return nil
		case <-ticker.C:
			targets, err := rdb.FetchMonitorTargets(ctx, db)
			if err != nil {
				slog.LogAttrs(
					ctx,
					slog.LevelWarn,
					"failed to fetch monitor targets, skipping this cycle",
					slog.String("error", err.Error()),
				)
				continue
			}
			if len(targets) == 0 {
				slog.LogAttrs(ctx, slog.LevelInfo, "no monitor targets found, skipping this cycle")
				continue
			}

			results := monitor.CheckTargets(ctx, &client, targets)

			// TODO: 監視結果をstorageパッケージを使って保存する。
			// TODO: sseパッケージを使って監視結果をクライアントに配信する
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring was completed")
		}
	}
}

// // checkTargets は1回分の監視を実行します。
// func checkTargets(ctx context.Context, client *http.Client, db *sql.DB, hub *sse.Hub) {
// 	targets, err := fetchMonitorTargets(ctx, db)
// 	if err != nil {
// 		slog.LogAttrs(ctx, slog.LevelError, "cannot fetch monitor targets, skipping this cycle", slog.String("error", err.Error()))
// 		return
// 	}

// 	if len(targets) == 0 {
// 		slog.LogAttrs(ctx, slog.LevelInfo, "nothing to monitor, skipping this cycle")
// 		return
// 	}

// 	// 監視対象1件の結果を格納するバッファ付きチャネル。
// 	// バッファ付きチャネルを作成することで、複数のゴルーチンが結果を送信する際にブロックされるのを防げる。
// 	ch := make(chan monitorResult, len(targets))
// 	// ここでfan-outして各監視対象に対して並行処理でHTTPリクエストを送信
// 	for _, target := range targets {
// 		go func(target monitorTarget) {
// 			ch <- check(ctx, client, target)
// 		}(target)
// 	}

// 	// 長さ0、容量が監視対象数となるスライス（メモリの追加割り当てによるパフォーマンス劣化を防止）
// 	results := make([]monitorResult, 0, len(targets))
// 	// ここでfan-inしてリクエスト結果を集約
// 	for i := 0; i < len(targets); i++ {
// 		results = append(results, <-ch)
// 	}

// 	err = saveMonitorResults(ctx, db, results)
// 	if err != nil {
// 		slog.LogAttrs(ctx, slog.LevelError, "cannot save monitor results, skipping this cycle", slog.String("error", err.Error()))
// 		return
// 	}

// 	var b bytes.Buffer
// 	enc := json.NewEncoder(&b)
// 	for _, result := range results {
// 		err := enc.Encode(newMonitorResultPayload(result))
// 		if err != nil {
// 			slog.LogAttrs(ctx, slog.LevelError, "cannot encode monitor result to JSON", slog.String("error", err.Error()))
// 			continue
// 		}

// 		hub.Publish(sse.Event{
// 			Event: "monitoring completed",
// 			Data:  b.String(),
// 		})
// 		b.Reset()
// 	}
// }

type monitorResult struct {
	monitorTargetID int
	checkedAt       time.Time
	isSuccess       bool
	statusCode      sql.Null[int]
	responseTimeMs  sql.Null[int]
	errorMessage    string
}

// saveMonitorResults は監視結果をDBに保存します。
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
		return fmt.Errorf("failed to execute insert: %w", err)
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

// newMonitorResultPayload は monitorResultPayload を返却します。
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

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
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

	for {
		select {
		case <-ctx.Done():
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring stopped", slog.String("reason", ctx.Err().Error()))
			return nil
		case <-ticker.C:
			checkTargets(ctx, &client, db)
		}
	}
}

// checkTargetsは1回分の監視を実行します。
func checkTargets(ctx context.Context, client *http.Client, db *sql.DB) {
	targets, err := fetchMonitorTargets(ctx, db)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "cannot fetch monitor targets, skipping this cycle", slog.String("error", err.Error()))
		return
	}

	// 監視対象1件の結果を格納するバッファ付きチャネル。
	// バッファ付きチャネルを作成することで、複数のゴルーチンが結果を送信する際にブロックされるのを防げる。
	// ここでfan-outしている
	ch := make(chan monitorResult, len(targets))
	for _, target := range targets {
		go func(target monitorTarget) {
			ch <- check(ctx, client, target)
		}(target)
	}

	// 長さ0、容量が監視対象数となるスライス（メモリの追加割り当てによるパフォーマンス劣化を防止）
	results := make([]monitorResult, 0, len(targets))
	// ここでfan-inして結果を集約
	for i := 0; i < len(targets); i++ {
		results = append(results, <-ch)
	}

	// TODO: 集約した結果をDBに保存する処理を実装する。
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

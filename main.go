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
	"sync"
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

// run はアプリケーションを開始する
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

// connectDBはMySQLへの疎通を確認し、接続可能な場合はコネクションプールを返す
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

func monitor(ctx context.Context, db *sql.DB) error {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	// 15分ごとに監視を実行
	ticker := time.NewTicker(900 * time.Second)
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

// checkTargetsは1回分の監視を実行する。
func checkTargets(ctx context.Context, client *http.Client, db *sql.DB) {
	// TODO: 監視結果をDBに保存する
	// まず、リクエスト先のURLをDBから取得する
	// 監視結果をmonitor_resultsテーブルに保存する
	urls := []string{
		"https://www.city.uki.kumamoto.jp/",
		"https://www.town.hikawa.kumamoto.jp/",
		"https://www.city.yatsushiro.lg.jp/default.html",
		"https://www.city.kumamoto.jp/",
		"https://www.town.kumamoto-misato.lg.jp/index.html",
		"https://www.town.kumamoto-misato.lg.jp/index.orig.html",
	}

	var wg sync.WaitGroup
	wg.Add(len(urls))

	for _, url := range urls {
		go func(url string) {
			defer wg.Done()

			err := check(ctx, client, url)
			if err != nil {
				// エラーがキャンセルの場合はリクエスト自体はキャンセルされているが、正常終了としてログに残す
				if errors.Is(err, context.Canceled) {
					slog.LogAttrs(ctx, slog.LevelInfo, "request canceled", slog.String("url", url))
					return
				}

				// キャンセル以外のエラーは異常終了としてログに残す
				slog.LogAttrs(ctx, slog.LevelError, "request failed", slog.String("url", url), slog.String("error", err.Error()))
			}
		}(url)
	}

	wg.Wait()
}

func check(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("error when creating request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error when sending request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status code is not OK: %d", res.StatusCode)
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "got successful response", slog.String("url", url), slog.Int("status_code", res.StatusCode))
	return nil
}

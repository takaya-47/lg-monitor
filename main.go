package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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

	store, err := rdb.NewStore(ctx, cfg.DBDSN)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.LogAttrs(ctx, slog.LevelWarn, "failed to close database", slog.String("error", err.Error()))
		}
	}()

	err = exec(ctx, store, cfg)
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

// exec はアプリケーションを実行します。
func exec(ctx context.Context, store *rdb.Store, cfg config) error {
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

	targetRepository, err := rdb.NewStore(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("failed to create target repository: %w", err)
	}
	resultRepository, err := rdb.NewStore(ctx, cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("failed to create result repository: %w", err)
	}
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
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring started")

			err := monitor.Monitor(ctx, &client, targetRepository, resultRepository, hub)
			if err != nil {
				slog.LogAttrs(
					ctx,
					slog.LevelWarn,
					"monitoring failed",
					slog.String("error", err.Error()),
				)
				continue
			}

			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring succeeded")
		}
	}
}

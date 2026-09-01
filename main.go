package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := http.Client{
		Timeout: 10 * time.Second,
	}
	urls := []string{
		"https://www.city.uki.kumamoto.jp/",
		"https://www.town.hikawa.kumamoto.jp/",
		"https://www.city.yatsushiro.lg.jp/default.html",
		"https://www.city.kumamoto.jp/",
		"https://www.town.kumamoto-misato.lg.jp/index.html",
		"https://www.town.kumamoto-misato.lg.jp/index.orig.html",
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.LogAttrs(ctx, slog.LevelInfo, "monitoring stopped", slog.String("reason", ctx.Err().Error()))
			return
		case <-ticker.C:
			var wg sync.WaitGroup
			wg.Add(len(urls))

			for _, url := range urls {
				go func(url string) {
					defer wg.Done()

					err := check(ctx, client, url)
					if err != nil {
						slog.LogAttrs(ctx, slog.LevelError, "request failed", slog.String("url", url), slog.String("error", err.Error()))
					}
				}(url)
			}

			wg.Wait()
		}
	}
}

func check(ctx context.Context, client http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.New("error when creating request: " + err.Error())
	}

	res, err := client.Do(req)
	if err != nil {
		return errors.New("error when sending request: " + err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status code is not OK: %d", res.StatusCode)
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "got successful response", slog.String("url", url), slog.Int("status_code", res.StatusCode))
	return nil
}

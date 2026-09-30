package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Target struct {
	ID  int
	URL string
}

type Result struct {
	Target         Target
	CheckedAt      time.Time
	IsSuccess      bool
	StatusCode     *int // レスポンスを受け取れない場合があるためポインタ型にしてnilを許容
	ResponseTimeMs *int // レスポンスを受け取れない場合があるためポインタ型にしてnilを許容
	ErrorMessage   string
}

// CheckTargets は監視対象全てに対して監視を実行します。
func CheckTargets(ctx context.Context, client *http.Client, targets []Target) []Result {
	// 監視対象1件の結果を格納するバッファ付きチャネル。
	// バッファ付きチャネルを作成することで、複数のゴルーチンが結果を送信する際にブロックされるのを防げる。
	ch := make(chan Result, len(targets))
	// ここでfan-outして各監視対象に対して並行処理でHTTPリクエストを送信
	for _, target := range targets {
		go func(target Target) {
			ch <- check(ctx, client, target)
		}(target)
	}

	// 長さ0、容量が監視対象数となるスライス（メモリの追加割り当てによるパフォーマンス劣化を防止）
	results := make([]Result, 0, len(targets))
	// ここでfan-inしてリクエスト結果を集約
	for i := 0; i < len(targets); i++ {
		results = append(results, <-ch)
	}

	return results
}

// check は監視対象にHTTPリクエストを送信し、結果を返却します。
func check(ctx context.Context, client *http.Client, target Target) Result {
	result := Result{
		Target:    target,
		CheckedAt: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		result.ErrorMessage = fmt.Errorf("failed to create request: %v", err).Error()
		return result
	}

	res, err := client.Do(req)
	if err != nil {
		result.ErrorMessage = fmt.Errorf("failed to send request: %v", err).Error()
		return result
	}
	defer res.Body.Close()

	result.StatusCode = &res.StatusCode
	ms := int(time.Since(result.CheckedAt).Milliseconds())
	result.ResponseTimeMs = &ms

	if res.StatusCode != http.StatusOK {
		result.ErrorMessage = fmt.Errorf("status code is not 2xx: %v", res.Status).Error()
		return result
	}

	result.IsSuccess = true
	return result
}

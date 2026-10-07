package rdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/takaya-47/lg-monitor/internal/monitor"
)

// Save は監視結果をDBに保存します。
func (s *Store) Save(ctx context.Context, results []monitor.Result) error {
	if len(results) == 0 {
		return nil
	}

	columns := 6
	placeholders := make([]string, 0, len(results))
	values := make([]any, 0, len(results)*columns)
	for _, r := range results {
		placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?)")
		values = append(values, r.Target.ID, r.CheckedAt, r.IsSuccess, r.StatusCode, r.ResponseTimeMs, r.ErrorMessage)
	}
	query := fmt.Sprintf(
		"INSERT INTO monitor_results (monitor_target_id, checked_at, is_success, status_code, response_time_ms, error_message) VALUES %s",
		strings.Join(placeholders, ","),
	)

	_, err := s.db.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to execute insert: %w", err)
	}

	return nil
}

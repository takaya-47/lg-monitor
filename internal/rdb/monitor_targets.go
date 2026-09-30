package rdb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/takaya-47/lg-monitor/internal/monitor"
)

// FetchMonitorTargets は監視対象を取得します。
func FetchMonitorTargets(ctx context.Context, db *sql.DB) ([]monitor.Target, error) {
	const query string = `
		SELECT id, url
	      FROM monitor_targets
		 WHERE is_active = 1
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch monitor targets: %w", err)
	}
	defer rows.Close()

	var targets []monitor.Target
	for rows.Next() {
		var target monitor.Target
		err := rows.Scan(&target.ID, &target.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		targets = append(targets, target)
	}

	// rows.Nextがfalseを返すとforループを抜けるが、falseを返した理由が全行を正常に読み終えたからなのか、途中でエラーが発生したからなのかを確認するのに必須。
	// つまり、rows.Err()がnilを返せば全行を正常に読み終えたということ。nilでなければ読み取り途中でエラーが発生したということ。
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return targets, nil
}

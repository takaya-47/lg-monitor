package monitor_test

import (
	"testing"
	"time"

	"github.com/takaya-47/lg-monitor/internal/monitor"
)

func TestNewResult(t *testing.T) {
	expectedCheckedAt := time.Now()
	expectedTargetID := 1
	r := monitor.NewResult(
		monitor.Target{
			ID:  expectedTargetID,
			URL: "http://example.com",
		},
		expectedCheckedAt,
	)

	if r.TargetID != expectedTargetID {
		t.Errorf("invalid target id, got %d, want %d", r.TargetID, expectedTargetID)
	}

	if !r.CheckedAt.Equal(expectedCheckedAt) {
		t.Errorf("invalid checked at, got %v, want %v", r.CheckedAt, expectedCheckedAt)
	}
}

package sse_test

import (
	"testing"

	"github.com/takaya-47/lg-monitor/internal/sse"
)

func TestNewHub(t *testing.T) {
	if got := sse.NewHub(); got == nil {
		t.Errorf("got %v, want non-nil", got)
	}
}

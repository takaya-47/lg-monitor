package monitor

import (
	"time"
)

type Result struct {
	TargetID       int
	CheckedAt      time.Time
	IsSuccess      bool
	StatusCode     *int // レスポンスを受け取れない場合があるためポインタ型にしてnilを許容
	ResponseTimeMs *int // レスポンスを受け取れない場合があるためポインタ型にしてnilを許容
	ErrorMessage   string
}

func NewResult(target Target, currentDateTime time.Time) Result {
	return Result{
		TargetID:  target.ID,
		CheckedAt: currentDateTime,
	}
}

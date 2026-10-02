package boxwifi

import (
	"context"
	"time"
)

func waitUntil(ctx context.Context, limit time.Duration, now func() time.Time, sleep func(context.Context, time.Duration) error, isReady func() bool) bool {
	deadline := now().Add(limit)
	for {
		if isReady() {
			return true
		}
		if !now().Before(deadline) {
			return false
		}
		if errorValue := sleep(ctx, onlinePollInterval); errorValue != nil {
			return false
		}
	}
}

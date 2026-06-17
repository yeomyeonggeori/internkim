package capabilityd

import (
	"context"
	"testing"
	"time"
)

func TestMattermostRateLimiterAllowsBurstThenThrottles(t *testing.T) {
	limiter := newMattermostRateLimiter(100, 10)
	for index := 0; index < 10; index++ {
		if waitDuration := limiter.reserveToken(); waitDuration != 0 {
			t.Fatalf("burst token %d should be immediate, waited %v", index, waitDuration)
		}
	}
	waitDuration := limiter.reserveToken()
	if waitDuration <= 0 {
		t.Fatal("token beyond burst must require waiting")
	}
	if waitDuration > 12*time.Millisecond {
		t.Fatalf("wait for one refilled token at 100/s should be about 10ms, got %v", waitDuration)
	}
}

func TestMattermostRateLimiterWaitHonorsContextCancellation(t *testing.T) {
	limiter := newMattermostRateLimiter(1, 1)
	if errorValue := limiter.wait(context.Background()); errorValue != nil {
		t.Fatalf("first token should be granted immediately: %v", errorValue)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if errorValue := limiter.wait(ctx); errorValue == nil {
		t.Fatal("second token at 1/s should block past the 10ms deadline and return the context error")
	}
}

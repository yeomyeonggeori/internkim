package capabilityd

import (
	"context"
	"errors"
	"testing"
)

func TestCommandLimiterHonorsContextWhileWaiting(t *testing.T) {
	limiter := newCommandLimiter(1)
	releaseFirstCommand := make(chan struct{})
	firstCommandStarted := make(chan struct{})
	firstCommandDone := make(chan struct{})

	go func() {
		defer close(firstCommandDone)
		_, _ = limiter.Run(context.Background(), func() ([]byte, error) {
			close(firstCommandStarted)
			<-releaseFirstCommand
			return []byte("ok"), nil
		})
	}()

	<-firstCommandStarted
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, errorValue := limiter.Run(ctx, func() ([]byte, error) {
		t.Fatal("second command should not run after context cancellation")
		return nil, nil
	})
	close(releaseFirstCommand)
	<-firstCommandDone

	if !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", errorValue)
	}
}

package capabilityd

import "context"

const defaultHeavyCommandConcurrency = 2

type commandLimiter struct {
	slots chan struct{}
}

var defaultCommandLimiter = newCommandLimiter(defaultHeavyCommandConcurrency)

func newCommandLimiter(concurrency int) *commandLimiter {
	if concurrency < 1 {
		concurrency = 1
	}
	return &commandLimiter{slots: make(chan struct{}, concurrency)}
}

func (limiter *commandLimiter) Run(ctx context.Context, function func() ([]byte, error)) ([]byte, error) {
	if function == nil {
		return nil, nil
	}
	select {
	case limiter.slots <- struct{}{}:
		defer func() {
			<-limiter.slots
		}()
		return function()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

package cli

import "time"

type retryOptions struct {
	AttemptCount           int
	DelayForAttempt        func(attemptIndex int) time.Duration
	ShouldRetry            func(errorValue error) bool
	SleepAfterFinalAttempt bool
}

func retryOperation(options retryOptions, operation func(attemptIndex int) error) error {
	var lastError error
	for attemptIndex := 0; attemptIndex < options.AttemptCount; attemptIndex++ {
		lastError = operation(attemptIndex)
		if lastError == nil {
			return nil
		}
		if options.ShouldRetry != nil && !options.ShouldRetry(lastError) {
			return lastError
		}
		if attemptIndex < options.AttemptCount-1 || options.SleepAfterFinalAttempt {
			if options.DelayForAttempt == nil {
				continue
			}
			delay := options.DelayForAttempt(attemptIndex)
			if delay > 0 {
				time.Sleep(delay)
			}
		}
	}
	return lastError
}

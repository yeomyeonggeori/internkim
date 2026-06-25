package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

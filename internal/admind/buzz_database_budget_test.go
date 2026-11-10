package admind

import (
	"context"
	"testing"
	"time"
)

// database/sql queues without a deadline of its own, so a caller that carries
// context.Background() into the shared pool waits for a free connection for the
// life of the process. A sign-in burst left one of these behind per sign-in.
func TestBackgroundWorkCarriesADeadlineIntoTheSharedPool(t *testing.T) {
	carried := make(chan context.Context, 1)
	inTheBackgroundWithin(buzzDatabaseRequestBudget, func(ctx context.Context) {
		carried <- ctx
	})

	select {
	case ctx := <-carried:
		deadline, hasDeadline := ctx.Deadline()
		if !hasDeadline {
			t.Fatal("background work was handed a context with no deadline, so a caller that queues for a connection queues forever")
		}
		if remaining := time.Until(deadline); remaining > buzzDatabaseRequestBudget {
			t.Fatalf("the deadline is %s away, past the %s budget", remaining, buzzDatabaseRequestBudget)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the background work never ran")
	}
}

func TestASweepCarriesADeadlineIntoTheSharedPool(t *testing.T) {
	service := NewService(Configuration{})
	carried := make(chan context.Context, 1)

	service.withinASweepBudget(context.Background(), func(ctx context.Context) {
		carried <- ctx
	})

	ctx := <-carried
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		t.Fatal("a sweep was handed a context with no deadline, so one stuck pass stops that sweep for the life of the process")
	}
	if remaining := time.Until(deadline); remaining > buzzDatabaseSweepBudget {
		t.Fatalf("the deadline is %s away, past the %s budget", remaining, buzzDatabaseSweepBudget)
	}
}

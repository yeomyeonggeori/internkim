package offsite

import "context"

const fetchesAtOnce = 16

type fetchOutcome[Fetched any] struct {
	fetched    Fetched
	errorValue error
}

func FetchAhead[Name any, Fetched any](ctx context.Context, names []Name, fetch func(context.Context, Name) (Fetched, error), use func(Fetched) error) error {
	fetching, stop := context.WithCancel(ctx)
	defer stop()
	outcomes := make([]chan fetchOutcome[Fetched], len(names))
	for index := range outcomes {
		outcomes[index] = make(chan fetchOutcome[Fetched], 1)
	}
	slots := make(chan struct{}, fetchesAtOnce)
	go func() {
		for index, name := range names {
			select {
			case slots <- struct{}{}:
			case <-fetching.Done():
				return
			}
			go func() {
				fetched, errorValue := fetch(fetching, name)
				outcomes[index] <- fetchOutcome[Fetched]{fetched: fetched, errorValue: errorValue}
			}()
		}
	}()
	for index := range names {
		select {
		case outcome := <-outcomes[index]:
			if outcome.errorValue != nil {
				return outcome.errorValue
			}
			if errorValue := use(outcome.fetched); errorValue != nil {
				return errorValue
			}
			<-slots
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

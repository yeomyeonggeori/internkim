package offsite

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func numbered(count int) []int {
	names := make([]int, count)
	for index := range names {
		names[index] = index
	}
	return names
}

func TestWhatIsFetchedAheadIsUsedInTheOrderItWasNamed(t *testing.T) {
	names := numbered(500)
	used := []int{}

	errorValue := FetchAhead(context.Background(), names, func(_ context.Context, name int) (int, error) {
		time.Sleep(time.Duration(name%7) * time.Millisecond)
		return name, nil
	}, func(fetched int) error {
		used = append(used, fetched)
		return nil
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(used, names) {
		t.Fatalf("used %d fetches out of the order they were named in", len(used))
	}
}

func TestFetchingAheadTakesSeveralAtOnceAndNeverMoreThanItsLimit(t *testing.T) {
	var underway, most atomic.Int32
	started := time.Now()

	errorValue := FetchAhead(context.Background(), numbered(160), func(context.Context, int) (int, error) {
		now := underway.Add(1)
		for seen := most.Load(); now > seen && !most.CompareAndSwap(seen, now); seen = most.Load() {
		}
		time.Sleep(20 * time.Millisecond)
		underway.Add(-1)
		return 0, nil
	}, func(int) error { return nil })

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if most.Load() > fetchesAtOnce {
		t.Fatalf("%d fetches ran at once, past the limit of %d", most.Load(), fetchesAtOnce)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("160 fetches of 20ms took %s, as long as fetching them one after another", elapsed)
	}
}

func TestAFetchThatFailsStopsTheRestAndNothingAfterItIsUsed(t *testing.T) {
	refused := errors.New("the store refused the read")
	used := []int{}

	errorValue := FetchAhead(context.Background(), numbered(100), func(_ context.Context, name int) (int, error) {
		if name == 40 {
			return 0, refused
		}
		return name, nil
	}, func(fetched int) error {
		used = append(used, fetched)
		return nil
	})

	if !errors.Is(errorValue, refused) {
		t.Fatalf("answered %v", errorValue)
	}
	if !slices.Equal(used, numbered(40)) {
		t.Fatalf("used %v around a fetch that failed", used)
	}
}

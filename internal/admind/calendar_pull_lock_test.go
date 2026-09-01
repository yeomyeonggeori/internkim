package admind

import (
	"testing"
	"time"
)

func TestCalendarPullLockReleasesAndPropagatesPanic(t *testing.T) {
	service := newCalendarTestService(t)
	panicValue := "calendar pull panic"
	recoveredValue := func() (recovered any) {
		defer func() {
			recovered = recover()
		}()
		service.runCalendarPullLocked(func() {
			panic(panicValue)
		})
		return nil
	}()
	if recoveredValue != panicValue {
		t.Fatalf("recovered panic=%v", recoveredValue)
	}
	reacquired := make(chan struct{})
	go func() {
		service.runCalendarPullLocked(func() {
			close(reacquired)
		})
	}()
	select {
	case <-reacquired:
	case <-time.After(time.Second):
		t.Fatal("calendar pull lock was not released after panic")
	}
}

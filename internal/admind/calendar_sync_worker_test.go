package admind

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunGoogleCalendarPullSkipsQueryWhenCTagUnchanged(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-known"
	updated, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}
	client := &fakeCalDAVPullClient{ctag: "ctag-known"}
	changed, errorValue := service.runGoogleCalendarPull(ctx, updated, client)
	if errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}
	if changed {
		t.Error("expected changed=false on identical ctag")
	}
	if client.ctagCalls != 1 {
		t.Errorf("ctag calls: got %d", client.ctagCalls)
	}
	if client.queryCalls != 0 {
		t.Errorf("query should be skipped on identical ctag, got %d", client.queryCalls)
	}
}

func TestRunGoogleCalendarPullPersistsCTagAfterChange(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	client := &fakeCalDAVPullClient{
		ctag: "ctag-new",
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "ctag-evt@google", `"e1"`, "After ctag change"),
		},
	}
	changed, errorValue := service.runGoogleCalendarPull(ctx, account, client)
	if errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}
	if !changed {
		t.Error("expected changed=true on ctag mismatch")
	}
	refreshed, _, _ := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if refreshed.DefaultCalendarCTag != "ctag-new" {
		t.Errorf("ctag not persisted: got %q", refreshed.DefaultCalendarCTag)
	}
}

func TestCalendarSyncSafetyIntervalIsHourly(t *testing.T) {
	if calendarSyncSafetyInterval != time.Hour {
		t.Fatalf("safety interval: got %v, want %v", calendarSyncSafetyInterval, time.Hour)
	}
}

func TestRunCalendarBackgroundSyncCycleSkipsPull(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	var pullCalls atomic.Int32
	var pushCalls atomic.Int32
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		pullCalls.Add(1)
		return false, nil
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		pushCalls.Add(1)
		return nil, nil
	}
	clock := func() time.Time { return now }
	service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, false)
	now = now.Add(30 * time.Second)
	service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, false)
	now = now.Add(31 * time.Second)
	service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, false)
	if pullCalls.Load() != 0 {
		t.Fatalf("background pull calls: got %d, want 0", pullCalls.Load())
	}
	if pushCalls.Load() != 3 {
		t.Fatalf("push calls: got %d, want 3", pushCalls.Load())
	}
}

func TestRunCalendarUserSyncCycleSkipsPullWithinCacheTTL(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	var pullCalls atomic.Int32
	var pushCalls atomic.Int32
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		pullCalls.Add(1)
		return false, nil
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		pushCalls.Add(1)
		return nil, nil
	}
	clock := func() time.Time { return now }
	firstResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	now = now.Add(30 * time.Second)
	secondResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	now = now.Add(31 * time.Second)
	thirdResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	if pullCalls.Load() != 2 {
		t.Fatalf("user pull calls: got %d, want 2", pullCalls.Load())
	}
	if !firstResult.PullAttempted || firstResult.PullSkippedByCache {
		t.Fatalf("first pull result: %#v", firstResult)
	}
	if secondResult.PullAttempted || !secondResult.PullSkippedByCache {
		t.Fatalf("second pull should be skipped by shared cache: %#v", secondResult)
	}
	if !thirdResult.PullAttempted || thirdResult.PullSkippedByCache {
		t.Fatalf("third pull result after TTL: %#v", thirdResult)
	}
	if pushCalls.Load() != 3 {
		t.Fatalf("push calls: got %d, want 3", pushCalls.Load())
	}
}

func TestRunCalendarUserSyncCyclePushesBeforePull(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	calls := []string{}
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		calls = append(calls, "pull")
		if _, ok := protectedUIDs["just-pushed@google"]; !ok {
			t.Fatalf("pull did not receive pushed UID protection: %#v", protectedUIDs)
		}
		return false, nil
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		calls = append(calls, "push")
		return map[string]struct{}{"just-pushed@google": {}}, nil
	}
	clock := func() time.Time { return now }
	service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	if len(calls) != 2 {
		t.Fatalf("calls: got %v, want push then pull", calls)
	}
	if calls[0] != "push" || calls[1] != "pull" {
		t.Fatalf("sync order: got %v, want push then pull", calls)
	}
}

func TestRunCalendarUserSyncCyclePushesAgainAfterChangedPull(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	calls := []string{}
	pushCallCount := 0
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		calls = append(calls, "pull")
		return true, nil
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		calls = append(calls, "push")
		pushCallCount++
		if pushCallCount == 2 {
			return map[string]struct{}{"exported-after-pull@google": {}}, nil
		}
		return nil, nil
	}
	clock := func() time.Time { return now }
	result := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	if !result.Succeeded() || !result.PullAttempted || !result.Changed {
		t.Fatalf("changed pull result: %#v", result)
	}
	if len(calls) != 3 || calls[0] != "push" || calls[1] != "pull" || calls[2] != "push" {
		t.Fatalf("sync order: got %v, want push pull push", calls)
	}
	protectedUIDs := service.recentlyPushedCalendarUIDs(now)
	if _, found := protectedUIDs["exported-after-pull@google"]; !found {
		t.Fatalf("second push UID was not recorded for protection: %#v", protectedUIDs)
	}
}

func TestRunCalendarUserSyncCycleSkipsSecondPushAfterPushFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	var pushCalls atomic.Int32
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		return true, nil
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		pushCalls.Add(1)
		return nil, errors.New("push unavailable")
	}
	clock := func() time.Time { return now }
	result := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	if pushCalls.Load() != 1 {
		t.Fatalf("failed first push should not be retried after pull: got %d calls", pushCalls.Load())
	}
	if !result.PushFailed || !result.PullAttempted || !result.Changed || result.Succeeded() {
		t.Fatalf("failed push result should preserve successful pull state: %#v", result)
	}
}

func TestRunCalendarUserSyncCycleRespectsPullCache(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	var pullCalls atomic.Int32
	var pushCalls atomic.Int32
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		pullCalls.Add(1)
		return true, nil
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		pushCalls.Add(1)
		return nil, nil
	}
	clock := func() time.Time { return now }
	firstResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	now = now.Add(30 * time.Second)
	secondResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	if pullCalls.Load() != 1 {
		t.Fatalf("cache-respecting pull calls: got %d, want 1", pullCalls.Load())
	}
	if !firstResult.PullAttempted || firstResult.PullSkippedByCache {
		t.Fatalf("first pull result: %#v", firstResult)
	}
	if secondResult.PullAttempted || !secondResult.PullSkippedByCache {
		t.Fatalf("second pull should be skipped by shared cache: %#v", secondResult)
	}
	if pushCalls.Load() != 3 {
		t.Fatalf("push calls: got %d, want 3", pushCalls.Load())
	}
}

func TestRunCalendarUserSyncCycleRetriesPullAfterFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	var pullCalls atomic.Int32
	var pushCalls atomic.Int32
	pull := func(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
		pullCalls.Add(1)
		return false, errors.New("pull unavailable")
	}
	push := func(ctx context.Context) (map[string]struct{}, error) {
		pushCalls.Add(1)
		return nil, nil
	}
	clock := func() time.Time { return now }
	firstResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	now = now.Add(30 * time.Second)
	secondResult := service.runCalendarSyncCycleWithHooks(ctx, clock, pull, push, true)
	if pullCalls.Load() != 2 {
		t.Fatalf("failed pull should not populate cache, got %d pull calls", pullCalls.Load())
	}
	if pushCalls.Load() != 2 {
		t.Fatalf("push calls: got %d, want 2", pushCalls.Load())
	}
	for _, result := range []calendarSyncCycleResult{firstResult, secondResult} {
		if !result.PullAttempted || !result.PullFailed || result.PullSkippedByCache || result.Succeeded() {
			t.Fatalf("failed pull result should be reported without cache skip: %#v", result)
		}
	}
}

func TestCalendarRemoteSyncLeaseSkipsAcrossServiceInstances(t *testing.T) {
	firstService := newCalendarTestService(t)
	secondService := NewService(firstService.Configuration)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	firstDecision, errorValue := firstService.acquireCalendarRemoteSync(ctx, now)
	if errorValue != nil {
		t.Fatalf("first acquire: %v", errorValue)
	}
	if !firstDecision.Acquired {
		t.Fatalf("first acquire decision: %#v", firstDecision)
	}
	secondDecision, errorValue := secondService.acquireCalendarRemoteSync(ctx, now.Add(time.Second))
	if errorValue != nil {
		t.Fatalf("second acquire: %v", errorValue)
	}
	if secondDecision.Acquired || !secondDecision.SkippedByLease {
		t.Fatalf("second acquire should skip active lease: %#v", secondDecision)
	}
}

func TestCalendarRemoteSyncSuccessCacheSkipsAcrossServiceInstances(t *testing.T) {
	firstService := newCalendarTestService(t)
	secondService := NewService(firstService.Configuration)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	firstDecision, errorValue := firstService.acquireCalendarRemoteSync(ctx, now)
	if errorValue != nil {
		t.Fatalf("first acquire: %v", errorValue)
	}
	if !firstDecision.Acquired {
		t.Fatalf("first acquire decision: %#v", firstDecision)
	}
	if errorValue := firstService.finishCalendarRemoteSync(ctx, firstDecision, now, true); errorValue != nil {
		t.Fatalf("finish: %v", errorValue)
	}
	secondDecision, errorValue := secondService.acquireCalendarRemoteSync(ctx, now.Add(30*time.Second))
	if errorValue != nil {
		t.Fatalf("second acquire: %v", errorValue)
	}
	if secondDecision.Acquired || !secondDecision.SkippedByCache {
		t.Fatalf("second acquire should skip recent success: %#v", secondDecision)
	}
	thirdDecision, errorValue := secondService.acquireCalendarRemoteSync(ctx, now.Add(calendarRemoteSyncSuccessCacheDuration+time.Second))
	if errorValue != nil {
		t.Fatalf("third acquire: %v", errorValue)
	}
	if !thirdDecision.Acquired {
		t.Fatalf("third acquire decision: %#v", thirdDecision)
	}
}

func TestCalendarOutboxWriteKeepsPullCache(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Unix(1000, 0).UTC()
	service.markCalendarPullCompleted(now)
	if service.shouldRunCalendarPull(now.Add(30 * time.Second)) {
		t.Fatal("fresh cache should skip pull before local write")
	}
	seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("cache-invalidate", "Cache Invalidate")
	if errorValue := service.enqueueCalendarOutboxForWrite(ctx, event, []string{calendarFieldTitle}); errorValue != nil {
		t.Fatalf("enqueue: %v", errorValue)
	}
	if service.shouldRunCalendarPull(now.Add(30 * time.Second)) {
		t.Fatal("local write should keep fresh pull cache")
	}
}

func TestRunCalendarSyncLoopUsesFixedInterval(t *testing.T) {
	service := newCalendarTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var totalCalls atomic.Int32
	cycle := func(ctx context.Context) bool {
		totalCalls.Add(1)
		return true
	}
	done := make(chan struct{})
	go func() {
		service.runCalendarSyncLoop(ctx, cycle, 30*time.Millisecond)
		close(done)
	}()
	time.Sleep(95 * time.Millisecond)
	cancel()
	<-done
	if totalCalls.Load() < 3 {
		t.Errorf("expected fixed interval cycles in 95ms, got %d", totalCalls.Load())
	}
}

func TestRunCalendarSyncLoopWakeUpResetsInterval(t *testing.T) {
	service := newCalendarTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := make(chan struct{}, 16)
	cycle := func(ctx context.Context) bool {
		select {
		case calls <- struct{}{}:
		default:
		}
		return false
	}
	done := make(chan struct{})
	go func() {
		service.runCalendarSyncLoop(ctx, cycle, 50*time.Millisecond)
		close(done)
	}()
	receiveCalendarSyncCall(t, calls, time.Second)
	receiveCalendarSyncCall(t, calls, time.Second)
	service.signalCalendarSyncWakeUp()
	receiveCalendarSyncCall(t, calls, 500*time.Millisecond)
	cancel()
	<-done
}

func TestRunCalendarSyncLoopStopsOnContextCancel(t *testing.T) {
	service := newCalendarTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cycle := func(ctx context.Context) bool { return false }
	done := make(chan struct{})
	go func() {
		service.runCalendarSyncLoop(ctx, cycle, 10*time.Second)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not exit after context cancel")
	}
}

func TestSignalCalendarSyncWakeUpIsNonBlockingAndCoalesces(t *testing.T) {
	service := newCalendarTestService(t)
	for i := 0; i < 5; i++ {
		service.signalCalendarSyncWakeUp()
	}
	select {
	case <-service.calendarSyncWakeUp:
	default:
		t.Fatal("expected at least one wake-up signal queued")
	}
	select {
	case <-service.calendarSyncWakeUp:
		t.Fatal("multiple signals should coalesce into one")
	default:
	}
}

func TestStartCalendarSyncWorkerRespectsDisabledFlag(t *testing.T) {
	service := newCalendarTestService(t)
	if !service.Configuration.CalendarSyncDisabled {
		t.Fatal("test service should default to sync disabled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.startCalendarSyncWorker(ctx)
	select {
	case <-service.calendarSyncWakeUp:
		t.Fatal("disabled worker should not consume wake-up channel")
	case <-time.After(100 * time.Millisecond):
	}
}

func receiveCalendarSyncCall(t *testing.T, calls chan struct{}, timeout time.Duration) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(timeout):
		t.Fatal("expected sync cycle invocation within timeout")
	}
}

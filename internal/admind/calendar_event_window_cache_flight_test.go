package admind

import (
	"context"
	"reflect"
	"runtime"
	"testing"
)

func TestCalendarEventWindowCacheBuildCoordinatorSharesResult(t *testing.T) {
	var coordinator calendarEventWindowCacheBuildCoordinator
	isBuilder, builderFlight := coordinator.begin("same-key")
	if !isBuilder {
		t.Fatal("first cache miss did not become builder")
	}
	const readerCount = 8
	followerFlights := make([]*calendarEventWindowCacheBuildFlight, 0, readerCount-1)
	for range readerCount - 1 {
		isFollowerBuilder, followerFlight := coordinator.begin("same-key")
		if isFollowerBuilder || followerFlight != builderFlight {
			t.Fatalf("follower builder = %v flight = %p, want %p", isFollowerBuilder, followerFlight, builderFlight)
		}
		followerFlights = append(followerFlights, followerFlight)
	}
	expectedEvents := []calendarEvent{{
		ID:           "shared-result",
		People:       []string{"Person"},
		Participants: []calendarParticipant{{PersonID: "person-1"}},
	}}
	coordinator.complete("same-key", builderFlight, calendarEventWindowCacheBuildResult{events: expectedEvents})
	for _, followerFlight := range followerFlights {
		result, errorValue := followerFlight.wait(t.Context())
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if result.shouldRetry || !reflect.DeepEqual(result.events, expectedEvents) {
			t.Fatalf("flight result = %+v", result)
		}
	}
	result, errorValue := followerFlights[0].wait(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result.events[0].People[0] = "Changed"
	result.events[0].Participants[0].PersonID = "changed"
	secondResult, errorValue := followerFlights[1].wait(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reflect.DeepEqual(secondResult.events, expectedEvents) {
		t.Fatalf("shared result was mutated = %+v", secondResult.events)
	}
}

func waitForCalendarEventWindowCacheFollowers(t testing.TB, ctx context.Context, service *Service, cacheKey string, minimumCount int) {
	t.Helper()
	for {
		service.calendarWindowBuilds.mutex.Lock()
		flight := service.calendarWindowBuilds.flightByKey[cacheKey]
		followerCount := 0
		if flight != nil {
			followerCount = flight.followerCount
		}
		service.calendarWindowBuilds.mutex.Unlock()
		if followerCount >= minimumCount {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		default:
			runtime.Gosched()
		}
	}
}

func TestCalendarEventWindowCacheBuildCoordinatorJoinsSameKey(t *testing.T) {
	var coordinator calendarEventWindowCacheBuildCoordinator
	isFirstBuilder, firstCompletion := coordinator.begin("same-key")
	isSecondBuilder, secondCompletion := coordinator.begin("same-key")
	if !isFirstBuilder || isSecondBuilder {
		t.Fatalf("builder states = %v, %v", isFirstBuilder, isSecondBuilder)
	}
	if firstCompletion != secondCompletion {
		t.Fatal("same cache key returned different completion channels")
	}
	coordinator.complete("same-key", firstCompletion, calendarEventWindowCacheBuildResult{})
	if _, errorValue := secondCompletion.wait(t.Context()); errorValue != nil {
		t.Fatal(errorValue)
	}
	isNextBuilder, nextCompletion := coordinator.begin("same-key")
	if !isNextBuilder {
		t.Fatal("completed cache key did not admit the next builder")
	}
	coordinator.complete("same-key", nextCompletion, calendarEventWindowCacheBuildResult{})
	if _, errorValue := nextCompletion.wait(t.Context()); errorValue != nil {
		t.Fatal(errorValue)
	}
}

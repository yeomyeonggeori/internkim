package admind

import (
	"context"
	"testing"
)

func TestCalendarProjectionLeaseKeepsNewGenerationUntilOwnerAdvances(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, database, "event-1"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	lease, acquired, errorValue := service.acquireCalendarProjectionLease(ctx, "event-1")
	if errorValue != nil || !acquired {
		t.Fatalf("lease acquired=%v error=%v", acquired, errorValue)
	}
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, database, "event-1"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	nextLease, hasNextGeneration, errorValue := service.completeCalendarProjectionGeneration(ctx, lease)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !hasNextGeneration || nextLease.Generation != lease.Generation+1 || nextLease.Owner != lease.Owner {
		t.Fatalf("next lease=%+v has_next=%v", nextLease, hasNextGeneration)
	}
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var generation int64
	var leaseOwner string
	if errorValue := database.QueryRowContext(ctx, `SELECT generation, lease_owner FROM calendar_channel_outbox WHERE event_id = ?`, "event-1").Scan(&generation, &leaseOwner); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	if generation != nextLease.Generation || leaseOwner != lease.Owner {
		t.Fatalf("persisted generation=%d lease_owner=%q", generation, leaseOwner)
	}
	_, hasNextGeneration, errorValue = service.completeCalendarProjectionGeneration(ctx, nextLease)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if hasNextGeneration {
		t.Fatal("unexpected generation after latest completion")
	}
	assertCalendarProjectionOutboxCount(t, service, 0)
}

func TestCalendarProjectionLeasesDoNotSerializeDifferentEvents(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, eventID := range []string{"event-1", "event-2"} {
		if errorValue := enqueueCalendarChannelProjection(ctx, database, eventID); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
	}
	database.Close()
	firstLease, firstAcquired, errorValue := service.acquireCalendarProjectionLease(ctx, "event-1")
	if errorValue != nil || !firstAcquired {
		t.Fatalf("first lease acquired=%v error=%v", firstAcquired, errorValue)
	}
	secondLease, secondAcquired, errorValue := service.acquireCalendarProjectionLease(ctx, "event-2")
	if errorValue != nil || !secondAcquired {
		t.Fatalf("second lease acquired=%v error=%v", secondAcquired, errorValue)
	}
	for _, lease := range []calendarProjectionLease{firstLease, secondLease} {
		if _, hasNextGeneration, errorValue := service.completeCalendarProjectionGeneration(ctx, lease); errorValue != nil || hasNextGeneration {
			t.Fatalf("complete lease=%+v next=%v error=%v", lease, hasNextGeneration, errorValue)
		}
	}
}

func TestCalendarProjectionLeaseIsReleasedOnSchemaRestart(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, database, "event-1"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	firstLease, acquired, errorValue := service.acquireCalendarProjectionLease(ctx, "event-1")
	if errorValue != nil || !acquired {
		t.Fatalf("first lease acquired=%v error=%v", acquired, errorValue)
	}
	service.databaseSchemas = newAdminDatabaseSchemas()
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()
	secondLease, acquired, errorValue := service.acquireCalendarProjectionLease(ctx, "event-1")
	if errorValue != nil || !acquired {
		t.Fatalf("restart lease acquired=%v error=%v", acquired, errorValue)
	}
	if secondLease.Owner == firstLease.Owner || secondLease.Generation != firstLease.Generation {
		t.Fatalf("first lease=%+v restart lease=%+v", firstLease, secondLease)
	}
	if _, hasNextGeneration, errorValue := service.completeCalendarProjectionGeneration(ctx, secondLease); errorValue != nil || hasNextGeneration {
		t.Fatalf("complete restart lease next=%v error=%v", hasNextGeneration, errorValue)
	}
}

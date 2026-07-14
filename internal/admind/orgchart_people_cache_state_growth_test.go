package admind

import (
	"context"
	"strconv"
	"testing"
)

func TestOrgchartUserMutationDoesNotPersistUnknownPersonStates(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	for batchIndex := 0; batchIndex < 3; batchIndex++ {
		identities := make([]orgchartPersonIdentity, 0, localUsersBatchMaximumUsers)
		for userIndex := 0; userIndex < localUsersBatchMaximumUsers; userIndex++ {
			suffix := strconv.Itoa(batchIndex*localUsersBatchMaximumUsers + userIndex)
			identities = append(identities, orgchartPersonIdentity{
				UserID: "new-user-" + suffix,
				Email:  "new-user-" + suffix + "@example.com",
			})
		}
		mutation, errorValue := service.startOrgchartUserMutation(ctx, identities)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		mutation.completeAfterRequest(ctx)
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var personStateCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM orgchart_people_cache_states WHERE cache_kind = ?`, string(orgchartPeopleCachePerson)).Scan(&personStateCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if personStateCount != 0 {
		t.Fatalf("person state count = %d; want 0", personStateCount)
	}
}

func TestOrgchartPeopleCacheRejectsWriteFromPreviousListRevision(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	personKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "new-user"}
	identity := orgchartPersonIdentity{UserID: personKey.Key, Email: "new@example.com"}
	mutation, errorValue := service.startOrgchartUserMutation(ctx, []orgchartPersonIdentity{identity})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := mutation.complete(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	writtenByKey, errorValue := service.writeOrgchartPeopleCachePayloadsIfCurrent(ctx, []orgchartPeopleCacheWrite{{
		Key:                     personKey,
		Revision:                0,
		HasExpectedListRevision: true,
		ExpectedListRevision:    0,
		PayloadJSON:             []byte(`{"record":{"userID":"new-user","email":"new@example.com"}}`),
	}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if writtenByKey[personKey] {
		t.Fatal("person cache write from previous list revision succeeded")
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var personStateCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM orgchart_people_cache_states WHERE cache_kind = ? AND cache_key = ?`, string(personKey.Kind), personKey.Key).Scan(&personStateCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if personStateCount != 0 {
		t.Fatalf("person state count = %d; want 0", personStateCount)
	}
}

func TestOrgchartPeopleCacheBypassesUnknownPersonWhileListMutationActive(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	identity := orgchartPersonIdentity{UserID: "new-user", Email: "new@example.com"}
	mutation, errorValue := service.startOrgchartUserMutation(ctx, []orgchartPersonIdentity{identity})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer mutation.completeAfterRequest(ctx)
	_, errorValue = service.applyCachedOrgchartPeople(ctx, pagesUsersResponse{Records: []adminUserMutation{{
		UserID: identity.UserID,
		Email:  identity.Email,
		Name:   "New User",
	}}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	personKey, found := orgchartPersonCacheKey(identity.UserID, identity.Email)
	if !found {
		t.Fatal("person cache key not found")
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{personKey})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if snapshots[personKey].Found {
		t.Fatal("person cache was written while list mutation was active")
	}
}

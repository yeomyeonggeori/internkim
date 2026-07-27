package admind

import (
	"context"
	"strconv"
	"testing"
)

func TestOrganizationUserMutationDoesNotPersistUnknownPersonStates(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	for batchIndex := 0; batchIndex < 3; batchIndex++ {
		identities := make([]organizationPersonIdentity, 0, localUsersBatchMaximumUsers)
		for userIndex := 0; userIndex < localUsersBatchMaximumUsers; userIndex++ {
			suffix := strconv.Itoa(batchIndex*localUsersBatchMaximumUsers + userIndex)
			identities = append(identities, organizationPersonIdentity{
				UserID: "new-user-" + suffix,
				Email:  "new-user-" + suffix + "@example.com",
			})
		}
		mutation, errorValue := service.startOrganizationUserMutation(ctx, identities)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		mutation.completeAfterRequest(ctx)
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var personStateCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_people_cache_states WHERE cache_kind = ?`, string(organizationPeopleCachePerson)).Scan(&personStateCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if personStateCount != 0 {
		t.Fatalf("person state count = %d; want 0", personStateCount)
	}
}

func TestOrganizationPeopleCacheRejectsWriteFromPreviousListRevision(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	personKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "new-user"}
	identity := organizationPersonIdentity{UserID: personKey.Key, Email: "new@example.com"}
	mutation, errorValue := service.startOrganizationUserMutation(ctx, []organizationPersonIdentity{identity})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := mutation.complete(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	writtenByKey, errorValue := service.writeOrganizationPeopleCachePayloadsIfCurrent(ctx, []organizationPeopleCacheWrite{{
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
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var personStateCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM organization_people_cache_states WHERE cache_kind = ? AND cache_key = ?`, string(personKey.Kind), personKey.Key).Scan(&personStateCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if personStateCount != 0 {
		t.Fatalf("person state count = %d; want 0", personStateCount)
	}
}

func TestOrganizationPeopleCacheBypassesUnknownPersonWhileListMutationActive(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	identity := organizationPersonIdentity{UserID: "new-user", Email: "new@example.com"}
	mutation, errorValue := service.startOrganizationUserMutation(ctx, []organizationPersonIdentity{identity})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer mutation.completeAfterRequest(ctx)
	_, errorValue = service.applyCachedOrganizationPeople(ctx, pagesUsersResponse{Records: []adminUserMutation{{
		UserID: identity.UserID,
		Email:  identity.Email,
		Name:   "New User",
	}}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	personKey, found := organizationPersonCacheKey(identity.UserID, identity.Email)
	if !found {
		t.Fatal("person cache key not found")
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{personKey})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if snapshots[personKey].Found {
		t.Fatal("person cache was written while list mutation was active")
	}
}

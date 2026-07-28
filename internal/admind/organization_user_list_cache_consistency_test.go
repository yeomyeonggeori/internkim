package admind

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOrganizationPeopleCacheReusesUserList(t *testing.T) {
	service := newLocalUsersTestService(t)
	loadCount := 0
	loader := func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}, nil
	}

	for range 2 {
		response, _, errorValue := service.readCachedOrganizationUserList(context.Background(), loader)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if len(response.Records) != 1 || response.Records[0].UserID != "user-1" {
			t.Fatalf("response = %#v", response)
		}
	}
	if loadCount != 1 {
		t.Fatalf("load count = %d; want 1", loadCount)
	}
}

func TestOrganizationPeopleCacheRefreshesUserListWhenSourceRevisionChanges(t *testing.T) {
	service := newLocalUsersTestService(t)
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	loadCount := 0
	loader := func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-" + string(rune('0'+loadCount)), Email: "one@example.com", Role: "member"}}}, nil
	}
	first, _, errorValue := service.readCachedOrganizationUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	second, _, errorValue := service.readCachedOrganizationUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.Records[0].UserID == second.Records[0].UserID || loadCount != 2 {
		t.Fatalf("first = %#v second = %#v load count = %d", first, second, loadCount)
	}
}

func TestOrganizationPeopleCacheInvalidatesPersonWhenPreviousListIsCorrupt(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, listKey, 0, "1", []byte(`{`)); errorValue != nil || !written {
		t.Fatalf("write corrupt list cache: written = %t error = %v", written, errorValue)
	}
	personKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	personPayload, errorValue := json.Marshal(organizationCachedPerson{Record: newOrganizationCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, personKey, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "After", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "After" {
		t.Fatalf("name = %q; want After", response.response.Records[0].Name)
	}
}

func TestOrganizationPeopleCacheDoesNotApplyPersonEntryWhenSourceChangesDuringLoad(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	personKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	personPayload, errorValue := json.Marshal(organizationCachedPerson{Record: newOrganizationCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, personKey, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "After", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "After" {
		t.Fatalf("name = %q; want After", response.response.Records[0].Name)
	}
}

func TestOrganizationPeopleCacheBypassesPersonEntriesWhenListWriteLosesRevisionRace(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
	_, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		if errorValue := service.beginOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{listKey}); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
		if errorValue := service.completeOrganizationPeopleCacheMutation(ctx, []organizationPeopleCacheKey{listKey}); errorValue != nil {
			return pagesUsersResponse{}, errorValue
		}
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if cachePolicy != organizationPeopleCacheBypassed {
		t.Fatalf("cache policy = %#v; want bypassed", cachePolicy)
	}
}

func TestOrganizationPeopleCacheDoesNotRebuildPersonAfterListReadRevisionChanges(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	loader := func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"}}}, nil
	}
	users, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue = service.readCachedOrganizationUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	identities := []organizationPersonIdentity{{UserID: "user-1", Email: "one@example.com"}}
	keys, errorValue := service.beginOrganizationUserMutation(ctx, identities)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.completeOrganizationUserMutation(ctx, keys); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrganizationPersonCacheFound(t, service, "user-1", false)
}

func TestOrganizationPeopleCacheDoesNotApplyPersonAfterSourceChangesFollowingListRead(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrganizationUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "Source", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	personPayload, errorValue := json.Marshal(organizationCachedPerson{Record: newOrganizationCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Cached", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	personKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-1"}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{personKey})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, personKey, snapshots[personKey].Revision, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrganizationPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "Source" {
		t.Fatalf("name = %q; want Source", response.response.Records[0].Name)
	}
}

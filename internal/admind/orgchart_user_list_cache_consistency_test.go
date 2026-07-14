package admind

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOrgchartPeopleCacheReusesUserList(t *testing.T) {
	service := newLocalUsersTestService(t)
	loadCount := 0
	loader := func(context.Context) (pagesUsersResponse, error) {
		loadCount++
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}, nil
	}

	for range 2 {
		response, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
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

func TestOrgchartPeopleCacheRefreshesUserListWhenSourceRevisionChanges(t *testing.T) {
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
	first, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	second, _, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.Records[0].UserID == second.Records[0].UserID || loadCount != 2 {
		t.Fatalf("first = %#v second = %#v load count = %d", first, second, loadCount)
	}
}

func TestOrgchartPeopleCacheInvalidatesPersonWhenPreviousListIsCorrupt(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, listKey, 0, "1", []byte(`{`)); errorValue != nil || !written {
		t.Fatalf("write corrupt list cache: written = %t error = %v", written, errorValue)
	}
	personKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	personPayload, errorValue := json.Marshal(orgchartCachedPerson{Record: newOrgchartCachedUserRecord(adminUserMutation{UserID: "user-1", Email: "one@example.com", Name: "Before", Role: "member"})})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, personKey, 0, "", personPayload); errorValue != nil || !written {
		t.Fatalf("write person cache: written = %t error = %v", written, errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: "After", Role: "member"}}}, nil
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.response.Records[0].Name != "After" {
		t.Fatalf("name = %q; want After", response.response.Records[0].Name)
	}
}

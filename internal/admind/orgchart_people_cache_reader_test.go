package admind

import (
	"context"
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
		response, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
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
	first, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	second, errorValue := service.readCachedOrgchartUserList(context.Background(), loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.Records[0].UserID == second.Records[0].UserID || loadCount != 2 {
		t.Fatalf("first = %#v second = %#v load count = %d", first, second, loadCount)
	}
}

func TestOrgchartPeopleCacheReusesPersonEntry(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Designer",
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}
	first, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_profiles SET job_title = 'Engineer' WHERE user_id = 'user-1'`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	second, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if first.response.Records[0].JobTitle != "Designer" || second.response.Records[0].JobTitle != "Designer" {
		t.Fatalf("first = %#v second = %#v", first.response, second.response)
	}
}

func TestOrgchartPeopleCacheRebuildsCorruptPersonPayload(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	key := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "user-1"}
	written, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !written {
		t.Fatal("expected corrupt fixture write")
	}
	users := pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Role: "member"}}}

	response, errorValue := service.applyCachedOrgchartPeople(ctx, users)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.response.Records) != 1 || response.response.Records[0].UserID != "user-1" {
		t.Fatalf("response = %#v", response.response)
	}
}

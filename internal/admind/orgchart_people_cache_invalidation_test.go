package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrgchartPeopleCacheInvalidatesChangedUserFromSourceRevision(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	name := "Old Name"
	loader := func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: []adminUserMutation{{UserID: "user-1", Email: "one@example.com", Name: name, Role: "member"}}}, nil
	}
	users, errorValue := service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	name = "New Name"
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCachedOrgchartUserList(ctx, loader); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-1", false)
}

func TestOrgchartPeopleCacheInvalidatesOnlyUpdatedProfile(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	users := pagesUsersResponse{Records: []adminUserMutation{
		{UserID: "user-1", Email: "one@example.com", Role: "member"},
		{UserID: "user-2", Email: "two@example.com", Role: "member"},
	}}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{{
		UserID:            "user-1",
		Email:             "one@example.com",
		JobTitle:          "Engineer",
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-1", false)
	assertOrgchartPersonCacheFound(t, service, "user-2", true)
}

func TestOrgchartPeopleCacheInvalidatesChangedGroupAndAffectedProfile(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "design", Name: "Design"}, {ID: "engineering", Name: "Engineering"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{
		{UserID: "user-1", Email: "one@example.com", PrimaryGroupID: "design", EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
		{UserID: "user-2", Email: "two@example.com", PrimaryGroupID: "engineering", EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	users := pagesUsersResponse{Records: []adminUserMutation{
		{UserID: "user-1", Email: "one@example.com", Role: "member"},
		{UserID: "user-2", Email: "two@example.com", Role: "member"},
	}}
	if _, errorValue := service.applyCachedOrgchartPeople(ctx, users); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{{ID: "design", Name: "Product Design"}, {ID: "engineering", Name: "Engineering"}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartCacheFound(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}, false)
	assertOrgchartPersonCacheFound(t, service, "user-1", false)
	assertOrgchartPersonCacheFound(t, service, "user-2", true)
}

func TestOrgchartPeopleCacheMarksLocalUserMutationDirtyBeforeExternalWrite(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
	personKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: "email:new@example.com"}
	for _, key := range []orgchartPeopleCacheKey{listKey, personKey} {
		if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	mattermostCreated := false
	blueclawSaved := false
	checkedDirty := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !checkedDirty {
			for _, key := range []orgchartPeopleCacheKey{listKey, personKey} {
				snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
				if errorValue != nil {
					t.Fatal(errorValue)
				}
				if !snapshots[key].IsDirty || snapshots[key].Found {
					t.Fatalf("cache state before %s %s = %#v", request.Method, request.URL.String(), snapshots[key])
				}
			}
			checkedDirty = true
		}
		if request.URL.Host == "blueclaw.local" {
			return localUsersBlueclawUpsertResponse(t, request, &blueclawSaved)
		}
		return localUsersMattermostUpsertResponse(t, request, &mattermostCreated)
	})}

	responseRecorder := httptest.NewRecorder()
	requestBody := strings.NewReader(`{"email":"new@example.com","handle":"new-user","name":"New User","role":"admin","circles":[],"note":"Local note"}`)
	service.localUpsertUser(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	for _, key := range []orgchartPeopleCacheKey{listKey, personKey} {
		snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, []orgchartPeopleCacheKey{key})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if snapshots[key].IsDirty {
			t.Fatalf("cache state remained dirty: %#v", snapshots[key])
		}
	}
}

func assertOrgchartPersonCacheFound(t *testing.T, service *Service, userID string, expected bool) {
	t.Helper()
	assertOrgchartCacheFound(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCachePerson, Key: userID}, expected)
}

func assertOrgchartCacheFound(t *testing.T, service *Service, key orgchartPeopleCacheKey, expected bool) {
	t.Helper()
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), []orgchartPeopleCacheKey{key})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if snapshots[key].Found != expected {
		t.Fatalf("cache %v found = %t; want %t", key, snapshots[key].Found, expected)
	}
}

package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOrgchartPeopleCacheInvalidatesCanonicalLocalUser(t *testing.T) {
	service := newLocalUsersTestService(t)
	canonicalUserID := "blueclaw-existing"
	email := "existing@example.com"
	preloadOrgchartIdentityCache(t, service, canonicalUserID, email)
	checkedCanonicalMutation := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, email), nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			assertOrgchartIdentityMutationActive(t, service, canonicalUserID, email)
			checkedCanonicalMutation = true
			return nil, errors.New("stop after canonical mutation check")
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	requestBody := strings.NewReader(`{"email":" Existing@Example.COM ","handle":"existing-user","name":"Existing User","role":"admin"}`)
	responseRecorder := httptest.NewRecorder()
	service.localUpsertUser(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !checkedCanonicalMutation {
		t.Fatal("canonical mutation was not checked")
	}
	assertOrgchartIdentityCacheFound(t, service, canonicalUserID, email, false)
}

func TestOrgchartPeopleCacheInvalidatesCanonicalDeletedUser(t *testing.T) {
	service := newLocalUsersTestService(t)
	canonicalUserID := "blueclaw-deleted"
	mattermostUserID := "mattermost-deleted"
	email := "deleted@example.com"
	preloadOrgchartIdentityCache(t, service, canonicalUserID, email)
	checkedCanonicalMutation := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/deleted@example.com":
			return jsonResponse(http.StatusOK, `{"id":"mattermost-deleted","email":"deleted@example.com","username":"deleted-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[
				{"id":"mattermost-admin","email":"admin@example.com","username":"admin-user","roles":"system_admin system_user","delete_at":0},
				{"id":"mattermost-deleted","email":"deleted@example.com","username":"deleted-user","roles":"system_user","delete_at":0}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[
				{"user_id":"mattermost-admin","roles":"team_user team_admin"},
				{"user_id":"mattermost-deleted","roles":"team_user"}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, email), nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/"+mattermostUserID:
			return jsonResponse(http.StatusOK, `{"id":"mattermost-deleted","email":"deleted@example.com","username":"deleted-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://mattermost.local/api/v4/users/"+mattermostUserID:
			assertOrgchartIdentityMutationActive(t, service, canonicalUserID, email)
			checkedCanonicalMutation = true
			return nil, errors.New("stop after canonical deletion check")
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseRecorder := httptest.NewRecorder()
	service.localRemoveUser(responseRecorder, httptest.NewRequest(http.MethodDelete, "/admin/api/users/deleted@example.com", nil), " Deleted@Example.COM ")

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !checkedCanonicalMutation {
		t.Fatal("canonical deletion mutation was not checked")
	}
	assertOrgchartIdentityCacheFound(t, service, canonicalUserID, email, false)
}

func TestOrgchartPeopleCacheInvalidatesCanonicalBatchUsers(t *testing.T) {
	service := newLocalUsersTestService(t)
	canonicalUserID := "blueclaw-existing"
	existingEmail := "existing@example.com"
	newEmail := "new-batch@example.com"
	preloadOrgchartIdentityCache(t, service, canonicalUserID, existingEmail)
	activePersonKeys := []string{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, existingEmail), nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			activePersonKeys = activeOrgchartPersonMutationKeys(t, service)
			return nil, errors.New("stop after batch mutation check")
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	requestBody := strings.NewReader(`{"users":[
		{"email":"existing@example.com","handle":"existing-user","name":"Existing User","role":"admin"},
		{"email":"new-batch@example.com","handle":"new-batch","name":"New Batch User","role":"admin"}
	]}`)
	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", requestBody))

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !containsString(activePersonKeys, canonicalUserID) {
		t.Fatalf("active person keys = %#v; missing %q", activePersonKeys, canonicalUserID)
	}
	for _, email := range []string{existingEmail, newEmail} {
		emailKey := "email:" + email
		if !containsString(activePersonKeys, emailKey) {
			t.Fatalf("active person keys = %#v; missing %q", activePersonKeys, emailKey)
		}
	}
	generatedUserIDFound := false
	for _, key := range activePersonKeys {
		if strings.HasPrefix(key, "user-") {
			generatedUserIDFound = true
		}
	}
	if !generatedUserIDFound {
		t.Fatalf("active person keys = %#v; missing generated user ID", activePersonKeys)
	}
	assertOrgchartPersonCacheFound(t, service, canonicalUserID, false)
}

func TestOrgchartPeopleCacheInvalidatesCanonicalProxyUser(t *testing.T) {
	for _, testCase := range []struct {
		name              string
		circlesDocument   string
		expectsPolicySave bool
	}{
		{name: "invite"},
		{name: "upsert", circlesDocument: `,"circles":[]`, expectsPolicySave: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := newOrgchartProxyMutationTestService(t)
			canonicalUserID := "blueclaw-existing"
			remoteUserID := "remote-existing"
			email := "existing@example.com"
			preloadOrgchartIdentityCache(t, service, canonicalUserID, email)
			pagesUserID := ""
			blueclawPersonID := ""
			policySaved := false
			service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
					return response, nil
				}
				switch {
				case request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
					return jsonResponse(http.StatusOK, `{"records":[{"userID":"`+remoteUserID+`","email":"existing@example.com","role":"admin"}]}`, nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
					return jsonResponse(http.StatusOK, localUsersPolicyWithPersonAndCircleSync(canonicalUserID, email), nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-1":
					return jsonResponse(http.StatusOK, `{"id":"user-1","email":"existing@example.com","username":"existing-user","roles":"system_user"}`, nil), nil
				case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch":
					return jsonResponse(http.StatusOK, `{"id":"user-1","email":"existing@example.com","username":"existing-user","roles":"system_user"}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "https://api.example.test/api/users":
					var payload adminUserMutation
					if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
						t.Fatal(errorValue)
					}
					pagesUserID = payload.UserID
					return jsonResponse(http.StatusOK, `{"records":[{"userID":"`+remoteUserID+`","email":"existing@example.com","role":"admin"}]}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/people/invite":
					assertOrgchartIdentityMutationActive(t, service, canonicalUserID, email)
					var payload map[string]string
					if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
						t.Fatal(errorValue)
					}
					blueclawPersonID = payload["personID"]
					if blueclawPersonID != canonicalUserID {
						t.Fatalf("Blueclaw personID = %q; want %q", blueclawPersonID, canonicalUserID)
					}
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/policy/save":
					policySaved = true
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/circle-staff":
					return jsonResponse(http.StatusOK, `{"id":"circle-staff-channel"}`, nil), nil
				case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/patch":
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/posts?per_page=100":
					return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/members":
					return jsonResponse(http.StatusCreated, `{}`, nil), nil
				default:
					t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
					return nil, nil
				}
			})}

			requestBody := strings.NewReader(`{"email":" Existing@Example.COM ","handle":"existing-user","name":"","role":"admin","mattermostUserID":"user-1"` + testCase.circlesDocument + `}`)
			responseRecorder := httptest.NewRecorder()
			service.proxyUsers(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))

			if responseRecorder.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
			}
			if pagesUserID != remoteUserID {
				t.Fatalf("Pages userID = %q; want %q", pagesUserID, remoteUserID)
			}
			if blueclawPersonID != canonicalUserID {
				t.Fatalf("Blueclaw personID = %q; want %q", blueclawPersonID, canonicalUserID)
			}
			if policySaved != testCase.expectsPolicySave {
				t.Fatalf("policy saved = %t; want %t", policySaved, testCase.expectsPolicySave)
			}
			assertOrgchartIdentityCacheFound(t, service, canonicalUserID, email, false)
		})
	}
}

func TestOrgchartPeopleCacheInvalidatesCanonicalProxyDeletedUser(t *testing.T) {
	service := newOrgchartProxyMutationTestService(t)
	canonicalUserID := "blueclaw-deleted"
	remoteUserID := "remote-deleted"
	mattermostUserID := "mattermost-deleted"
	email := "deleted@example.com"
	preloadOrgchartIdentityCache(t, service, canonicalUserID, email)
	mattermostLoginCount := 0
	checkedCanonicalMutation := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			mattermostLoginCount++
			if mattermostLoginCount == 1 {
				return nil, errors.New("skip provisioner sync")
			}
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[
				{"userID":"admin-source","email":"admin@example.com","role":"admin"},
				{"userID":"`+remoteUserID+`","email":"deleted@example.com","role":"member","mattermostUserID":"`+mattermostUserID+`"}
			]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, email), nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/"+mattermostUserID:
			return jsonResponse(http.StatusOK, `{"id":"mattermost-deleted","email":"deleted@example.com","username":"deleted-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://mattermost.local/api/v4/users/"+mattermostUserID:
			assertOrgchartIdentityMutationActive(t, service, canonicalUserID, email)
			checkedCanonicalMutation = true
			return nil, errors.New("stop after proxy deletion check")
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseRecorder := httptest.NewRecorder()
	service.proxyUsers(responseRecorder, httptest.NewRequest(http.MethodDelete, "/admin/api/users/deleted@example.com", nil))

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !checkedCanonicalMutation {
		t.Fatal("canonical proxy deletion mutation was not checked")
	}
	assertOrgchartIdentityCacheFound(t, service, canonicalUserID, email, false)
}

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
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	name = "New Name"
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2","users":["one@example.com"]}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, _, errorValue := service.readCachedOrgchartUserList(ctx, loader); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-1", false)
}

func TestOrgchartPeopleCacheInvalidatesAddedAndRemovedUsersFromSourceRevision(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	statePath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"1"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	currentRecords := []adminUserMutation{
		{UserID: "user-removed", Email: "removed@example.com", Role: "member"},
		{UserID: "user-retained", Email: "retained@example.com", Role: "member"},
	}
	loader := func(context.Context) (pagesUsersResponse, error) {
		return pagesUsersResponse{Records: currentRecords}, nil
	}
	users, cachePolicy, errorValue := service.readCachedOrgchartUserList(ctx, loader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.applyOrgchartPeople(ctx, users, cachePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	currentRecords = []adminUserMutation{
		{UserID: "user-retained", Email: "retained@example.com", Role: "member"},
		{UserID: "user-added", Email: "added@example.com", Role: "member"},
	}
	if errorValue := os.WriteFile(statePath, []byte(`{"revision":"2"}`), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, _, errorValue := service.readCachedOrgchartUserList(ctx, loader); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartPersonCacheFound(t, service, "user-removed", false)
	assertOrgchartPersonCacheFound(t, service, "user-retained", true)
	assertOrgchartPersonCacheFound(t, service, "user-added", false)
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

func TestOrgchartPeopleCacheInvalidatesMergedDeletedGroupsAndRewrittenProfiles(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{
		{ID: "engineering", Name: "Engineering"},
		{ID: "engineering-duplicate", Name: "Platform"},
		{ID: "operations", Name: "Operations"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(ctx, []orgchartProfile{
		{UserID: "user-1", Email: "one@example.com", PrimaryGroupID: "engineering-duplicate", GroupIDs: []string{"engineering-duplicate"}, EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
		{UserID: "user-2", Email: "two@example.com", PrimaryGroupID: "operations", GroupIDs: []string{"operations"}, EmploymentStatus: orgchartEmploymentStatusActive, IsOrgchartVisible: true},
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
	if errorValue := service.writeOrgchartGroups(ctx, []orgGroupRecord{
		{ID: "engineering", Name: "Engineering"},
		{ID: "engineering-duplicate", Name: "engineering"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertOrgchartCacheFound(t, service, orgchartPeopleCacheKey{Kind: orgchartPeopleCacheGroups, Key: orgchartPeopleCacheSingletonKey}, false)
	assertOrgchartPersonCacheFound(t, service, "user-1", false)
	assertOrgchartPersonCacheFound(t, service, "user-2", false)
	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if profilesByEmail["one@example.com"].PrimaryGroupID != "engineering" {
		t.Fatalf("rewritten profile = %#v", profilesByEmail["one@example.com"])
	}
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
		if !checkedDirty && request.Method != http.MethodGet {
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

func TestOrgchartPeopleCacheCompletesLocalUserMutationAfterExternalFailure(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("external user write failed")
	})}

	responseRecorder := httptest.NewRecorder()
	requestBody := strings.NewReader(`{"email":"new@example.com","handle":"new-user","name":"New User","role":"admin","circles":[]}`)
	service.localUpsertUser(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	keys := []orgchartPeopleCacheKey{
		{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey},
		{Kind: orgchartPeopleCachePerson, Key: "email:new@example.com"},
	}
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(ctx, keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		if snapshots[key].IsDirty || snapshots[key].ActiveMutations != 0 {
			t.Fatalf("cache state after failed mutation = %#v", snapshots[key])
		}
	}
}

func TestOrgchartUserMutationCleanupFailurePreservesSuccessLocal(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		expectedService := newLocalUsersTestService(t)
		expectedService.HTTPClient = newOrgchartCleanupFailureLocalUpsertClient(t)
		expectedResponse := performOrgchartCleanupFailureLocalUpsert(t, expectedService, false)

		actualService := newLocalUsersTestService(t)
		actualService.HTTPClient = newOrgchartCleanupFailureLocalUpsertClient(t)
		identity := orgchartPersonIdentity{UserID: "user-new", Email: "new@example.com"}
		preloadOrgchartUserMutationCache(t, actualService, identity)
		failOrgchartUserMutationCompletion(t, actualService)
		actualResponse := performOrgchartCleanupFailureLocalUpsert(t, actualService, false)

		assertOrgchartCleanupFailureUserResponse(t, expectedResponse, actualResponse, "Local note")
		assertOrgchartUserMutationCacheIncomplete(t, actualService, identity)
	})

	t.Run("batch", func(t *testing.T) {
		expectedService := newLocalUsersTestService(t)
		expectedService.HTTPClient = newOrgchartCleanupFailureLocalUpsertClient(t)
		expectedResponse := performOrgchartCleanupFailureLocalUpsert(t, expectedService, true)

		actualService := newLocalUsersTestService(t)
		actualService.HTTPClient = newOrgchartCleanupFailureLocalUpsertClient(t)
		failOrgchartUserMutationCompletion(t, actualService)
		actualResponse := performOrgchartCleanupFailureLocalUpsert(t, actualService, true)

		assertOrgchartCleanupFailureUserResponse(t, expectedResponse, actualResponse, "")
	})

	t.Run("delete", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		service.HTTPClient = newOrgchartCleanupFailureLocalDeleteClient(t)
		failOrgchartUserMutationCompletion(t, service)

		responseRecorder := httptest.NewRecorder()
		service.localRemoveUser(responseRecorder, httptest.NewRequest(http.MethodDelete, "/admin/api/users/member@example.com", nil), "member@example.com")

		if responseRecorder.Code != http.StatusOK || responseRecorder.Body.String() != "{\"ok\":true}\n" {
			t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
		}
	})
}

func TestOrgchartUserMutationCleanupFailureDeferredRetryClearsCacheState(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	identity := orgchartPersonIdentity{UserID: "user-new", Email: "new@example.com"}
	preloadOrgchartUserMutationCache(t, service, identity)
	mutation, errorValue := service.startOrgchartUserMutation(ctx, []orgchartPersonIdentity{identity})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	failOrgchartUserMutationCompletion(t, service)

	func() {
		defer mutation.completeAfterRequest(ctx)
		mutation.completeAfterSourceMutation(ctx)
		if mutation.isComplete {
			t.Fatal("mutation completed after failed cache cleanup")
		}
		assertOrgchartUserMutationCacheIncomplete(t, service, identity)
		dropOrgchartUserMutationCompletionFailure(t, service)
	}()

	if !mutation.isComplete {
		t.Fatal("deferred cache cleanup did not complete mutation")
	}
	assertOrgchartUserMutationCacheComplete(t, service, identity)
}

func performOrgchartCleanupFailureLocalUpsert(t *testing.T, service *Service, isBatch bool) *httptest.ResponseRecorder {
	t.Helper()
	responseRecorder := httptest.NewRecorder()
	if isBatch {
		requestBody := strings.NewReader(`{"users":[{"email":"new@example.com","handle":"new-user","name":"New User","role":"admin","circles":[],"note":"Local note"}]}`)
		service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", requestBody))
		return responseRecorder
	}
	requestBody := strings.NewReader(`{"email":"new@example.com","handle":"new-user","name":"New User","role":"admin","circles":[],"note":"Local note"}`)
	service.localUpsertUser(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))
	return responseRecorder
}

func failOrgchartUserMutationCompletion(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openOrgchartDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.Exec(`
		CREATE TRIGGER fail_orgchart_user_mutation_completion
		BEFORE UPDATE OF active_mutations ON orgchart_people_cache_states
		WHEN NEW.active_mutations < OLD.active_mutations
		BEGIN
			SELECT RAISE(FAIL, 'forced orgchart user mutation completion failure');
		END
	`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func dropOrgchartUserMutationCompletionFailure(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openOrgchartDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`DROP TRIGGER fail_orgchart_user_mutation_completion`); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func newOrgchartCleanupFailureLocalUpsertClient(t *testing.T) *http.Client {
	t.Helper()
	mattermostCreated := false
	blueclawSaved := false
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200" {
			return jsonResponse(http.StatusOK, `[{"id":"user-new","email":"new@example.com","username":"new-user","nickname":"New User","roles":"system_user","delete_at":0}]`, nil), nil
		}
		if request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members" {
			return jsonResponse(http.StatusOK, `[{"user_id":"user-new","roles":"team_user team_admin"}]`, nil), nil
		}
		if request.URL.Host == "blueclaw.local" {
			return localUsersBlueclawUpsertResponse(t, request, &blueclawSaved)
		}
		return localUsersMattermostUpsertResponse(t, request, &mattermostCreated)
	})}
}

func newOrgchartCleanupFailureLocalDeleteClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/member@example.com":
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"member@example.com","username":"member-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[{"id":"user-1","email":"admin@example.com","username":"admin-user","roles":"system_admin system_user","delete_at":0},{"id":"user-2","email":"member@example.com","username":"member-user","roles":"system_user","delete_at":0}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[{"user_id":"user-1","roles":"team_user team_admin"},{"user_id":"user-2","roles":"team_user"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-2":
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"member@example.com","username":"member-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://mattermost.local/api/v4/users/user-2":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/people/invite":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/policy/save":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://blueclaw.local/admin/api/people?email=member%40example.com":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
}

func assertOrgchartCleanupFailureUserResponse(t *testing.T, expectedResponse *httptest.ResponseRecorder, actualResponse *httptest.ResponseRecorder, expectedNote string) {
	t.Helper()
	if expectedResponse.Code != http.StatusOK || actualResponse.Code != expectedResponse.Code {
		t.Fatalf("status = %d body = %s; want status = %d body = %s", actualResponse.Code, actualResponse.Body.String(), expectedResponse.Code, expectedResponse.Body.String())
	}
	expectedDocument := normalizedOrgchartCleanupFailureResponse(t, expectedResponse)
	actualDocument := normalizedOrgchartCleanupFailureResponse(t, actualResponse)
	if !reflect.DeepEqual(actualDocument, expectedDocument) {
		t.Fatalf("response = %#v; want %#v", actualDocument, expectedDocument)
	}
	var response pagesUsersResponse
	if errorValue := json.Unmarshal(actualResponse.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedRecord := adminUserMutation{
		UserID:             "user-new",
		Handle:             "new-user",
		Name:               "New User",
		Email:              "new@example.com",
		Image:              profileImagePathForEmail("new@example.com"),
		Note:               expectedNote,
		Role:               "admin",
		Circles:            []string{"staff", "admin"},
		MattermostUserID:   "user-new",
		MattermostUsername: "new-user",
		Status:             "active",
	}
	if len(response.Records) != 1 || !reflect.DeepEqual(response.Records[0], expectedRecord) {
		t.Fatalf("record = %#v; want %#v", response.Records, expectedRecord)
	}
}

func normalizedOrgchartCleanupFailureResponse(t *testing.T, responseRecorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var document map[string]any
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	temporaryPassword, found := document["temporaryPassword"].(string)
	if !found || strings.TrimSpace(temporaryPassword) == "" {
		t.Fatalf("temporary password missing from response = %#v", document)
	}
	delete(document, "temporaryPassword")
	return document
}

func preloadOrgchartUserMutationCache(t *testing.T, service *Service, identity orgchartPersonIdentity) {
	t.Helper()
	for _, key := range orgchartUserMutationCacheKeys([]orgchartPersonIdentity{identity}) {
		if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func assertOrgchartUserMutationCacheIncomplete(t *testing.T, service *Service, identity orgchartPersonIdentity) {
	t.Helper()
	keys := orgchartUserMutationCacheKeys([]orgchartPersonIdentity{identity})
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		snapshot := snapshots[key]
		if (!snapshot.IsDirty && snapshot.ActiveMutations == 0) || snapshot.Found {
			t.Fatalf("incomplete mutation cache state for %v = %#v", key, snapshot)
		}
	}
}

func assertOrgchartUserMutationCacheComplete(t *testing.T, service *Service, identity orgchartPersonIdentity) {
	t.Helper()
	keys := orgchartUserMutationCacheKeys([]orgchartPersonIdentity{identity})
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		snapshot := snapshots[key]
		if snapshot.IsDirty || snapshot.ActiveMutations != 0 || snapshot.Found {
			t.Fatalf("completed mutation cache state for %v = %#v", key, snapshot)
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

func preloadOrgchartIdentityCache(t *testing.T, service *Service, userID string, email string) {
	t.Helper()
	for _, key := range orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: userID, Email: email}) {
		if _, errorValue := service.writeOrgchartPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func assertOrgchartIdentityMutationActive(t *testing.T, service *Service, userID string, email string) {
	t.Helper()
	keys := orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: userID, Email: email})
	snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		snapshot := snapshots[key]
		if !snapshot.IsDirty || snapshot.ActiveMutations != 1 || snapshot.Found {
			t.Fatalf("identity mutation state for %v = %#v", key, snapshot)
		}
	}
}

func assertOrgchartIdentityCacheFound(t *testing.T, service *Service, userID string, email string, expected bool) {
	t.Helper()
	for _, key := range orgchartPersonCacheKeys(orgchartPersonIdentity{UserID: userID, Email: email}) {
		assertOrgchartCacheFound(t, service, key, expected)
	}
}

func activeOrgchartPersonMutationKeys(t *testing.T, service *Service) []string {
	t.Helper()
	database, errorValue := service.openOrgchartDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.Query(`SELECT cache_key FROM orgchart_people_cache_states WHERE cache_kind = ? AND active_mutations > 0 ORDER BY cache_key`, string(orgchartPeopleCachePerson))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if errorValue := rows.Scan(&key); errorValue != nil {
			t.Fatal(errorValue)
		}
		keys = append(keys, key)
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return keys
}

func localUsersPolicyWithPerson(userID string, email string) string {
	return `{"people":[{"personID":"` + userID + `","emails":["` + email + `"]}],"circles":[]}`
}

func localUsersPolicyWithPersonAndCircleSync(userID string, email string) string {
	return `{"people":[{"personID":"` + userID + `","emails":["` + email + `"]}],"circles":[],"circleSync":{"mattermostPrivateChannels":[{"circleID":"staff","channelName":"circle-staff"}]}}`
}

func newOrgchartProxyMutationTestService(t *testing.T) *Service {
	t.Helper()
	service := newLocalUsersTestService(t)
	service.Configuration.APIBaseURL = "https://api.example.test"
	service.Configuration.FleetIDPath = writeTestFile(t, "dc719d8e")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-value")
	return service
}

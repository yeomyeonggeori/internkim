package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

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

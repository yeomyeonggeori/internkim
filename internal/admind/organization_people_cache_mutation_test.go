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

func TestOrganizationPeopleCacheMarksLocalUserMutationDirtyBeforeExternalWrite(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
	personKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "email:new@example.com"}
	for _, key := range []organizationPeopleCacheKey{listKey, personKey} {
		if _, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(ctx, key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	mattermostCreated := false
	blueclawSaved := false
	checkedDirty := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !checkedDirty && request.Method != http.MethodGet {
			for _, key := range []organizationPeopleCacheKey{listKey, personKey} {
				snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
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
	for _, key := range []organizationPeopleCacheKey{listKey, personKey} {
		snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, []organizationPeopleCacheKey{key})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if snapshots[key].IsDirty {
			t.Fatalf("cache state remained dirty: %#v", snapshots[key])
		}
	}
}

func TestOrganizationPeopleCacheCompletesLocalUserMutationAfterExternalFailure(t *testing.T) {
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
	keys := []organizationPeopleCacheKey{
		{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey},
		{Kind: organizationPeopleCachePerson, Key: "email:new@example.com"},
	}
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(ctx, keys)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range keys {
		if snapshots[key].IsDirty || snapshots[key].ActiveMutations != 0 {
			t.Fatalf("cache state after failed mutation = %#v", snapshots[key])
		}
	}
}

func TestOrganizationUserMutationCleanupFailurePreservesSuccessLocal(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		expectedService := newLocalUsersTestService(t)
		expectedService.HTTPClient = newOrganizationCleanupFailureLocalUpsertClient(t)
		expectedResponse := performOrganizationCleanupFailureLocalUpsert(t, expectedService, false)

		actualService := newLocalUsersTestService(t)
		actualService.HTTPClient = newOrganizationCleanupFailureLocalUpsertClient(t)
		identity := organizationPersonIdentity{UserID: "user-new", Email: "new@example.com"}
		preloadOrganizationUserMutationCache(t, actualService, identity)
		failOrganizationUserMutationCompletion(t, actualService)
		actualResponse := performOrganizationCleanupFailureLocalUpsert(t, actualService, false)

		assertOrganizationCleanupFailureUserResponse(t, expectedResponse, actualResponse, "Local note")
		assertOrganizationUserMutationCacheIncomplete(t, actualService, identity)
	})

	t.Run("batch", func(t *testing.T) {
		expectedService := newLocalUsersTestService(t)
		expectedService.HTTPClient = newOrganizationCleanupFailureLocalUpsertClient(t)
		expectedResponse := performOrganizationCleanupFailureLocalUpsert(t, expectedService, true)

		actualService := newLocalUsersTestService(t)
		actualService.HTTPClient = newOrganizationCleanupFailureLocalUpsertClient(t)
		failOrganizationUserMutationCompletion(t, actualService)
		actualResponse := performOrganizationCleanupFailureLocalUpsert(t, actualService, true)

		assertOrganizationCleanupFailureUserResponse(t, expectedResponse, actualResponse, "")
	})

	t.Run("delete", func(t *testing.T) {
		service := newLocalUsersTestService(t)
		service.HTTPClient = newOrganizationCleanupFailureLocalDeleteClient(t)
		failOrganizationUserMutationCompletion(t, service)

		responseRecorder := httptest.NewRecorder()
		service.localRemoveUser(responseRecorder, httptest.NewRequest(http.MethodDelete, "/admin/api/users/member@example.com", nil), "member@example.com")

		if responseRecorder.Code != http.StatusOK || responseRecorder.Body.String() != "{\"ok\":true}\n" {
			t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
		}
	})
}

func TestOrganizationUserMutationCleanupFailureDeferredRetryClearsCacheState(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	identity := organizationPersonIdentity{UserID: "user-new", Email: "new@example.com"}
	preloadOrganizationUserMutationCache(t, service, identity)
	mutation, errorValue := service.startOrganizationUserMutation(ctx, []organizationPersonIdentity{identity})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	failOrganizationUserMutationCompletion(t, service)

	func() {
		defer mutation.completeAfterRequest(ctx)
		mutation.completeAfterSourceMutation(ctx)
		if mutation.isComplete {
			t.Fatal("mutation completed after failed cache cleanup")
		}
		assertOrganizationUserMutationCacheIncomplete(t, service, identity)
		dropOrganizationUserMutationCompletionFailure(t, service)
	}()

	if !mutation.isComplete {
		t.Fatal("deferred cache cleanup did not complete mutation")
	}
	assertOrganizationUserMutationCacheComplete(t, service, identity)
}
func performOrganizationCleanupFailureLocalUpsert(t *testing.T, service *Service, isBatch bool) *httptest.ResponseRecorder {
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

func failOrganizationUserMutationCompletion(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.Exec(`
		CREATE TRIGGER fail_organization_user_mutation_completion
		BEFORE UPDATE OF active_mutations ON organization_people_cache_states
		WHEN NEW.active_mutations < OLD.active_mutations
		BEGIN
			SELECT RAISE(FAIL, 'forced organization user mutation completion failure');
		END
	`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func dropOrganizationUserMutationCompletionFailure(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`DROP TRIGGER fail_organization_user_mutation_completion`); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func newOrganizationCleanupFailureLocalUpsertClient(t *testing.T) *http.Client {
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

func newOrganizationCleanupFailureLocalDeleteClient(t *testing.T) *http.Client {
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

func assertOrganizationCleanupFailureUserResponse(t *testing.T, expectedResponse *httptest.ResponseRecorder, actualResponse *httptest.ResponseRecorder, expectedNote string) {
	t.Helper()
	if expectedResponse.Code != http.StatusOK || actualResponse.Code != expectedResponse.Code {
		t.Fatalf("status = %d body = %s; want status = %d body = %s", actualResponse.Code, actualResponse.Body.String(), expectedResponse.Code, expectedResponse.Body.String())
	}
	expectedDocument := normalizedOrganizationCleanupFailureResponse(t, expectedResponse)
	actualDocument := normalizedOrganizationCleanupFailureResponse(t, actualResponse)
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

func normalizedOrganizationCleanupFailureResponse(t *testing.T, responseRecorder *httptest.ResponseRecorder) map[string]any {
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

func preloadOrganizationUserMutationCache(t *testing.T, service *Service, identity organizationPersonIdentity) {
	t.Helper()
	for _, key := range organizationUserMutationCacheKeys([]organizationPersonIdentity{identity}) {
		if _, errorValue := service.writeOrganizationPeopleCachePayloadIfCurrent(context.Background(), key, 0, "", []byte(`{}`)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func assertOrganizationUserMutationCacheIncomplete(t *testing.T, service *Service, identity organizationPersonIdentity) {
	t.Helper()
	keys := organizationUserMutationCacheKeys([]organizationPersonIdentity{identity})
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(context.Background(), keys)
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

func assertOrganizationUserMutationCacheComplete(t *testing.T, service *Service, identity organizationPersonIdentity) {
	t.Helper()
	keys := organizationUserMutationCacheKeys([]organizationPersonIdentity{identity})
	snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(context.Background(), keys)
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

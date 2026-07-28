package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOrganizationPageServesRouteAssetsAndIndex(t *testing.T) {
	adminUIPath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(adminUIPath, "organization", "assets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "organization", "index.html"), []byte("organization index"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "organization", "assets", "app.js"), []byte("organization asset"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com"), AdminUIPath: adminUIPath})

	redirectResponse := httptest.NewRecorder()
	service.router().ServeHTTP(redirectResponse, httptest.NewRequest(http.MethodGet, "/organization", nil))
	if redirectResponse.Code != http.StatusFound || redirectResponse.Header().Get("Location") != "/organization/" {
		t.Fatalf("redirect status = %d location = %q", redirectResponse.Code, redirectResponse.Header().Get("Location"))
	}

	indexResponse := httptest.NewRecorder()
	service.router().ServeHTTP(indexResponse, httptest.NewRequest(http.MethodGet, "/organization/", nil))
	if indexResponse.Code != http.StatusOK || indexResponse.Body.String() != "organization index" {
		t.Fatalf("index status = %d body = %q", indexResponse.Code, indexResponse.Body.String())
	}

	assetResponse := httptest.NewRecorder()
	service.router().ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/organization/assets/app.js", nil))
	if assetResponse.Code != http.StatusOK || assetResponse.Body.String() != "organization asset" {
		t.Fatalf("asset status = %d body = %q", assetResponse.Code, assetResponse.Body.String())
	}
}

func TestOrganizationDirectoryListsVisibleProfilesForStaff(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrganizationGroups(context.Background(), []orgGroupRecord{
		{ID: "engineering", Name: "엔지니어링"},
		{ID: "operations", Name: "운영"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{
		{
			UserID:                "user-ada",
			Email:                 "ada@example.com",
			JobTitle:              "Engineering Lead",
			GroupID:               "engineering",
			ProjectIDs:            []string{"platform"},
			EmploymentStatus:      organizationEmploymentStatusActive,
			IsOrganizationVisible: true,
		},
		{
			UserID:                "user-grace",
			Email:                 "grace@example.com",
			JobTitle:              "Backend Engineer",
			GroupID:               "engineering",
			SupervisorID:          "user-ada",
			ProjectIDs:            []string{"platform", "hiring"},
			EmploymentStatus:      organizationEmploymentStatusActive,
			IsOrganizationVisible: true,
		},
		{
			UserID:                "user-hidden",
			Email:                 "hidden@example.com",
			EmploymentStatus:      organizationEmploymentStatusActive,
			IsOrganizationVisible: false,
		},
		{
			UserID:                "user-resigned",
			Email:                 "resigned@example.com",
			EmploymentStatus:      organizationEmploymentStatusResigned,
			IsOrganizationVisible: true,
		},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[
				{"id":"user-1","email":"ada@example.com","username":"ada","nickname":"Ada Kim","roles":"system_user","delete_at":0},
				{"id":"user-2","email":"grace@example.com","username":"grace","nickname":"Grace Lee","roles":"system_user","delete_at":0},
				{"id":"user-3","email":"hidden@example.com","username":"hidden","nickname":"Hidden Lee","roles":"system_user","delete_at":0},
				{"id":"user-4","email":"resigned@example.com","username":"resigned","nickname":"Resigned Park","roles":"system_user","delete_at":0}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[
				{"user_id":"user-1","roles":"team_user"},
				{"user_id":"user-2","roles":"team_user"},
				{"user_id":"user-3","roles":"team_user"},
				{"user_id":"user-4","roles":"team_user"}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, organizationDirectoryPolicyDocument(), nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/organization/api/people", nil)
	request.Header.Set("X-Forwarded-Email", "grace@example.com")
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	assertOrganizationDirectoryContract(t, responseRecorder.Body.Bytes())
	var response organizationDirectoryResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Records) != 2 {
		t.Fatalf("records = %#v; want visible active user records", response.Records)
	}
	if response.Records[0].UserID != "user-ada" || response.Records[1].UserID != "user-grace" {
		t.Fatalf("record order = %#v; want ada then grace", response.Records)
	}
	if len(response.AvailableGroups) != 1 || response.AvailableGroups[0].ID != "engineering" {
		t.Fatalf("available groups = %#v; want connected groups only", response.AvailableGroups)
	}
}

func assertOrganizationDirectoryContract(t *testing.T, responseBody []byte) {
	t.Helper()
	var response map[string]json.RawMessage
	if errorValue := json.Unmarshal(responseBody, &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertJSONKeysAllowed(t, response, map[string]bool{"records": true, "availableGroups": true})
	var records []map[string]json.RawMessage
	if errorValue := json.Unmarshal(response["records"], &records); errorValue != nil {
		t.Fatal(errorValue)
	}
	allowedRecordKeys := map[string]bool{
		"userID": true, "handle": true, "name": true, "email": true, "image": true,
		"hireDate": true, "jobTitle": true, "groupID": true,
		"groupIDs": true, "supervisorID": true,
	}
	for _, record := range records {
		assertJSONKeysAllowed(t, record, allowedRecordKeys)
	}
}

func assertJSONKeysAllowed(t *testing.T, value map[string]json.RawMessage, allowedKeys map[string]bool) {
	t.Helper()
	for key := range value {
		if !allowedKeys[key] {
			t.Errorf("unexpected JSON field %q", key)
		}
	}
}

func organizationDirectoryPolicyDocument() string {
	return `{
		"people":[
			{"personID":"user-ada","displayName":"Ada Kim","emails":["ada@example.com"],"circles":["staff"],"isAdmin":false},
			{"personID":"user-grace","displayName":"Grace Lee","emails":["grace@example.com"],"circles":["staff"],"isAdmin":false},
			{"personID":"user-hidden","displayName":"Hidden Lee","emails":["hidden@example.com"],"circles":["staff"],"isAdmin":false},
			{"personID":"user-resigned","displayName":"Resigned Park","emails":["resigned@example.com"],"circles":["staff"],"isAdmin":false}
		],
		"circles":[{"circleID":"staff","displayName":"Staff"}]
	}`
}

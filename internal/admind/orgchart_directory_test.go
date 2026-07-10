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

func TestOrgchartPageServesRouteAssetsAndIndex(t *testing.T) {
	adminUIPath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(adminUIPath, "orgchart", "assets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "orgchart", "index.html"), []byte("orgchart index"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "orgchart", "assets", "app.js"), []byte("orgchart asset"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com"), AdminUIPath: adminUIPath})

	redirectResponse := httptest.NewRecorder()
	service.router().ServeHTTP(redirectResponse, httptest.NewRequest(http.MethodGet, "/orgchart", nil))
	if redirectResponse.Code != http.StatusFound || redirectResponse.Header().Get("Location") != "/orgchart/" {
		t.Fatalf("redirect status = %d location = %q", redirectResponse.Code, redirectResponse.Header().Get("Location"))
	}

	indexResponse := httptest.NewRecorder()
	service.router().ServeHTTP(indexResponse, httptest.NewRequest(http.MethodGet, "/orgchart/", nil))
	if indexResponse.Code != http.StatusOK || indexResponse.Body.String() != "orgchart index" {
		t.Fatalf("index status = %d body = %q", indexResponse.Code, indexResponse.Body.String())
	}

	assetResponse := httptest.NewRecorder()
	service.router().ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/orgchart/assets/app.js", nil))
	if assetResponse.Code != http.StatusOK || assetResponse.Body.String() != "orgchart asset" {
		t.Fatalf("asset status = %d body = %q", assetResponse.Code, assetResponse.Body.String())
	}
}

func TestOrgchartDirectoryListsVisibleProfilesForStaff(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrgchartGroups(context.Background(), []orgGroupRecord{
		{ID: "engineering", Name: "엔지니어링"},
		{ID: "operations", Name: "운영"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrgchartProfiles(context.Background(), []orgchartProfile{
		{
			UserID:            "user-ada",
			Email:             "ada@example.com",
			JobTitle:          "Engineering Lead",
			PrimaryGroupID:    "engineering",
			GroupIDs:          []string{"engineering"},
			ProjectIDs:        []string{"platform"},
			EmploymentStatus:  orgchartEmploymentStatusActive,
			IsOrgchartVisible: true,
		},
		{
			UserID:            "user-grace",
			Email:             "grace@example.com",
			JobTitle:          "Backend Engineer",
			PrimaryGroupID:    "engineering",
			GroupIDs:          []string{"engineering", "operations"},
			SupervisorID:      "user-ada",
			ProjectIDs:        []string{"platform", "hiring"},
			EmploymentStatus:  orgchartEmploymentStatusActive,
			IsOrgchartVisible: true,
		},
		{
			UserID:            "user-hidden",
			Email:             "hidden@example.com",
			EmploymentStatus:  orgchartEmploymentStatusActive,
			IsOrgchartVisible: false,
		},
		{
			UserID:            "user-resigned",
			Email:             "resigned@example.com",
			EmploymentStatus:  orgchartEmploymentStatusResigned,
			IsOrgchartVisible: true,
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
			return jsonResponse(http.StatusOK, orgchartDirectoryPolicyDocument(), nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/orgchart/api/people", nil)
	request.Header.Set("X-Forwarded-Email", "grace@example.com")
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response pagesUsersResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Records) != 2 {
		t.Fatalf("records = %#v; want visible active user records", response.Records)
	}
	if response.Records[0].UserID != "user-ada" || response.Records[1].UserID != "user-grace" {
		t.Fatalf("record order = %#v; want ada then grace", response.Records)
	}
	if len(response.AvailableGroups) != 2 {
		t.Fatalf("available groups = %#v; want connected groups only", response.AvailableGroups)
	}
}

func orgchartDirectoryPolicyDocument() string {
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

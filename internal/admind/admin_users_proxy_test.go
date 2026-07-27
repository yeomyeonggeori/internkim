package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAdminUserProxyGetMergesOrganizationMetadata(t *testing.T) {
	service := newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
			return response, nil
		}
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{{
		UserID:                "user-member",
		Email:                 "member@example.com",
		JobTitle:              "Design Lead",
		GroupID:               "design",
		SupervisorID:          "user-admin",
		EmploymentStatus:      organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/api/users?includePolicy=true", nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("user list status = %d body = %s", response.Code, response.Body.String())
	}
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &usersResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(usersResponse.Records) != 1 {
		t.Fatalf("records = %d; want 1", len(usersResponse.Records))
	}
	record := usersResponse.Records[0]
	if record.JobTitle != "Design Lead" || record.GroupID != "design" || record.SupervisorID != "user-admin" {
		t.Fatalf("record organization metadata = %#v", record)
	}
	expectedImage := calendarParticipantImagePath(stableFlowID("member@example.com"))
	if record.Image != expectedImage {
		t.Fatalf("record image = %q; want %q", record.Image, expectedImage)
	}
}

func TestAdminUsersCachePreservesIncludePolicyScope(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		order []bool
	}{
		{name: "included then excluded", order: []bool{true, false}},
		{name: "excluded then included", order: []bool{false, true}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
				if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
					return response, nil
				}
				switch {
				case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
					return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
				case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
					return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
				case isBlueclawPolicyGet(request):
					return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
				default:
					t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
					return nil, nil
				}
			})

			for _, includePolicy := range testCase.order {
				usersResponse := requestAdminUsersForPolicyScope(t, service, includePolicy)
				record := usersResponse.Records[0]
				if record.Role != "member" || record.MattermostUserID != "user-1" || record.MattermostUsername != "member" {
					t.Fatalf("authoritative record fields = %#v", record)
				}
				if includePolicy {
					if len(record.Circles) != 1 || record.Circles[0] != "staff" || record.Note != "Existing member note" {
						t.Fatalf("included policy record = %#v", record)
					}
					continue
				}
				if len(record.Circles) != 0 || record.Note != "" {
					t.Fatalf("excluded policy record = %#v", record)
				}
			}
		})
	}
}

func requestAdminUsersForPolicyScope(t *testing.T, service *Service, includePolicy bool) pagesUsersResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/admin/api/users?includePolicy="+strconv.FormatBool(includePolicy), nil)
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user list status = %d body = %s", response.Code, response.Body.String())
	}
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &usersResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(usersResponse.Records) != 1 {
		t.Fatalf("records = %d; want 1", len(usersResponse.Records))
	}
	return usersResponse
}

func TestAdminUserSavePatchesMattermostIdentityByStoredID(t *testing.T) {
	var pagesPayload map[string]any
	var mattermostPatch map[string]string
	var service *Service
	checkedDirty := false
	service = newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
			return response, nil
		}
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"oldhandle","name":"Old Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"oldhandle","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch" && request.Method == http.MethodPut:
			listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
			personKey := organizationPeopleCacheKey{Kind: organizationPeopleCachePerson, Key: "user-member"}
			keys := []organizationPeopleCacheKey{listKey, personKey}
			snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(request.Context(), keys)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if !snapshots[listKey].IsDirty || snapshots[listKey].ActiveMutations != 1 {
				t.Fatalf("list cache state before remote write = %#v", snapshots[listKey])
			}
			if snapshots[personKey].IsDirty || snapshots[personKey].ActiveMutations != 0 || snapshots[personKey].Found {
				t.Fatalf("unknown person cache state before remote write = %#v", snapshots[personKey])
			}
			checkedDirty = true
			if errorValue := json.NewDecoder(request.Body).Decode(&mattermostPatch); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"newhandle"}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			if errorValue := json.NewDecoder(request.Body).Decode(&pagesPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"newhandle","name":"New Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"newhandle"}]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"userID":"user-member","handle":"newhandle","name":"New Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle","note":"Needs HR compensation follow-up"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user save status = %d body = %s", response.Code, response.Body.String())
	}
	if !checkedDirty {
		t.Fatal("expected dirty cache check before remote write")
	}
	if mattermostPatch["username"] != "newhandle" || mattermostPatch["first_name"] != "New" || mattermostPatch["last_name"] != "Name" || mattermostPatch["nickname"] != "New" {
		t.Fatalf("mattermost patch = %#v", mattermostPatch)
	}
	if pagesPayload["userID"] != "user-member" || pagesPayload["handle"] != "newhandle" || pagesPayload["name"] != "New Name" {
		t.Fatalf("pages payload = %#v", pagesPayload)
	}
	if pagesPayload["note"] != "Needs HR compensation follow-up" {
		t.Fatalf("pages note payload = %#v", pagesPayload)
	}
}

func TestAdminUserSaveWritesBlueclawNote(t *testing.T) {
	var savedPerson map[string]any
	service := newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
			return response, nil
		}
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member"}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"user-member","displayName":"Member User","emails":["member@example.com"],"circles":["staff"],"isAdmin":false,"note":"Existing note"}],"channels":[],"circleSync":{"mattermostPrivateChannels":[{"circleID":"staff","channelName":"circle-staff"}]},"retention":{"rawEventDays":60}}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://127.0.0.1:8080/admin/api/policy/save":
			var policyDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&policyDocument); errorValue != nil {
				t.Fatal(errorValue)
			}
			people, _ := policyDocument["people"].([]any)
			if len(people) != 1 {
				t.Fatalf("Blueclaw people = %#v", policyDocument["people"])
			}
			savedPerson, _ = people[0].(map[string]any)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/circle-staff":
			return jsonResponse(http.StatusOK, `{"id":"circle-staff-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	requestBody := `{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member","circles":["staff"],"note":"Needs HR compensation follow-up"}`
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(requestBody))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user save status = %d body = %s", response.Code, response.Body.String())
	}
	if savedPerson["note"] != "Needs HR compensation follow-up" {
		t.Fatalf("Blueclaw note = %#v; person = %#v", savedPerson["note"], savedPerson)
	}
}

func TestOrganizationUserMutationCleanupFailurePreservesSuccessProxy(t *testing.T) {
	sourceResponseBody := `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}],"source":"pages"}`
	service := newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
			return response, nil
		}
		switch {
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","email":"member@example.com","role":"member","mattermostUserID":"user-1"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member"}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusAccepted, sourceResponseBody, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})
	failOrganizationUserMutationCompletion(t, service)

	requestBody := strings.NewReader(`{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}`)
	responseRecorder := httptest.NewRecorder()
	service.proxyUsers(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))

	expectedBody := string(usersResponseBodyWithProfileImages([]byte(sourceResponseBody)))
	if responseRecorder.Code != http.StatusAccepted || responseRecorder.Body.String() != expectedBody {
		t.Fatalf("status = %d body = %s; want status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String(), http.StatusAccepted, expectedBody)
	}
}

func newAdminUsersProxyTestService(t *testing.T, transport roundTripFunc) *Service {
	t.Helper()
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, filepath.Join(deviceDirectory, "admin-email"), "admin@example.com")
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.example.test",
		MattermostBaseURL:           "http://mattermost.local",
		BlueclawBaseURL:             "http://127.0.0.1:8080",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		ClaimedAdminEmailPath:       writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: transport}
	return service
}

func adminUsersProxyCommonMattermostResponse(t *testing.T, request *http.Request) (*http.Response, bool) {
	t.Helper()
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/users/login":
		return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
		return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case isMattermostConnectCommandSetupRequest(request):
		return mattermostConnectCommandSetupResponse(t, request), true
	case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
		assertMattermostNicknameDisplayPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
		return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), true
	case isMattermostFlowSetupRequest(request):
		return mattermostFlowSetupResponse(t, request), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
		return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
		return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
		assertBotDirectChannelShown(t, request, "user-1", "bot-1")
		return jsonResponse(http.StatusOK, `{}`, nil), true
	default:
		return nil, false
	}
}

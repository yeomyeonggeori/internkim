package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	listMutationActive := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, existingEmail), nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			activePersonKeys = activeOrgchartPersonMutationKeys(t, service)
			listKey := orgchartPeopleCacheKey{Kind: orgchartPeopleCacheList, Key: orgchartPeopleCacheSingletonKey}
			snapshots, errorValue := service.readOrgchartPeopleCacheSnapshots(context.Background(), []orgchartPeopleCacheKey{listKey})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			listSnapshot := snapshots[listKey]
			listMutationActive = listSnapshot.IsDirty && listSnapshot.ActiveMutations == 1
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
	existingEmailKey := "email:" + existingEmail
	if !containsString(activePersonKeys, existingEmailKey) {
		t.Fatalf("active person keys = %#v; missing %q", activePersonKeys, existingEmailKey)
	}
	if containsString(activePersonKeys, "email:"+newEmail) {
		t.Fatalf("active person keys = %#v; contains unknown email key", activePersonKeys)
	}
	for _, key := range activePersonKeys {
		if strings.HasPrefix(key, "user-") {
			t.Fatalf("active person keys = %#v; contains generated user ID", activePersonKeys)
		}
	}
	if !listMutationActive {
		t.Fatal("list cache mutation was not active before external write")
	}
	assertOrgchartPersonCacheFound(t, service, canonicalUserID, false)
}

func TestOrgchartPeopleCacheInvalidatesSourceAndCanonicalProxyUser(t *testing.T) {
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
			preloadOrgchartIdentityCache(t, service, remoteUserID, email)
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
					assertOrgchartIdentityMutationActive(t, service, remoteUserID, email)
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
			assertOrgchartIdentityCacheFound(t, service, remoteUserID, email, false)
		})
	}
}

func TestOrgchartPeopleCacheInvalidatesSourceAndCanonicalProxyDeletedUser(t *testing.T) {
	service := newOrgchartProxyMutationTestService(t)
	canonicalUserID := "blueclaw-deleted"
	remoteUserID := "remote-deleted"
	mattermostUserID := "mattermost-deleted"
	email := "deleted@example.com"
	preloadOrgchartIdentityCache(t, service, canonicalUserID, email)
	preloadOrgchartIdentityCache(t, service, remoteUserID, email)
	mattermostLoginCount := 0
	checkedMutationIdentities := false
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
			assertOrgchartIdentityMutationActive(t, service, remoteUserID, email)
			checkedMutationIdentities = true
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
	if !checkedMutationIdentities {
		t.Fatal("source and canonical proxy deletion mutations were not checked")
	}
	assertOrgchartIdentityCacheFound(t, service, canonicalUserID, email, false)
	assertOrgchartIdentityCacheFound(t, service, remoteUserID, email, false)
}

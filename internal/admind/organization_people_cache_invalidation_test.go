package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestOrganizationPeopleCacheInvalidatesCanonicalLocalUser(t *testing.T) {
	service := newLocalUsersTestService(t)
	canonicalUserID := "blueclaw-existing"
	email := "existing@example.com"
	preloadOrganizationIdentityCache(t, service, canonicalUserID, email)
	checkedCanonicalMutation := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, email), nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			assertOrganizationIdentityMutationActive(t, service, canonicalUserID, email)
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
	assertOrganizationIdentityCacheFound(t, service, canonicalUserID, email, false)
}

func TestOrganizationPeopleCacheInvalidatesCanonicalDeletedUser(t *testing.T) {
	service := newLocalUsersTestService(t)
	canonicalUserID := "blueclaw-deleted"
	mattermostUserID := "mattermost-deleted"
	email := "deleted@example.com"
	preloadOrganizationIdentityCache(t, service, canonicalUserID, email)
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
			assertOrganizationIdentityMutationActive(t, service, canonicalUserID, email)
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
	assertOrganizationIdentityCacheFound(t, service, canonicalUserID, email, false)
}

func TestOrganizationPeopleCacheInvalidatesCanonicalBatchUsers(t *testing.T) {
	service := newLocalUsersTestService(t)
	canonicalUserID := "blueclaw-existing"
	existingEmail := "existing@example.com"
	newEmail := "new-batch@example.com"
	preloadOrganizationIdentityCache(t, service, canonicalUserID, existingEmail)
	activePersonKeys := []string{}
	listMutationActive := false
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case isCompanyDirectoryRequest(request):
			return companyDirectoryResponse(t, request)
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, existingEmail), nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			activePersonKeys = activeOrganizationPersonMutationKeys(t, service)
			listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
			snapshots, errorValue := service.readOrganizationPeopleCacheSnapshots(context.Background(), []organizationPeopleCacheKey{listKey})
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
	assertOrganizationPersonCacheFound(t, service, canonicalUserID, false)
}

func TestOrganizationPeopleCacheInvalidatesTheProxyUsersOwnIdentity(t *testing.T) {
	for _, testCase := range []struct {
		name            string
		circlesDocument string
	}{
		{name: "invite"},
		{name: "upsert", circlesDocument: `,"circles":[]`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := newOrganizationProxyMutationTestService(t)
			canonicalUserID := "blueclaw-existing"
			email := "existing@example.com"
			preloadOrganizationIdentityCache(t, service, canonicalUserID, email)
			pagesPayloadCarriesAPersonID := false
			blueclawPersonID := ""
			policySaved := false
			seatPeopleInACompanyDirectoryForTest(t, service)
			service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
					return response, nil
				}
				switch {
				case isCompanyDirectoryRequest(request):
					return companyDirectoryResponse(t, request)
				case request.Method == http.MethodGet && request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
					return jsonResponse(http.StatusOK, `{"records":[{"email":"existing@example.com","role":"admin"}]}`, nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
					return jsonResponse(http.StatusOK, rosterAsBlueclawWouldRead(t, service, localUsersPolicyWithPersonAndCircleSync(canonicalUserID, email)), nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-1":
					return jsonResponse(http.StatusOK, `{"id":"user-1","email":"existing@example.com","username":"existing-user","roles":"system_user"}`, nil), nil
				case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch":
					return jsonResponse(http.StatusOK, `{"id":"user-1","email":"existing@example.com","username":"existing-user","roles":"system_user"}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "https://api.example.test/api/users":
					var payload adminUserMutation
					if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
						t.Fatal(errorValue)
					}
					pagesPayloadCarriesAPersonID = strings.TrimSpace(payload.MemberID) != ""
					return jsonResponse(http.StatusOK, `{"records":[{"email":"existing@example.com","role":"admin"}]}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/policy/reload":
					assertOrganizationIdentityMutationActive(t, service, canonicalUserID, email)
					blueclawPersonID = deliveredPersonIDForEmail(t, service, email)
					if blueclawPersonID != canonicalUserID {
						t.Fatalf("the delivered roster names %q for %s; want %q", blueclawPersonID, email, canonicalUserID)
					}
					policySaved = true
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/circle-member":
					return jsonResponse(http.StatusOK, `{"id":"circle-member-channel"}`, nil), nil
				case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/channels/circle-member-channel/patch":
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/channels/circle-member-channel/posts?per_page=100":
					return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
				case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/channels/circle-member-channel/members":
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
			if pagesPayloadCarriesAPersonID {
				t.Fatal("the fleet account list is not where a person's identity lives")
			}
			if policySaved && blueclawPersonID != canonicalUserID {
				t.Fatalf("the delivered roster names %q for %s; want %q", blueclawPersonID, email, canonicalUserID)
			}
			assertOrganizationIdentityCacheFound(t, service, canonicalUserID, email, false)
		})
	}
}

func TestOrganizationPeopleCacheInvalidatesSourceAndCanonicalProxyDeletedUser(t *testing.T) {
	service := newOrganizationProxyMutationTestService(t)
	canonicalUserID := "blueclaw-deleted"
	remoteUserID := "remote-deleted"
	mattermostUserID := "mattermost-deleted"
	email := "deleted@example.com"
	preloadOrganizationIdentityCache(t, service, canonicalUserID, email)
	preloadOrganizationIdentityCache(t, service, remoteUserID, email)
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
				{"memberID":"admin-source","email":"admin@example.com","role":"admin"},
				{"memberID":"`+remoteUserID+`","email":"deleted@example.com","role":"member","mattermostUserID":"`+mattermostUserID+`"}
			]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyWithPerson(canonicalUserID, email), nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/"+mattermostUserID:
			return jsonResponse(http.StatusOK, `{"id":"mattermost-deleted","email":"deleted@example.com","username":"deleted-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://mattermost.local/api/v4/users/"+mattermostUserID:
			assertOrganizationIdentityMutationActive(t, service, canonicalUserID, email)
			assertOrganizationIdentityMutationActive(t, service, remoteUserID, email)
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
	assertOrganizationIdentityCacheFound(t, service, canonicalUserID, email, false)
	assertOrganizationIdentityCacheFound(t, service, remoteUserID, email, false)
}

// deliveredPersonIDForEmail reads the roster the host wrote, which is what the agent reads
// when it is told to reload. Asserting on the reload request would only prove a call fired.
func deliveredPersonIDForEmail(t *testing.T, service *Service, email string) string {
	t.Helper()
	document, errorValue := os.ReadFile(service.Configuration.BlueclawPolicyDeliveryPath)
	if errorValue != nil {
		t.Fatalf("the host writes the roster before telling the agent to reload: %v", errorValue)
	}
	var policyDocument struct {
		People []struct {
			PersonID string   `json:"personID"`
			Emails   []string `json:"emails"`
		} `json:"people"`
	}
	if errorValue := json.Unmarshal(document, &policyDocument); errorValue != nil {
		t.Fatalf("a roster the agent cannot parse refuses everybody: %v", errorValue)
	}
	for _, person := range policyDocument.People {
		for _, personEmail := range person.Emails {
			if strings.EqualFold(strings.TrimSpace(personEmail), strings.TrimSpace(email)) {
				return person.PersonID
			}
		}
	}
	return ""
}

// rosterAsBlueclawWouldRead answers the read with the file the host last wrote, because
// that is what the agent serves. A stub that always answers the original roster hides the
// second write of an upsert overwriting the first.
func rosterAsBlueclawWouldRead(t *testing.T, service *Service, beforeAnyWrite string) string {
	t.Helper()
	document, errorValue := os.ReadFile(service.Configuration.BlueclawPolicyDeliveryPath)
	if errorValue != nil {
		return beforeAnyWrite
	}
	return string(document)
}

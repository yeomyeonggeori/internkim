package admind

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestLocalUpsertUsersBatchRejectsMoreThanFiftyUsersBeforeExternalRequests(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("unexpected external request")
	})}
	users := make([]adminUserMutation, localUsersBatchMaximumUsers+1)
	for index := range users {
		suffix := strconv.Itoa(index)
		users[index] = adminUserMutation{Email: "member-" + suffix + "@example.com", Handle: "member-" + suffix, Name: "Member " + suffix, Role: "member"}
	}
	requestBody, errorValue := json.Marshal(map[string][]adminUserMutation{"users": users})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", bytes.NewReader(requestBody)))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	if externalRequestMade {
		t.Fatal("external request was made before batch size validation")
	}
}

func TestLocalUpsertUsersBatchAcceptsExactlyFiftyUsers(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("stop after batch size validation")
	})}
	users := make([]adminUserMutation, localUsersBatchMaximumUsers)
	for index := range users {
		suffix := strconv.Itoa(index)
		users[index] = adminUserMutation{Email: "member-" + suffix + "@example.com", Handle: "member-" + suffix, Name: "Member " + suffix, Role: "member"}
	}
	requestBody, errorValue := json.Marshal(map[string][]adminUserMutation{"users": users})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", bytes.NewReader(requestBody)))

	if !externalRequestMade {
		t.Fatal("exact maximum batch did not reach external request")
	}
	if responseRecorder.Code == http.StatusBadRequest {
		t.Fatalf("exact maximum batch was rejected: %s", responseRecorder.Body.String())
	}
}

func TestLocalUpsertUsersBatchRejectsOversizedBodyBeforeExternalRequests(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("unexpected external request")
	})}
	requestBody, errorValue := json.Marshal(map[string][]adminUserMutation{"users": {{
		Email:  "member@example.com",
		Handle: "member",
		Name:   strings.Repeat("x", int(localUsersBatchMaximumRequestBytes)+1),
		Role:   "member",
	}}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", bytes.NewReader(requestBody)))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "request body exceeds maximum size") {
		t.Fatalf("body = %q", responseRecorder.Body.String())
	}
	if externalRequestMade {
		t.Fatal("external request was made before request body size validation")
	}
}

func TestLocalUpsertUsersBatchRejectsOversizedTrailingBodyBeforeExternalRequests(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("unexpected external request")
	})}
	validRequestBody := `{"users":[{"email":"member@example.com","handle":"member","name":"Member","role":"member"}]}`
	requestBody := validRequestBody + strings.Repeat(" ", int(localUsersBatchMaximumRequestBytes))

	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", strings.NewReader(requestBody)))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "request body exceeds maximum size") {
		t.Fatalf("body = %q", responseRecorder.Body.String())
	}
	if externalRequestMade {
		t.Fatal("external request was made before trailing request body size validation")
	}
}

func TestLocalUpsertUsersBatchAcceptsExactMaximumBodySize(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("stop after request body size validation")
	})}
	validRequestBody := `{"users":[{"email":"member@example.com","handle":"member","name":"Member","role":"member"}]}`
	paddingLength := int(localUsersBatchMaximumRequestBytes) - len(validRequestBody)
	requestBody := validRequestBody + strings.Repeat(" ", paddingLength)

	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", strings.NewReader(requestBody)))

	if !externalRequestMade {
		t.Fatal("exact maximum request body did not reach external request")
	}
	if responseRecorder.Code == http.StatusBadRequest {
		t.Fatalf("exact maximum request body was rejected: %s", responseRecorder.Body.String())
	}
}

package admind

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func memoryDirectoryForTest() *companyDirectoryForTest {
	return companyDirectoryHolding(centralplane.Member{MemberID: "user:person-1", Email: "member@example.com", Name: "Member", Role: "member", Status: "active"})
}

func TestMemoryAPIUsesMattermostSessionUserFacts(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/memory/facts" && request.Method == http.MethodGet {
			if request.URL.Query().Get("readerPersonID") != "user:person-1" {
				t.Fatalf("readerPersonID = %q", request.URL.Query().Get("readerPersonID"))
			}
			return jsonResponse(http.StatusOK, `{"personID":"person-1","profile":{"identityLines":[],"currentLines":[]},"facts":[]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/facts?limit=10", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory graph status = %d body = %s", response.Code, response.Body.String())
	}
	var graph map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&graph); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, ok := graph["facts"]; !ok {
		t.Fatalf("memory facts response = %+v", graph)
	}
}

func TestMemoryAPIResolvesTheSessionUserSchedules(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/schedule" && request.Method == http.MethodGet {
			if request.URL.Query().Get("creatorPersonID") != "user:person-1" {
				t.Fatalf("creatorPersonID = %q", request.URL.Query().Get("creatorPersonID"))
			}
			if request.URL.Query().Get("includeExpired") != "true" {
				t.Fatalf("includeExpired = %q", request.URL.Query().Get("includeExpired"))
			}
			if request.URL.Query().Get("limit") != "" {
				t.Fatalf("limit = %q", request.URL.Query().Get("limit"))
			}
			if request.URL.Query().Get("page") != "3" {
				t.Fatalf("page = %q", request.URL.Query().Get("page"))
			}
			if request.URL.Query().Get("pageSize") != "25" {
				t.Fatalf("pageSize = %q", request.URL.Query().Get("pageSize"))
			}
			return jsonResponse(http.StatusOK, `{"schedules":[{"taskScheduleID":"schedule-1","creatorPersonID":"user:person-1","executionMode":"agent","kind":"cron","cronExpression":"0 9 * * *","nextRunAt":"2026-06-09T00:00:00Z","createdAt":"2026-06-08T00:00:00Z","updatedAt":"2026-06-08T00:00:00Z","deliveryChannelID":"channel-1","promptPreview":"팀 일정 알려주기"}],"count":1,"totalCount":1,"page":3,"pageSize":25}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/schedules?creatorPersonID=other-person&limit=200&page=3&pageSize=25", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory schedules status = %d body = %s", response.Code, response.Body.String())
	}
	var schedules map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&schedules); errorValue != nil {
		t.Fatal(errorValue)
	}
	if schedules["count"] != float64(1) {
		t.Fatalf("memory schedules response = %+v", schedules)
	}
}

func TestScheduleToolListSignsTheActiveRequesterAndForwardsExactInput(t *testing.T) {
	assertionKey := "schedule-assertion-secret"
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.Configuration.BlueclawAssertionKeyPath = writeTestFile(t, assertionKey)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path != "/admin/api/schedule/tool-list" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		body, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if string(body) != `{"status":"failed","limit":1}` {
			t.Fatalf("schedule input = %s", body)
		}
		assertMemoryFactSignature(t, request, body, assertionKey)
		return jsonResponse(http.StatusOK, `{"schedules":[{"scheduleID":"schedule-1","taskInstruction":"full instruction","cadence":"cron","status":"failed"}]}`, nil), nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/tool-list", strings.NewReader(`{"status":"failed","limit":1}`))
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"taskInstruction":"full instruction"`) {
		t.Fatalf("schedule list response = %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/memory/api/schedules/tool-list", strings.NewReader(`{"status":"active"}{"limit":1}`))
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response = httptest.NewRecorder()
	service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("trailing JSON status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestMemorySchedulesQuerySanitizesPagination(t *testing.T) {
	query := memorySchedulesQuery(url.Values{
		"creatorPersonID": []string{"other-person"},
		"limit":           []string{"200"},
		"page":            []string{"0"},
		"pageSize":        []string{"999"},
	}, "user:person-1")
	values, errorValue := url.ParseQuery(query)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if values.Get("creatorPersonID") != "user:person-1" {
		t.Fatalf("creatorPersonID = %q", values.Get("creatorPersonID"))
	}
	if values.Get("includeExpired") != "true" {
		t.Fatalf("includeExpired = %q", values.Get("includeExpired"))
	}
	if values.Get("limit") != "" {
		t.Fatalf("limit = %q", values.Get("limit"))
	}
	if values.Get("page") != "1" {
		t.Fatalf("page = %q", values.Get("page"))
	}
	if values.Get("pageSize") != "100" {
		t.Fatalf("pageSize = %q", values.Get("pageSize"))
	}
}

func TestMemorySchedulesQueryPreservesExpiredVisibilityFilter(t *testing.T) {
	query := memorySchedulesQuery(url.Values{
		"includeExpired": []string{"false"},
	}, "user:person-1")
	values, errorValue := url.ParseQuery(query)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if values.Get("includeExpired") != "false" {
		t.Fatalf("includeExpired = %q", values.Get("includeExpired"))
	}
}

func TestMemoryAPICancelScheduleInjectsResolvedPersonID(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/schedule/cancel" && request.Method == http.MethodPost {
			var payload struct {
				TaskScheduleID  string `json:"taskScheduleID"`
				CreatorPersonID string `json:"creatorPersonID"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.TaskScheduleID != "schedule-1" {
				t.Fatalf("taskScheduleID = %q", payload.TaskScheduleID)
			}
			if payload.CreatorPersonID != "user:person-1" {
				t.Fatalf("creatorPersonID = %q", payload.CreatorPersonID)
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/cancel", strings.NewReader(`{"taskScheduleID":"schedule-1","creatorPersonID":"spoofed-person"}`))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory schedule cancel status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMemoryAPIDeleteScheduleInjectsResolvedPersonID(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/schedule/delete" && request.Method == http.MethodPost {
			var payload struct {
				TaskScheduleID  string `json:"taskScheduleID"`
				CreatorPersonID string `json:"creatorPersonID"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.TaskScheduleID != "schedule-1" {
				t.Fatalf("taskScheduleID = %q", payload.TaskScheduleID)
			}
			if payload.CreatorPersonID != "user:person-1" {
				t.Fatalf("creatorPersonID = %q", payload.CreatorPersonID)
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/delete", strings.NewReader(`{"taskScheduleID":"schedule-1","creatorPersonID":"spoofed-person"}`))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory schedule delete status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMemoryAPIUpdateScheduleInjectsResolvedPersonID(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/schedule/update" && request.Method == http.MethodPost {
			var payload struct {
				TaskScheduleID  string `json:"taskScheduleID"`
				CreatorPersonID string `json:"creatorPersonID"`
				Name            string `json:"name"`
				IntervalSecond  int    `json:"intervalSecond"`
				RepeatPolicy    string `json:"repeatPolicy"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.TaskScheduleID != "schedule-1" {
				t.Fatalf("taskScheduleID = %q", payload.TaskScheduleID)
			}
			if payload.CreatorPersonID != "user:person-1" {
				t.Fatalf("creatorPersonID = %q", payload.CreatorPersonID)
			}
			if payload.Name != "새 이름" || payload.IntervalSecond != 1800 || payload.RepeatPolicy != "unbounded" {
				t.Fatalf("payload = %+v", payload)
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/update", strings.NewReader(`{"taskScheduleID":"schedule-1","creatorPersonID":"spoofed-person","name":"새 이름","intervalSecond":1800,"repeatPolicy":"unbounded"}`))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("memory schedule update status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMemoryAPIForgetInjectsResolvedPersonID(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/memory/facts/forget" && request.Method == http.MethodPost {
			var payload struct {
				ReaderPersonID string   `json:"readerPersonID"`
				FactIDs        []string `json:"factIDs"`
				Reason         string   `json:"reason"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.ReaderPersonID != "user:person-1" {
				t.Fatalf("readerPersonID = %q", payload.ReaderPersonID)
			}
			if len(payload.FactIDs) != 1 || payload.FactIDs[0] != "fact-1" || payload.Reason != "asked" {
				t.Fatalf("forget payload = %+v", payload)
			}
			return jsonResponse(http.StatusOK, `{"forgottenFactIDs":["fact-1"]}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/facts/forget", strings.NewReader(`{"readerPersonID":"spoofed-person","factIDs":["fact-1"],"reason":"asked"}`))
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusOK {
		t.Fatalf("memory forget status = %d body = %s", response.Code, response.Body.String())
	}
}

func assertMemoryFactSignature(t *testing.T, request *http.Request, body []byte, key string) {
	t.Helper()
	parts := strings.Split(request.Header.Get(memoryAssertionHeader), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("invalid memory assertion header: %q", request.Header.Get(memoryAssertionHeader))
	}
	assertion, errorValue := base64.RawURLEncoding.DecodeString(parts[0])
	if errorValue != nil {
		t.Fatalf("decode assertion: %v", errorValue)
	}
	var claims struct {
		ReaderPersonID string `json:"readerPersonID"`
		ExpiresAt      int64  `json:"expiresAt"`
		BodySHA256     string `json:"bodySHA256"`
	}
	if errorValue := json.Unmarshal(assertion, &claims); errorValue != nil {
		t.Fatalf("decode assertion claims: %v", errorValue)
	}
	if claims.ReaderPersonID != "user:person-1" {
		t.Fatalf("assertion readerPersonID = %q", claims.ReaderPersonID)
	}
	if remaining := time.Until(time.Unix(claims.ExpiresAt, 0)); remaining < 0 || remaining > memoryAssertionLifetime {
		t.Fatalf("assertion expiry is outside the expected window: %s", remaining)
	}
	bodyHash := sha256.Sum256(body)
	if claims.BodySHA256 != hex.EncodeToString(bodyHash[:]) {
		t.Fatalf("assertion body hash = %q, want %q", claims.BodySHA256, hex.EncodeToString(bodyHash[:]))
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(http.MethodPost + "\n" + request.URL.Path + "\n" + parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if parts[1] != expected {
		t.Fatalf("assertion signature = %q, want %q", parts[1], expected)
	}
	claims.ReaderPersonID = "user:other"
	tamperedAssertion, errorValue := json.Marshal(claims)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	tamperedAssertion64 := base64.RawURLEncoding.EncodeToString(tamperedAssertion)
	tamperedMAC := hmac.New(sha256.New, []byte(key))
	_, _ = tamperedMAC.Write([]byte(http.MethodPost + "\n" + request.URL.Path + "\n" + tamperedAssertion64))
	if parts[1] == base64.RawURLEncoding.EncodeToString(tamperedMAC.Sum(nil)) {
		t.Fatal("reader tampering unexpectedly retained the signature")
	}

	tamperedBody := append([]byte(nil), body...)
	tamperedBody[len(tamperedBody)-1] ^= 1
	tamperedHash := sha256.Sum256(tamperedBody)
	if hex.EncodeToString(tamperedHash[:]) == claims.BodySHA256 {
		t.Fatal("tampered body unexpectedly retained the signed hash")
	}
}

func TestMemoryAPISchedulesHidesUpstreamFailureDetails(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/schedule" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusInternalServerError, `private backend detail`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/schedules", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("memory schedules status = %d body = %s", response.Code, response.Body.String())
	}
	responseBody := response.Body.String()
	if strings.Contains(responseBody, "private backend detail") {
		t.Fatalf("memory schedules leaked upstream detail: %s", responseBody)
	}
	if !strings.Contains(responseBody, "memory schedules unavailable") {
		t.Fatalf("memory schedules body = %s", responseBody)
	}
}

func TestMemoryAPIFactsHidesUpstreamFailureDetails(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path == "/admin/api/memory/facts" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusInternalServerError, `Traceback /workspace/.blueclaw/private.py`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/facts", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("memory graph status = %d body = %s", response.Code, response.Body.String())
	}
	responseBody := response.Body.String()
	if strings.Contains(responseBody, "Traceback") || strings.Contains(responseBody, "/workspace") {
		t.Fatalf("memory graph leaked upstream detail: %s", responseBody)
	}
	if !strings.Contains(responseBody, "memory facts unavailable") {
		t.Fatalf("memory graph body = %s", responseBody)
	}
}

func TestMemoryAPIFactsHidesIdentityFailureDetails(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return jsonResponse(http.StatusInternalServerError, `internal identity path /root/internkim/private.go`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	request := httptest.NewRequest(http.MethodGet, "/memory/api/facts", nil)
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusBadGateway {
		t.Fatalf("memory graph identity status = %d body = %s", response.Code, response.Body.String())
	}
	responseBody := response.Body.String()
	if strings.Contains(responseBody, "/root/internkim") || strings.Contains(responseBody, "private.go") {
		t.Fatalf("memory graph leaked identity detail: %s", responseBody)
	}
	if !strings.Contains(responseBody, "memory identity unavailable") {
		t.Fatalf("memory graph identity body = %s", responseBody)
	}
}

func TestMemoryAPIScheduleMutationHidesDecodeFailureDetails(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/update", strings.NewReader(`{`))
	request.RemoteAddr = "198.51.100.10:443"
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("memory schedule update status = %d body = %s", response.Code, response.Body.String())
	}
	responseBody := response.Body.String()
	if strings.Contains(responseBody, "unexpected EOF") {
		t.Fatalf("memory schedule update leaked decode detail: %s", responseBody)
	}
	if !strings.Contains(responseBody, "invalid memory schedule request") {
		t.Fatalf("memory schedule update body = %s", responseBody)
	}
}

func TestScheduleToolCreateSignsTheRequesterAndForwardsTheExactDocument(t *testing.T) {
	assertionKey := "schedule-assertion-secret"
	written := `{"scheduleID":"schedule-1","description":"주간 보고","taskInstruction":"주간 보고서를 정리해 올린다","timeZone":"Asia/Seoul","kind":"cron","cronExpression":"0 9 * * 1","nextRunAt":"2026-09-21T00:00:00Z","conversationID":"channel-1","replyTargetID":"message-1","agentProfileName":"internkim"}`
	asked := `{"taskInstruction":"주간 보고서를 정리해 올린다","kind":"cron","cronExpression":"0 9 * * 1","repeatPolicy":"unbounded","platform":"buzz","conversationID":"channel-1","replyTargetID":"message-1"}`
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.Configuration.BlueclawAssertionKeyPath = writeTestFile(t, assertionKey)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path != "/admin/api/schedule/tool-create" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		body, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if string(body) != asked {
			t.Fatalf("schedule create input = %s", body)
		}
		assertMemoryFactSignature(t, request, body, assertionKey)
		return jsonResponse(http.StatusOK, written, nil), nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/tool-create", strings.NewReader(asked))
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusOK || response.Body.String() != written {
		t.Fatalf("schedule create response = %d %s", response.Code, response.Body.String())
	}
}

func TestScheduleToolWritesRefuseADocumentTheContractRefuses(t *testing.T) {
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.Configuration.BlueclawAssertionKeyPath = writeTestFile(t, "schedule-assertion-secret")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		t.Fatalf("a refused document reached Blueclaw: %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	for _, refused := range []struct {
		path     string
		document string
	}{
		{"/memory/api/schedules/tool-create", `{"taskInstruction":"주간 보고서","kind":"once","creatorPersonID":"spoofed-person"}`},
		{"/memory/api/schedules/tool-update", `{"scheduleHint":"주간 보고"}{"intervalSecond":3600}`},
		{"/memory/api/schedules/tool-cancel", `{"scheduleIDs":["schedule-1"]}`},
	} {
		request := httptest.NewRequest(http.MethodPost, refused.path, strings.NewReader(refused.document))
		request.Header.Set(requesterEmailHeader, "member@example.com")
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s answered %d for %s", refused.path, response.Code, refused.document)
		}
	}
}

func TestScheduleToolCancelPassesBlueclawsRefusalThrough(t *testing.T) {
	refusal := `{"error":"주간 보고 is the description of two schedules","errorCode":"interaction_required","candidates":[{"scheduleID":"schedule-1"},{"scheduleID":"schedule-2"}]}`
	service := NewService(Configuration{
		APIBaseURL:      "https://api.example.test",
		BlueclawBaseURL: "http://blueclaw.local",
		FleetIDPath:     writeTestFile(t, "device-1"),
		FleetSecretPath: writeTestFile(t, "secret-1"),
	})
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	seatPeopleInACompanyDirectoryForTest(t, service)
	service.Configuration.BlueclawAssertionKeyPath = writeTestFile(t, "schedule-assertion-secret")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isCompanyDirectoryRequest(request) {
			return memoryDirectoryForTest().respond(t, request)
		}
		if request.URL.Path != "/admin/api/schedule/tool-cancel" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		return jsonResponse(http.StatusConflict, refusal, nil), nil
	})}

	request := httptest.NewRequest(http.MethodPost, "/memory/api/schedules/tool-cancel", strings.NewReader(`{"scheduleHints":["주간 보고"]}`))
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, arrivingOnTheRequesterSocket(request))

	if response.Code != http.StatusConflict || response.Body.String() != refusal {
		t.Fatalf("schedule cancel response = %d %s", response.Code, response.Body.String())
	}
}
